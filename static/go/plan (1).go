package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

const (
	daysPerWeek   = 6 // days are numbered 0..5, exactly as in the rest of the project
	periodsPerDay = 4 // periods are numbered 1..4 in PlanRow.Hour (index 0..3 inside this file)

	// maxSearchSteps stops one search from running forever.
	// If it is reached, the program says "not found within the limit"
	// (it does NOT claim that a timetable is impossible).
	maxSearchSteps = 500000
)

type PlanRow struct {
	Grade      int
	ClassID    int
	Day        int
	Hour       int
	LessonName string
	TeacherID  int
}

// ---------------------------------------------------------------------------
// makePlan: the function the rest of the project already calls
// ---------------------------------------------------------------------------

// makePlan loads the data, builds a timetable in memory, validates it and only
// then replaces the saved timetable. If anything fails, the saved timetable in
// the database is left untouched and the reasons are printed to the console.
func makePlan() {
	teacherRows := collectTeacherInformation()
	lessonsRows := collectLessonsInformation()
	classRows := collectClassInformation()

	// The collect functions return empty lists when a database error happens.
	// In that case we must not replace the saved timetable.
	if len(classRows) == 0 || len(lessonsRows) == 0 {
		fmt.Println("Timetable not generated: no classes or no lessons were found. The saved timetable was not changed.")
		return
	}

	plans, warnings, problems := generatePlan(teacherRows, lessonsRows, classRows)

	for _, message := range warnings {
		fmt.Println("Warning:", message)
	}
	if len(problems) > 0 {
		fmt.Println("The timetable could not be completed and was NOT saved. The saved timetable was not changed.")
		for _, message := range problems {
			fmt.Println(" -", message)
		}
		return
	}

	// Check the finished timetable once more, independently of the search.
	problems = validatePlan(plans, teacherRows, lessonsRows, classRows)
	if len(problems) > 0 {
		fmt.Println("The generated timetable failed validation and was NOT saved. The saved timetable was not changed.")
		for _, message := range problems {
			fmt.Println(" -", message)
		}
		return
	}

	saveError := InsertIntoWeek(plans)
	if saveError != nil {
		fmt.Println("The timetable could not be saved:", saveError)
		return
	}
	fmt.Println("Timetable generated and saved:", len(plans), "rows")
}

// InsertIntoWeek replaces the content of the week table with the given rows.
// Everything happens in one transaction: if any row fails, nothing changes.
func InsertIntoWeek(plans []PlanRow) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	// After a successful Commit this Rollback does nothing.
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM week")
	if err != nil {
		return err
	}

	statement, err := tx.Prepare("INSERT INTO week(grade,class_id,day,hour,lesson_name,lesson_grade,teacher_id) VALUES(?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer statement.Close()

	for _, row := range plans {
		_, err = statement.Exec(row.Grade, row.ClassID, row.Day, row.Hour, row.LessonName, row.Grade, row.TeacherID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ---------------------------------------------------------------------------
// In-memory data used by the scheduler
// ---------------------------------------------------------------------------

// planTeacher is a teacher with a ready-to-use availability table.
type planTeacher struct {
	ID        int
	Name      string
	Subject   string
	Free      [daysPerWeek][periodsPerDay]bool // Free[day][period-1]
	FreeCount int
}

// planTask means: "this class needs this lesson this many times per week".
type planTask struct {
	ClassIndex int // position of the class in classRows
	Grade      int
	ClassID    int
	LessonName string
	Count      int
	Continuous bool
	Teachers   []int // positions (in the teachers slice) of teachers who can teach it
}

// planCell is one slot (day + period) in a class timetable.
type planCell struct {
	Used    bool
	Task    int
	Teacher int // position in the teachers slice
}

// planCandidate is one possible way to place a block.
type planCandidate struct {
	Teacher int
	Day     int
	Period  int // first period, 0..3
}

// subjectKey makes subject comparison forgiving about spaces and letter case.
func subjectKey(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

// ---------------------------------------------------------------------------
// Step 1: convert the loaded rows into scheduler data
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Reading the "hours" number of teacher_times
// ---------------------------------------------------------------------------

// The hours column is an INTEGER. Two ways of writing it are understood:
//
//	period list: the digits are the available periods.  124  = periods 1, 2 and 4
//	flags:       one 0/1 flag per period, in order.      1011 = periods 1, 3 and 4
//	             (a leading zero is lost in an INTEGER, so 111 means 0111)
//
// The two can be told apart for every value except the single digit 1
// (period list: period 1, flags: 0001 = period 4). For that value the rest of
// the data decides. The value 0 always means "no period".

// isFlagsValue is true for 2 to 4 characters that are all 0 or 1.
func isFlagsValue(text string) bool {
	if len(text) < 2 || len(text) > periodsPerDay {
		return false
	}
	for _, ch := range text {
		if ch != '0' && ch != '1' {
			return false
		}
	}
	return true
}

// isPeriodListValue is true for distinct digits that are all real periods.
func isPeriodListValue(text string) bool {
	if len(text) < 1 || len(text) > periodsPerDay {
		return false
	}
	seen := map[rune]bool{}
	for _, ch := range text {
		if ch < '1' || ch > rune('0'+periodsPerDay) || seen[ch] {
			return false
		}
		seen[ch] = true
	}
	return true
}

// detectFlagsEncoding looks at all availability values once.
// flags is true when the data uses the flag style, mixed is true when both
// styles appear (which is suspicious).
func detectFlagsEncoding(teacherRows []TeacherRow) (flags bool, mixed bool) {
	flagsSeen := false
	listSeen := false
	for _, row := range teacherRows {
		for _, times := range row.teacherTimes {
			if times.Hour <= 0 {
				continue
			}
			text := strconv.Itoa(times.Hour)
			if isFlagsValue(text) {
				flagsSeen = true
			} else if text != "1" && isPeriodListValue(text) {
				listSeen = true
			}
		}
	}
	return flagsSeen && !listSeen, flagsSeen && listSeen
}

// decodeHours turns one stored number into the list of available periods
// (index 0 is period 1). ok is false when the number cannot be understood.
func decodeHours(value int, flagsDataset bool) (free [periodsPerDay]bool, ok bool) {
	if value == 0 {
		return free, true
	}
	text := strconv.Itoa(value)

	switch {
	case isFlagsValue(text), text == "1" && flagsDataset:
		for len(text) < periodsPerDay {
			text = "0" + text
		}
		for i, ch := range text {
			if ch == '1' {
				free[i] = true
			}
		}
		return free, true
	case isPeriodListValue(text):
		for _, ch := range text {
			free[int(ch-'0')-1] = true
		}
		return free, true
	}
	return free, false
}

// buildPlanTeachers decodes teacher availability (see decodeHours).
//
// Encoding used by the project: for every day there is one number in
// teacher_times.hours. Its decimal digits are the available periods,
// for example 124 means periods 1, 2 and 4. The value 0 means "no period".
func buildPlanTeachers(teacherRows []TeacherRow) ([]planTeacher, []string) {
	var teachers []planTeacher
	var warnings []string

	flagsDataset, mixed := detectFlagsEncoding(teacherRows)
	if mixed {
		warnings = append(warnings, "The availability values mix two styles (periods such as 124 and flags such as 1011). Check teacher_times.")
	}

	for _, row := range teacherRows {
		person := planTeacher{ID: row.ID, Name: row.Name, Subject: row.Subject}

		if len(row.teacherTimes) == 0 {
			warnings = append(warnings, fmt.Sprintf("Teacher %q (id %d) has no availability records and will not be used.", row.Name, row.ID))
		}

		for _, times := range row.teacherTimes {
			if times.Day < 0 || times.Day >= daysPerWeek {
				warnings = append(warnings, fmt.Sprintf("Teacher %q (id %d) has availability for day %d, which is outside the school week. It was ignored.", row.Name, row.ID, times.Day))
				continue
			}
			if times.Hour == 0 {
				continue // nothing available on this day
			}

			free, ok := decodeHours(times.Hour, flagsDataset)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("Teacher %q (id %d), day %d: the availability value %d cannot be read (expected periods such as 124, or flags such as 1011). It was ignored.", row.Name, row.ID, times.Day, times.Hour))
				continue
			}
			for p := 0; p < periodsPerDay; p++ {
				if free[p] {
					person.Free[times.Day][p] = true
				}
			}
		}

		for d := 0; d < daysPerWeek; d++ {
			for p := 0; p < periodsPerDay; p++ {
				if person.Free[d][p] {
					person.FreeCount++
				}
			}
		}
		if len(row.teacherTimes) > 0 && person.FreeCount == 0 {
			warnings = append(warnings, fmt.Sprintf("Teacher %q (id %d) has no usable available period and will not be used.", row.Name, row.ID))
		}

		teachers = append(teachers, person)
	}

	return teachers, warnings
}

// buildPlanTasks creates one task for every (class, lesson of the same grade).
func buildPlanTasks(classRows []ClassRow, lessonsRows []LessonsRow, teachers []planTeacher) ([]planTask, []string, []string) {
	var tasks []planTask
	var warnings []string
	var problems []string

	gradeHasClass := map[int]bool{}
	for _, classRow := range classRows {
		gradeHasClass[classRow.Grade] = true
	}

	for _, lessonRow := range lessonsRows {
		if !gradeHasClass[lessonRow.Grade] {
			warnings = append(warnings, fmt.Sprintf("Lesson %q for grade %d was ignored because there is no class in that grade.", lessonRow.Name, lessonRow.Grade))
		}
		if lessonRow.Count <= 0 && gradeHasClass[lessonRow.Grade] {
			problems = append(problems, fmt.Sprintf("Lesson %q (grade %d) has %d weekly periods. The value is missing, zero or could not be read.", lessonRow.Name, lessonRow.Grade, lessonRow.Count))
		}
	}

	for classIndex, classRow := range classRows {
		for _, lessonRow := range lessonsRows {
			if lessonRow.Grade != classRow.Grade || lessonRow.Count <= 0 {
				continue
			}

			task := planTask{
				ClassIndex: classIndex,
				Grade:      classRow.Grade,
				ClassID:    classRow.Grade_id,
				LessonName: lessonRow.Name,
				Count:      lessonRow.Count,
				Continuous: lessonRow.ScheduleType,
			}
			for teacherIndex := range teachers {
				if teachers[teacherIndex].FreeCount > 0 && subjectKey(teachers[teacherIndex].Subject) == subjectKey(lessonRow.Name) {
					task.Teachers = append(task.Teachers, teacherIndex)
				}
			}
			tasks = append(tasks, task)
		}
	}

	if len(tasks) == 0 {
		problems = append(problems, "No lesson belongs to any existing class, so there is nothing to schedule.")
	}

	return tasks, warnings, problems
}

// ---------------------------------------------------------------------------
// Step 2: checks that can PROVE a timetable is impossible (simple arithmetic)
// ---------------------------------------------------------------------------

// teacherFreeFor tells if the teacher is available for `length` consecutive
// periods starting at `period` on `day` (ignoring everything else).
func teacherFreeFor(person *planTeacher, day int, period int, length int) bool {
	if period < 0 || period+length > periodsPerDay {
		return false
	}
	for k := 0; k < length; k++ {
		if !person.Free[day][period+k] {
			return false
		}
	}
	return true
}

// countOptions counts how many (teacher, day, start period) combinations exist
// for a block of the given length, looking only at teacher availability.
func countOptions(task planTask, length int, teachers []planTeacher) int {
	total := 0
	for _, teacherIndex := range task.Teachers {
		for d := 0; d < daysPerWeek; d++ {
			for p := 0; p+length <= periodsPerDay; p++ {
				if teacherFreeFor(&teachers[teacherIndex], d, p, length) {
					total++
				}
			}
		}
	}
	return total
}

type subjectInfo struct {
	Name     string
	Required int // periods per week that all classes together need
	Classes  int // number of classes that need this subject
	Capacity int // most periods per week the teachers could give
}

// subjectTotals compares what every subject needs with what its teachers can give.
// In one period, at most "number of classes" lessons of a subject can run, and at
// most "number of free teachers" can run, so the smaller number is the capacity.
func subjectTotals(tasks []planTask, teachers []planTeacher) map[string]*subjectInfo {
	info := map[string]*subjectInfo{}

	for _, task := range tasks {
		key := subjectKey(task.LessonName)
		if info[key] == nil {
			info[key] = &subjectInfo{Name: task.LessonName}
		}
		info[key].Required += task.Count
		info[key].Classes++
	}

	for key, subject := range info {
		for d := 0; d < daysPerWeek; d++ {
			for p := 0; p < periodsPerDay; p++ {
				free := 0
				for teacherIndex := range teachers {
					if subjectKey(teachers[teacherIndex].Subject) == key && teachers[teacherIndex].Free[d][p] {
						free++
					}
				}
				if free > subject.Classes {
					free = subject.Classes
				}
				subject.Capacity += free
			}
		}
	}

	return info
}

// checkPlanResources finds data that makes a complete timetable impossible
// for sure. Each message here is a real proof, not a guess.
func checkPlanResources(tasks []planTask, teachers []planTeacher) []string {
	var problems []string
	reported := map[string]bool{}
	addOnce := func(key string, message string) {
		if !reported[key] {
			reported[key] = true
			problems = append(problems, message)
		}
	}

	weekPeriods := daysPerWeek * periodsPerDay

	for _, task := range tasks {
		label := fmt.Sprintf("Lesson %q (grade %d)", task.LessonName, task.Grade)
		key := fmt.Sprintf("%s|%d", task.LessonName, task.Grade)

		if len(task.Teachers) == 0 {
			addOnce(key, label+": no teacher with available periods teaches this subject.")
			continue
		}
		if task.Count > weekPeriods {
			addOnce(key, fmt.Sprintf("%s: needs %d periods, but a week has only %d.", label, task.Count, weekPeriods))
			continue
		}
		if task.Continuous && task.Count > periodsPerDay {
			addOnce(key, fmt.Sprintf("%s: must be consecutive and needs %d periods, but a day has only %d.", label, task.Count, periodsPerDay))
			continue
		}
		if task.Continuous {
			if countOptions(task, task.Count, teachers) == 0 {
				addOnce(key, fmt.Sprintf("%s: no eligible teacher is free for %d consecutive periods on any day.", label, task.Count))
			}
			continue
		}

		var usable [daysPerWeek][periodsPerDay]bool
		usableCount := 0
		for _, teacherIndex := range task.Teachers {
			for d := 0; d < daysPerWeek; d++ {
				for p := 0; p < periodsPerDay; p++ {
					if teachers[teacherIndex].Free[d][p] && !usable[d][p] {
						usable[d][p] = true
						usableCount++
					}
				}
			}
		}
		if usableCount < task.Count {
			addOnce(key, fmt.Sprintf("%s: needs %d periods, but its eligible teachers are free in only %d different periods.", label, task.Count, usableCount))
		}
	}

	// A class cannot need more periods than the week has.
	classLoad := map[[2]int]int{}
	for _, task := range tasks {
		classLoad[[2]int{task.Grade, task.ClassID}] += task.Count
	}
	seenClass := map[[2]int]bool{}
	for _, task := range tasks {
		classKey := [2]int{task.Grade, task.ClassID}
		if seenClass[classKey] {
			continue
		}
		seenClass[classKey] = true
		if classLoad[classKey] > weekPeriods {
			problems = append(problems, fmt.Sprintf("Grade %d class %d: its lessons need %d periods per week, but a week has only %d.", task.Grade, task.ClassID, classLoad[classKey], weekPeriods))
		}
	}

	// A subject cannot need more teacher time than its teachers have.
	info := subjectTotals(tasks, teachers)
	seenSubject := map[string]bool{}
	for _, task := range tasks {
		key := subjectKey(task.LessonName)
		if seenSubject[key] {
			continue
		}
		seenSubject[key] = true
		subject := info[key]
		if subject.Capacity > 0 && subject.Required > subject.Capacity {
			problems = append(problems, fmt.Sprintf("Subject %q: the classes need %d periods per week, but its teachers can give at most %d.", subject.Name, subject.Required, subject.Capacity))
		}
	}

	return problems
}

// ---------------------------------------------------------------------------
// Step 3: the backtracking search
// ---------------------------------------------------------------------------

// buildPlanPressure gives every task a number: how much of its subject's
// teacher time is needed. Higher means "scarcer", and scarcer lessons are
// preferred when two lessons are otherwise equally constrained.
func buildPlanPressure(tasks []planTask, teachers []planTeacher) []float64 {
	info := subjectTotals(tasks, teachers)
	pressure := make([]float64, len(tasks))
	for taskIndex, task := range tasks {
		subject := info[subjectKey(task.LessonName)]
		if subject.Capacity > 0 {
			pressure[taskIndex] = float64(subject.Required) / float64(subject.Capacity)
		}
	}
	return pressure
}

// planScheduler holds the timetable that is being built and everything needed
// to undo a step.
//
// A continuous lesson is placed in ONE step (one block of Count periods).
// A normal lesson is placed in Count steps (one period each).
type planScheduler struct {
	teachers []planTeacher
	tasks    []planTask
	pressure []float64

	// strict = true:  once a class got a teacher for a lesson, all later
	//                 periods of that lesson must use the same teacher.
	// strict = false: the same teacher is tried first, but another teacher
	//                 is allowed when nothing else works.
	strict bool

	classGrid   [][daysPerWeek][periodsPerDay]planCell // what is in each class slot
	teacherBusy [][daysPerWeek][periodsPerDay]bool     // is the teacher already teaching then?
	taskTeacher []int                                  // teacher of a task's first placed block, -1 if none yet
	assigned    []int                                  // periods placed so far for each task
	remaining   []int                                  // steps still to do for each task
	lastSlot    []int                                  // slot (day*4+period) of the last placed period of a normal lesson, -1 if none
	dayLoad     [][daysPerWeek]int                     // periods of a task already placed on each day

	totalSteps int
	steps      int
	limitHit   bool

	// best partial attempt, used only for the failure report
	bestDepth    int
	bestStuck    int // task that had no place left at the deepest point, -1 if unknown
	bestAssigned []int
}

func newPlanScheduler(teachers []planTeacher, tasks []planTask, pressure []float64, classCount int, strict bool) *planScheduler {
	s := &planScheduler{
		teachers:     teachers,
		tasks:        tasks,
		pressure:     pressure,
		strict:       strict,
		classGrid:    make([][daysPerWeek][periodsPerDay]planCell, classCount),
		teacherBusy:  make([][daysPerWeek][periodsPerDay]bool, len(teachers)),
		taskTeacher:  make([]int, len(tasks)),
		assigned:     make([]int, len(tasks)),
		remaining:    make([]int, len(tasks)),
		lastSlot:     make([]int, len(tasks)),
		dayLoad:      make([][daysPerWeek]int, len(tasks)),
		bestDepth:    -1,
		bestStuck:    -1,
		bestAssigned: make([]int, len(tasks)),
	}
	for taskIndex, task := range tasks {
		s.taskTeacher[taskIndex] = -1
		s.lastSlot[taskIndex] = -1
		if task.Continuous {
			s.remaining[taskIndex] = 1
		} else {
			s.remaining[taskIndex] = task.Count
		}
		s.totalSteps += s.remaining[taskIndex]
	}
	return s
}

// blockLength is how many periods one step of this task places.
func (s *planScheduler) blockLength(taskIndex int) int {
	if s.tasks[taskIndex].Continuous {
		return s.tasks[taskIndex].Count
	}
	return 1
}

// solve does one step per call. Each time it:
//  1. counts the possible places of every unfinished lesson,
//  2. gives up on this branch at once if some lesson has NO place left,
//  3. otherwise takes the lesson with the fewest places and tries them one by one.
//
// If a place leads to a dead end, it is undone and the next place is tried.
// That is the backtracking. It returns true when every lesson is complete.
func (s *planScheduler) solve(depth int) bool {
	if depth > s.bestDepth {
		s.bestDepth = depth
		s.bestStuck = -1
		copy(s.bestAssigned, s.assigned)
	}
	if depth == s.totalSteps {
		return true
	}

	s.steps++
	if s.steps > maxSearchSteps {
		s.limitHit = true
		return false
	}

	chosen := -1
	chosenCount := 0
	for taskIndex := range s.tasks {
		if s.remaining[taskIndex] == 0 {
			continue
		}
		stopAfter := 0
		if chosen != -1 {
			stopAfter = chosenCount // counting further cannot make it the best
		}
		count := s.countCandidates(taskIndex, stopAfter)
		if count == 0 {
			if depth == s.bestDepth && s.bestStuck == -1 {
				s.bestStuck = taskIndex
			}
			return false
		}
		if chosen == -1 || s.isMoreConstrained(taskIndex, count, chosen, chosenCount) {
			chosen = taskIndex
			chosenCount = count
		}
	}

	for _, option := range s.candidates(chosen) {
		setTeacher := false
		if s.taskTeacher[chosen] == -1 {
			s.taskTeacher[chosen] = option.Teacher
			setTeacher = true
		}
		previousSlot := s.lastSlot[chosen]

		s.place(chosen, option)
		if s.solve(depth + 1) {
			return true
		}
		s.unplace(chosen, option, previousSlot)

		if setTeacher {
			s.taskTeacher[chosen] = -1
		}
		if s.limitHit {
			return false
		}
	}
	return false
}

// isMoreConstrained decides which of two lessons should be placed first:
// fewer places, then longer blocks, then scarcer teachers.
func (s *planScheduler) isMoreConstrained(a int, countA int, b int, countB int) bool {
	if countA != countB {
		return countA < countB
	}
	if s.blockLength(a) != s.blockLength(b) {
		return s.blockLength(a) > s.blockLength(b)
	}
	return s.pressure[a] > s.pressure[b]
}

// forEachCandidate calls visit for every valid way to place the next step of
// the task. visit returns false to stop early.
func (s *planScheduler) forEachCandidate(taskIndex int, visit func(planCandidate) bool) {
	task := s.tasks[taskIndex]
	length := s.blockLength(taskIndex)

	teacherList := task.Teachers
	if s.strict && s.taskTeacher[taskIndex] != -1 {
		teacherList = []int{s.taskTeacher[taskIndex]}
	}

	// The periods of a normal lesson are identical, so they are placed in
	// increasing time order. This avoids trying the same timetable many times
	// in different orders.
	minSlot := 0
	if !task.Continuous {
		minSlot = s.lastSlot[taskIndex] + 1
	}

	for _, teacherIndex := range teacherList {
		for d := 0; d < daysPerWeek; d++ {
			for p := 0; p+length <= periodsPerDay; p++ {
				if d*periodsPerDay+p < minSlot {
					continue
				}
				if s.fits(task.ClassIndex, teacherIndex, d, p, length) {
					if !visit(planCandidate{Teacher: teacherIndex, Day: d, Period: p}) {
						return
					}
				}
			}
		}
	}
}

// countCandidates counts the places for a task. If stopAfter is above zero,
// counting stops as soon as the count is larger than stopAfter.
func (s *planScheduler) countCandidates(taskIndex int, stopAfter int) int {
	count := 0
	s.forEachCandidate(taskIndex, func(planCandidate) bool {
		count++
		return stopAfter <= 0 || count <= stopAfter
	})
	return count
}

// candidates lists every valid place for a task, best choice first.
func (s *planScheduler) candidates(taskIndex int) []planCandidate {
	var list []planCandidate
	s.forEachCandidate(taskIndex, func(option planCandidate) bool {
		list = append(list, option)
		return true
	})

	// Order: the lesson's usual teacher first, then days where this lesson
	// appears least (to spread it over the week), then the earliest time.
	preferred := s.taskTeacher[taskIndex]
	sort.SliceStable(list, func(x, y int) bool {
		a, b := list[x], list[y]
		if preferred != -1 && (a.Teacher == preferred) != (b.Teacher == preferred) {
			return a.Teacher == preferred
		}
		loadA := s.dayLoad[taskIndex][a.Day]
		loadB := s.dayLoad[taskIndex][b.Day]
		if loadA != loadB {
			return loadA < loadB
		}
		return a.Day*periodsPerDay+a.Period < b.Day*periodsPerDay+b.Period
	})
	return list
}

// fits checks the WHOLE block before anything is written:
// the class is free, the teacher is available and the teacher is not busy.
func (s *planScheduler) fits(classIndex int, teacherIndex int, day int, period int, length int) bool {
	person := &s.teachers[teacherIndex]
	for k := 0; k < length; k++ {
		p := period + k
		if s.classGrid[classIndex][day][p].Used {
			return false
		}
		if !person.Free[day][p] {
			return false
		}
		if s.teacherBusy[teacherIndex][day][p] {
			return false
		}
	}
	return true
}

func (s *planScheduler) place(taskIndex int, option planCandidate) {
	task := s.tasks[taskIndex]
	length := s.blockLength(taskIndex)
	for k := 0; k < length; k++ {
		s.classGrid[task.ClassIndex][option.Day][option.Period+k] = planCell{Used: true, Task: taskIndex, Teacher: option.Teacher}
		s.teacherBusy[option.Teacher][option.Day][option.Period+k] = true
	}
	s.assigned[taskIndex] += length
	s.remaining[taskIndex]--
	s.dayLoad[taskIndex][option.Day] += length
	s.lastSlot[taskIndex] = option.Day*periodsPerDay + option.Period
}

// unplace undoes place completely: class slots, teacher slots and all counters.
func (s *planScheduler) unplace(taskIndex int, option planCandidate, previousSlot int) {
	task := s.tasks[taskIndex]
	length := s.blockLength(taskIndex)
	for k := 0; k < length; k++ {
		s.classGrid[task.ClassIndex][option.Day][option.Period+k] = planCell{}
		s.teacherBusy[option.Teacher][option.Day][option.Period+k] = false
	}
	s.assigned[taskIndex] -= length
	s.remaining[taskIndex]++
	s.dayLoad[taskIndex][option.Day] -= length
	s.lastSlot[taskIndex] = previousSlot
}

// ---------------------------------------------------------------------------
// Step 5: turn a finished search into PlanRow values, or into a failure report
// ---------------------------------------------------------------------------

// toPlanRows creates one row for every slot of every class, in the same order
// and with the same "empty slot" convention (LessonName "" and TeacherID 0)
// that the project used before.
func (s *planScheduler) toPlanRows(classRows []ClassRow) []PlanRow {
	var rows []PlanRow
	for classIndex, classRow := range classRows {
		for d := 0; d < daysPerWeek; d++ {
			for p := 0; p < periodsPerDay; p++ {
				row := PlanRow{Grade: classRow.Grade, ClassID: classRow.Grade_id, Day: d, Hour: p + 1}
				cell := s.classGrid[classIndex][d][p]
				if cell.Used {
					row.LessonName = s.tasks[cell.Task].LessonName
					row.TeacherID = s.teachers[cell.Teacher].ID
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}

// teacherSwitchWarnings lists lessons that needed more than one teacher.
func (s *planScheduler) teacherSwitchWarnings() []string {
	var warnings []string
	for taskIndex, task := range s.tasks {
		used := map[int]bool{}
		for d := 0; d < daysPerWeek; d++ {
			for p := 0; p < periodsPerDay; p++ {
				cell := s.classGrid[task.ClassIndex][d][p]
				if cell.Used && cell.Task == taskIndex {
					used[cell.Teacher] = true
				}
			}
		}
		if len(used) > 1 {
			warnings = append(warnings, fmt.Sprintf("Grade %d class %d, lesson %q is taught by %d different teachers, because no timetable with one teacher per lesson was found.", task.Grade, task.ClassID, task.LessonName, len(used)))
		}
	}
	return warnings
}

func (s *planScheduler) teacherSummary(task planTask) string {
	var parts []string
	for _, teacherIndex := range task.Teachers {
		parts = append(parts, fmt.Sprintf("%s: %d available periods per week", s.teachers[teacherIndex].Name, s.teachers[teacherIndex].FreeCount))
	}
	return strings.Join(parts, "; ")
}

// failureReport describes the best partial attempt. That partial attempt is
// only used for this report. It is never saved.
func (s *planScheduler) failureReport() []string {
	var lines []string

	if s.limitHit {
		lines = append(lines, fmt.Sprintf("No complete timetable was found within the search limit (%d steps). This does not prove that no timetable exists.", maxSearchSteps))
	} else {
		lines = append(lines, "The search tried every possible arrangement and found no timetable that satisfies all rules.")
	}

	stuckTask := s.bestStuck

	for taskIndex, task := range s.tasks {
		placed := s.bestAssigned[taskIndex]
		if placed >= task.Count {
			continue
		}

		reason := "it was not reached, because the search got stuck on another lesson"
		if taskIndex == stuckTask {
			if s.limitHit {
				reason = "the search was still working on this lesson when the step limit was reached"
			} else {
				reason = "no period was left in which the class and one of its eligible teachers (" + s.teacherSummary(task) + ") were both free"
			}
		} else if placed > 0 {
			reason = "only part of it was placed before the search got stuck"
		}

		lines = append(lines, fmt.Sprintf("Grade %d class %d, lesson %q: needs %d period(s), the best attempt placed %d. Reason: %s.", task.Grade, task.ClassID, task.LessonName, task.Count, placed, reason))
	}

	return lines
}

// ---------------------------------------------------------------------------
// generatePlan: the whole generation, in memory only (no database access)
// ---------------------------------------------------------------------------

// generatePlan returns the timetable rows, warnings and problems.
// If problems is not empty, no timetable is returned.
func generatePlan(teacherRows []TeacherRow, lessonsRows []LessonsRow, classRows []ClassRow) ([]PlanRow, []string, []string) {
	teachers, warnings := buildPlanTeachers(teacherRows)
	tasks, taskWarnings, problems := buildPlanTasks(classRows, lessonsRows, teachers)
	warnings = append(warnings, taskWarnings...)
	problems = append(problems, checkPlanResources(tasks, teachers)...)
	if len(problems) > 0 {
		return nil, warnings, problems
	}

	pressure := buildPlanPressure(tasks, teachers)

	// First try: one teacher per lesson for each class.
	best := newPlanScheduler(teachers, tasks, pressure, len(classRows), true)
	found := best.solve(0)

	// Second try (only if the first failed): teachers may change inside a lesson.
	if !found {
		best = newPlanScheduler(teachers, tasks, pressure, len(classRows), false)
		found = best.solve(0)
		if found {
			warnings = append(warnings, best.teacherSwitchWarnings()...)
		}
	}

	if !found {
		return nil, warnings, best.failureReport()
	}
	return best.toPlanRows(classRows), warnings, nil
}

// ---------------------------------------------------------------------------
// Step 6: independent validation of the finished timetable
// ---------------------------------------------------------------------------

type planClassKey struct {
	Grade   int
	ClassID int
}
type planLessonKey struct {
	Name  string
	Grade int
}
type planSlotKey struct {
	Grade   int
	ClassID int
	Day     int
	Hour    int
}
type planTeacherSlotKey struct {
	TeacherID int
	Day       int
	Hour      int
}
type planClassLessonKey struct {
	Grade   int
	ClassID int
	Name    string
}

// validatePlan re-checks every rule using only the finished rows and the
// original data. It does not use any search state.
func validatePlan(plans []PlanRow, teacherRows []TeacherRow, lessonsRows []LessonsRow, classRows []ClassRow) []string {
	var problems []string
	teachers, _ := buildPlanTeachers(teacherRows)

	teacherByID := map[int]planTeacher{}
	for _, person := range teachers {
		teacherByID[person.ID] = person
	}
	lessonExists := map[planLessonKey]bool{}
	for _, lessonRow := range lessonsRows {
		lessonExists[planLessonKey{lessonRow.Name, lessonRow.Grade}] = true
	}
	knownClass := map[planClassKey]bool{}
	for _, classRow := range classRows {
		knownClass[planClassKey{classRow.Grade, classRow.Grade_id}] = true
	}

	seenSlot := map[planSlotKey]bool{}
	teacherSlot := map[planTeacherSlotKey]bool{}
	placed := map[planClassLessonKey][]PlanRow{}

	for _, row := range plans {
		where := fmt.Sprintf("Grade %d class %d, day %d, period %d", row.Grade, row.ClassID, row.Day, row.Hour)

		if row.Day < 0 || row.Day >= daysPerWeek || row.Hour < 1 || row.Hour > periodsPerDay {
			problems = append(problems, where+": the day or period is outside the school week.")
			continue
		}
		if !knownClass[planClassKey{row.Grade, row.ClassID}] {
			problems = append(problems, where+": this class does not exist.")
			continue
		}
		slot := planSlotKey{row.Grade, row.ClassID, row.Day, row.Hour}
		if seenSlot[slot] {
			problems = append(problems, where+": the class has more than one entry in this period.")
			continue
		}
		seenSlot[slot] = true

		if row.LessonName == "" {
			if row.TeacherID != 0 {
				problems = append(problems, where+": an empty period has a teacher.")
			}
			continue
		}
		if !lessonExists[planLessonKey{row.LessonName, row.Grade}] {
			problems = append(problems, fmt.Sprintf("%s: lesson %q is not defined for grade %d.", where, row.LessonName, row.Grade))
			continue
		}
		if row.TeacherID == 0 {
			problems = append(problems, fmt.Sprintf("%s: lesson %q has no teacher.", where, row.LessonName))
			continue
		}

		person, exists := teacherByID[row.TeacherID]
		if !exists {
			problems = append(problems, fmt.Sprintf("%s: teacher id %d does not exist.", where, row.TeacherID))
			continue
		}
		if subjectKey(person.Subject) != subjectKey(row.LessonName) {
			problems = append(problems, fmt.Sprintf("%s: teacher %q teaches %q, not %q.", where, person.Name, person.Subject, row.LessonName))
		}
		if !person.Free[row.Day][row.Hour-1] {
			problems = append(problems, fmt.Sprintf("%s: teacher %q is not available in this period.", where, person.Name))
		}
		teacherKey := planTeacherSlotKey{row.TeacherID, row.Day, row.Hour}
		if teacherSlot[teacherKey] {
			problems = append(problems, fmt.Sprintf("%s: teacher %q already teaches another class in this period.", where, person.Name))
		}
		teacherSlot[teacherKey] = true

		key := planClassLessonKey{row.Grade, row.ClassID, row.LessonName}
		placed[key] = append(placed[key], row)
	}

	for _, classRow := range classRows {
		for d := 0; d < daysPerWeek; d++ {
			for h := 1; h <= periodsPerDay; h++ {
				if !seenSlot[planSlotKey{classRow.Grade, classRow.Grade_id, d, h}] {
					problems = append(problems, fmt.Sprintf("Grade %d class %d, day %d, period %d: this period is missing from the timetable.", classRow.Grade, classRow.Grade_id, d, h))
				}
			}
		}

		for _, lessonRow := range lessonsRows {
			if lessonRow.Grade != classRow.Grade {
				continue
			}
			rows := placed[planClassLessonKey{classRow.Grade, classRow.Grade_id, lessonRow.Name}]
			if len(rows) != lessonRow.Count {
				problems = append(problems, fmt.Sprintf("Grade %d class %d, lesson %q: requires %d periods, but %d were assigned.", classRow.Grade, classRow.Grade_id, lessonRow.Name, lessonRow.Count, len(rows)))
				continue
			}
			if lessonRow.ScheduleType && !isOneConsecutiveBlock(rows) {
				problems = append(problems, fmt.Sprintf("Grade %d class %d, lesson %q: must be consecutive on one day with one teacher, but it is not.", classRow.Grade, classRow.Grade_id, lessonRow.Name))
			}
		}
	}

	return problems
}

// isOneConsecutiveBlock is true when all rows are on the same day, taught by
// the same teacher, in directly following periods.
func isOneConsecutiveBlock(rows []PlanRow) bool {
	if len(rows) == 0 {
		return true
	}
	sorted := append([]PlanRow(nil), rows...)
	sort.Slice(sorted, func(a, b int) bool {
		return sorted[a].Hour < sorted[b].Hour
	})
	for k := range sorted {
		if sorted[k].Day != sorted[0].Day || sorted[k].TeacherID != sorted[0].TeacherID || sorted[k].Hour != sorted[0].Hour+k {
			return false
		}
	}
	return true
}

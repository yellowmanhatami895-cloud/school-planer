package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// teacherWithHours makes a teacher who has the same "hours" value on every school day.
func teacherWithHours(id int, subject string, hours int) TeacherRow {
	row := TeacherRow{
		ID:      id,
		Name:    "Teacher" + subject,
		Subject: []string{subject},
	}
	for day := 0; day < daysPerWeek; day++ {
		row.teacherTimes = append(row.teacherTimes, TeacherTimes{Day: day, Hour: hours})
	}
	return row
}

// scenarioRows builds the test school: 3 grades x 3 classes, 6 subjects,
// 3 periods per subject per grade (18 of the 20 weekly slots of a class),
// 3 teachers per subject, available in every period (hours 1234).
// When continuous is true, every second lesson (S1, S3, S5) must be consecutive.
func scenarioRows(continuous bool) ([]TeacherRow, []LessonsRow, []ClassRow) {
	subjects := []string{"S1", "S2", "S3", "S4", "S5", "S6"}

	var teachers []TeacherRow
	id := 1
	for _, subject := range subjects {
		for k := 0; k < 3; k++ {
			teachers = append(teachers, teacherWithHours(id, subject, 1234))
			id++
		}
	}

	var lessons []LessonsRow
	var classes []ClassRow
	for _, grade := range []int{10, 11, 12} {
		for classID := 1; classID <= 3; classID++ {
			classes = append(classes, ClassRow{Grade: grade, Grade_id: classID})
		}
		for index, subject := range subjects {
			lessons = append(lessons, LessonsRow{Name: subject, Grade: grade, Count: 3, ScheduleType: continuous && index%2 == 0})
		}
	}
	return teachers, lessons, classes
}

// setLessonCounts gives the lessons of every grade the given periods, in subject order.
func setLessonCounts(lessonRows []LessonsRow, counts []int) {
	for i := range lessonRows {
		lessonRows[i].Count = counts[i%len(counts)]
	}
}

func containsText(messages []string, text string) bool {
	for _, message := range messages {
		if strings.Contains(message, text) {
			return true
		}
	}
	return false
}

func TestWeekIsFiveDaysOfFourPeriods(t *testing.T) {
	if daysPerWeek != 5 || periodsPerDay != 4 || daysPerWeek*periodsPerDay != 20 {
		t.Fatalf("expected 5 days x 4 periods = 20 slots, got %d x %d", daysPerWeek, periodsPerDay)
	}
	if planDayName(0) != "شنبه" || planDayName(4) != "چهارشنبه" {
		t.Errorf("day mapping is wrong: 0 = %s, 4 = %s", planDayName(0), planDayName(4))
	}
	if planDayName(legacyThursday) != "day 5" {
		t.Errorf("day 5 must not be a school day, got %s", planDayName(legacyThursday))
	}
}

func TestDecodeHours(t *testing.T) {
	tests := []struct {
		value int
		want  []int // available periods, numbered 1..4
		ok    bool
	}{
		{1234, []int{1, 2, 3, 4}, true},
		{124, []int{1, 2, 4}, true},
		{24, []int{2, 4}, true},
		{12, []int{1, 2}, true},
		{1, []int{1}, true}, // a single digit is always that period
		{0, nil, true},
		{1111, []int{1, 2, 3, 4}, true}, // old test data: four 0/1 flags
		{1011, []int{1, 3, 4}, true},    // old test data: four 0/1 flags
		{111, nil, false},               // 3 flags: a leading zero may be lost, so not guessed
		{11, nil, false},
		{55, nil, false},
		{11111, nil, false},
		{-1, nil, false},
	}

	for _, test := range tests {
		free, ok := decodeHours(test.value)
		if ok != test.ok {
			t.Errorf("decodeHours(%d): ok = %v, want %v", test.value, ok, test.ok)
			continue
		}
		var got []int
		for p := 0; p < periodsPerDay; p++ {
			if free[p] {
				got = append(got, p+1)
			}
		}
		if len(got) != len(test.want) {
			t.Errorf("decodeHours(%d) = %v, want %v", test.value, got, test.want)
			continue
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Errorf("decodeHours(%d) = %v, want %v", test.value, got, test.want)
			}
		}
	}
}

func TestLoneOneIsNeverReadAsPeriodFour(t *testing.T) {
	// One teacher has old-style 1111 data, the other was saved by the form with only period 1.
	oldStyle := teacherWithHours(1, "S1", 1111)
	fromForm := teacherWithHours(2, "S1", 1)

	teachers, warnings := buildPlanTeachers([]TeacherRow{oldStyle, fromForm})
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if teachers[0].FreeCount != daysPerWeek*periodsPerDay {
		t.Errorf("1111 should give every period, got %d", teachers[0].FreeCount)
	}
	for day := 0; day < daysPerWeek; day++ {
		if !teachers[1].Free[day][0] || teachers[1].Free[day][3] {
			t.Fatalf("hours 1 must mean period 1 only, day %d: %v", day, teachers[1].Free[day])
		}
	}
}

func TestTeacherAvailableInAllPeriodsHasTwentyPeriods(t *testing.T) {
	teachers, warnings := buildPlanTeachers([]TeacherRow{teacherWithHours(1, "S1", 1234)})
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(teachers) != 1 || teachers[0].FreeCount != 20 {
		t.Fatalf("a teacher free in every period of 5 days should have 20 periods, got %+v", teachers)
	}
}

func TestLegacyThursdayAvailabilityIsIgnoredSilently(t *testing.T) {
	row := teacherWithHours(1, "S1", 1234)
	row.teacherTimes = append(row.teacherTimes, TeacherTimes{Day: legacyThursday, Hour: 1234})

	teachers, warnings := buildPlanTeachers([]TeacherRow{row})
	if len(warnings) != 0 {
		t.Errorf("the old Thursday record must be ignored without a warning, got: %v", warnings)
	}
	if teachers[0].FreeCount != 20 {
		t.Errorf("Thursday must add no periods, got %d", teachers[0].FreeCount)
	}

	// a day that never existed is still reported
	row.teacherTimes = append(row.teacherTimes, TeacherTimes{Day: 7, Hour: 1234})
	_, warnings = buildPlanTeachers([]TeacherRow{row})
	if len(warnings) != 1 {
		t.Errorf("day 7 should give exactly one warning, got: %v", warnings)
	}
}

func TestSubjectDemandAndCapacity(t *testing.T) {
	teacherRows, lessonRows, classRows := scenarioRows(false)
	teachers, _ := buildPlanTeachers(teacherRows)
	tasks, _, problems := buildPlanTasks(classRows, lessonRows, teachers)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}

	info := subjectTotals(tasks, teachers)
	if len(info) != 6 {
		t.Fatalf("expected 6 subjects, got %d", len(info))
	}
	for key, subject := range info {
		// 9 classes x 3 periods
		if subject.Required != 27 {
			t.Errorf("subject %s: required = %d, want 27", key, subject.Required)
		}
		// 3 teachers x every period of the week
		if want := 3 * daysPerWeek * periodsPerDay; subject.Capacity != want {
			t.Errorf("subject %s: capacity = %d, want %d", key, subject.Capacity, want)
		}
	}

	if problems := checkPlanResources(tasks, teachers); len(problems) != 0 {
		t.Errorf("the scenario must pass the resource checks, got: %v", problems)
	}
}

func TestInsufficientCapacityIsReported(t *testing.T) {
	teacherRows, lessonRows, classRows := scenarioRows(false)
	// Give every S1 teacher one period per day: 3 teachers x 5 = 15 < 27.
	for i := range teacherRows {
		if len(teacherRows[i].Subject) > 0 && teacherRows[i].Subject[0] == "S1" {
			teacherRows[i] = teacherWithHours(teacherRows[i].ID, "S1", 1)
		}
	}
	plans, _, problems := generatePlan(teacherRows, lessonRows, classRows)
	if plans != nil || len(problems) == 0 {
		t.Fatalf("expected a failure with no timetable, got %d rows and problems %v", len(plans), problems)
	}
}

func checkScenarioPlan(t *testing.T, continuous bool) []PlanRow {
	teacherRows, lessonRows, classRows := scenarioRows(continuous)
	plans, _, problems := generatePlan(teacherRows, lessonRows, classRows)
	if len(problems) != 0 {
		t.Fatalf("generatePlan failed: %v", problems)
	}
	if len(plans) != 9*daysPerWeek*periodsPerDay {
		t.Fatalf("expected %d rows, got %d", 9*daysPerWeek*periodsPerDay, len(plans))
	}
	if problems := validatePlan(plans, teacherRows, lessonRows, classRows); len(problems) != 0 {
		t.Fatalf("validatePlan failed: %v", problems)
	}

	// only days 0..4 and periods 1..4, every class has exactly 20 slots, no slot twice
	slotsPerClass := map[[2]int]int{}
	seenSlot := map[[4]int]bool{}
	for _, row := range plans {
		if row.Day < 0 || row.Day > 4 || row.Hour < 1 || row.Hour > 4 {
			t.Fatalf("row outside the five-day week: %+v", row)
		}
		slotsPerClass[[2]int{row.Grade, row.ClassID}]++
		key := [4]int{row.Grade, row.ClassID, row.Day, row.Hour}
		if seenSlot[key] {
			t.Fatalf("slot used twice: %v", key)
		}
		seenSlot[key] = true
	}
	for _, classRow := range classRows {
		if got := slotsPerClass[[2]int{classRow.Grade, classRow.Grade_id}]; got != 20 {
			t.Errorf("grade %d class %d has %d slots, want 20", classRow.Grade, classRow.Grade_id, got)
		}
	}
	return plans
}

func TestScenarioCanBeScheduled(t *testing.T) {
	checkScenarioPlan(t, false)
}

// checkScenarioPlan also runs validatePlan, which checks that every continuous
// lesson is one teacher on one day in directly following periods.
func TestScenarioWithContinuousLessonsCanBeScheduled(t *testing.T) {
	checkScenarioPlan(t, true)
}

func TestClassWithExactlyTwentyPeriodsIsAccepted(t *testing.T) {
	teacherRows, lessonRows, classRows := scenarioRows(false)
	setLessonCounts(lessonRows, []int{4, 4, 3, 3, 3, 3}) // 20 periods per class

	teachers, _ := buildPlanTeachers(teacherRows)
	tasks, _, problems := buildPlanTasks(classRows, lessonRows, teachers)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if problems := checkPlanResources(tasks, teachers); len(problems) != 0 {
		t.Fatalf("exactly 20 periods must pass the capacity check, got: %v", problems)
	}

	plans, _, problems := generatePlan(teacherRows, lessonRows, classRows)
	if len(problems) != 0 {
		t.Fatalf("generatePlan failed: %v", problems)
	}
	for _, row := range plans {
		if row.LessonName == "" {
			t.Fatalf("a class with 20 periods must have no empty slot: %+v", row)
		}
	}
	if problems := validatePlan(plans, teacherRows, lessonRows, classRows); len(problems) != 0 {
		t.Fatalf("validatePlan failed: %v", problems)
	}
}

func TestClassWithMoreThanTwentyPeriodsIsRejected(t *testing.T) {
	for _, counts := range [][]int{{4, 4, 4, 3, 3, 3}, {4, 4, 4, 4, 4, 4}} { // 21 and 24 periods
		teacherRows, lessonRows, classRows := scenarioRows(false)
		setLessonCounts(lessonRows, counts)

		plans, _, problems := generatePlan(teacherRows, lessonRows, classRows)
		if plans != nil || !containsText(problems, "a week has only 20") {
			t.Errorf("counts %v: expected a rejection mentioning 20 periods, got %d rows and %v", counts, len(plans), problems)
		}
	}
}

func TestContinuousBlocksFit(t *testing.T) {
	tests := []struct {
		lengths []int
		want    bool
	}{
		{nil, true},
		{[]int{3, 3, 3, 3, 3}, true},
		{[]int{3, 3, 3, 3, 3, 3}, false}, // 6 blocks of 3 need 6 days
		{[]int{3, 3, 3, 3, 3, 2}, false}, // only 17 periods, but no day has room for 2 more
		{[]int{3, 3, 3, 3, 3, 1, 1, 1, 1, 1}, true},
		{[]int{2, 2, 2, 2, 2, 2, 2, 2, 2, 2}, true}, // 10 blocks of 2 fill 20 periods
		{[]int{4, 4, 4, 4, 4}, true},
		{[]int{4, 4, 4, 4, 4, 1}, false},
	}
	for _, test := range tests {
		if got := continuousBlocksFit(test.lengths); got != test.want {
			t.Errorf("continuousBlocksFit(%v) = %v, want %v", test.lengths, got, test.want)
		}
	}
}

func TestContinuousLessonsThatCannotFitAreReportedDirectly(t *testing.T) {
	teacherRows, lessonRows, classRows := scenarioRows(false)
	for i := range lessonRows {
		lessonRows[i].ScheduleType = true // 6 continuous lessons of 3 periods, but only 5 days
	}

	plans, _, problems := generatePlan(teacherRows, lessonRows, classRows)
	if plans != nil || !containsText(problems, "continuous lessons") {
		t.Fatalf("expected a direct continuous-lesson message, got %d rows and %v", len(plans), problems)
	}
	if containsText(problems, "search limit") {
		t.Errorf("the impossibility must be found before the search, got %v", problems)
	}
}

func TestValidateDetectsTeacherConflict(t *testing.T) {
	plans := checkScenarioPlan(t, false)
	teacherRows, lessonRows, classRows := scenarioRows(false)

	// Find two classes that have the same subject in the same period and give
	// them the same teacher.
	for i := range plans {
		for j := range plans {
			if i == j || plans[i].LessonName == "" || plans[i].LessonName != plans[j].LessonName {
				continue
			}
			if plans[i].Day != plans[j].Day || plans[i].Hour != plans[j].Hour || plans[i].TeacherID == plans[j].TeacherID {
				continue
			}
			plans[j].TeacherID = plans[i].TeacherID
			problems := validatePlan(plans, teacherRows, lessonRows, classRows)
			if !containsText(problems, "already teaches") {
				t.Fatalf("teacher conflict was not detected, problems: %v", problems)
			}
			return
		}
	}
	t.Skip("the generated timetable had no two classes with the same subject in the same period")
}

func TestValidateDetectsClassConflict(t *testing.T) {
	plans := checkScenarioPlan(t, false)
	teacherRows, lessonRows, classRows := scenarioRows(false)

	plans = append(plans, plans[0]) // the same class slot twice
	problems := validatePlan(plans, teacherRows, lessonRows, classRows)
	if !containsText(problems, "more than one entry") {
		t.Fatalf("class conflict was not detected, problems: %v", problems)
	}
}

func TestValidateDetectsTeacherOutsideAvailability(t *testing.T) {
	plans := checkScenarioPlan(t, false)
	teacherRows, lessonRows, classRows := scenarioRows(false)

	// Now every teacher is only available in periods 1 and 2, but the timetable uses periods 3 and 4 too.
	for i := range teacherRows {
		teacherRows[i].teacherTimes = nil

		for day := 0; day < daysPerWeek; day++ {
			teacherRows[i].teacherTimes = append(
				teacherRows[i].teacherTimes,
				TeacherTimes{Day: day, Hour: 12},
			)
		}
	}
	problems := validatePlan(plans, teacherRows, lessonRows, classRows)
	if !containsText(problems, "not available") {
		t.Fatalf("assignment outside availability was not detected, problems: %v", problems)
	}
}

func TestFailedPlanKeepsSavedTimetable(t *testing.T) {
	oldDB := db
	defer func() { db = oldDB }()

	var openError error
	db, openError = sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if openError != nil {
		t.Fatal(openError)
	}
	defer db.Close()

	createTables()

	statements := []string{
		"INSERT INTO teachers(id,name) VALUES(1,'T')",
		"INSERT INTO teacher_subject(teacher_id,subject) VALUES(1,'Math')",
		"INSERT INTO teacher_times(teacher_id,day,hours) VALUES(1,0,1234)",
		"INSERT INTO classes VALUES(1,10)",
		"INSERT INTO lessons VALUES('Math','10','8',0)", // needs 8 periods, but the teacher has only 4 in the whole week
		"INSERT INTO week(grade,class_id,day,hour,lesson_name,lesson_grade,teacher_id) VALUES(10,1,0,1,'Old','10',1)",
	}
	for _, statement := range statements {
		if _, execError := db.Exec(statement); execError != nil {
			t.Fatal(execError)
		}
	}

	makePlan() // must fail and must not touch the week table

	rows := collectWeekInformation()
	if len(rows) != 1 || rows[0].LessonName != "Old" {
		t.Fatalf("the saved timetable was changed: %+v", rows)
	}
}

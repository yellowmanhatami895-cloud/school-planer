package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// makeTeacher makes a teacher with the given subjects and availability records.
func makeTeacher(id int, subjects []string, times ...TeacherTimes) TeacherRow {
	return TeacherRow{ID: id, Name: "T" + string(rune('0'+id)), Subject: subjects, teacherTimes: times}
}

// scenarioRows builds the test school: 3 grades x 3 classes, 6 subjects,
// 3 periods per subject per grade (18 of the 24 weekly slots of a class),
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

// gradeTenOnly keeps one grade, so a test stays small and fast.
func gradeTenOnly(lessonRows []LessonsRow, classRows []ClassRow) ([]LessonsRow, []ClassRow) {
	var lessons []LessonsRow
	var classes []ClassRow
	for _, row := range lessonRows {
		if row.Grade == 10 {
			lessons = append(lessons, row)
		}
	}
	for _, row := range classRows {
		if row.Grade == 10 {
			classes = append(classes, row)
		}
	}
	return lessons, classes
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

// emptyPlanRows makes a complete timetable with every slot empty.
func emptyPlanRows(classRows []ClassRow) []PlanRow {
	var rows []PlanRow
	for _, classRow := range classRows {
		for d := 0; d < daysPerWeek; d++ {
			for h := 1; h <= periodsPerDay; h++ {
				rows = append(rows, PlanRow{Grade: classRow.Grade, ClassID: classRow.Grade_id, Day: d, Hour: h})
			}
		}
	}
	return rows
}

// setSlot fills one slot of a timetable made by emptyPlanRows.
func setSlot(t *testing.T, plans []PlanRow, grade, classID, day, hour int, lesson string, teacherID int) {
	t.Helper()
	for i := range plans {
		if plans[i].Grade == grade && plans[i].ClassID == classID && plans[i].Day == day && plans[i].Hour == hour {
			plans[i].LessonName = lesson
			plans[i].TeacherID = teacherID
			return
		}
	}
	t.Fatalf("slot not found: grade %d class %d day %d hour %d", grade, classID, day, hour)
}

// lessonRowsOf returns the non-empty rows of a timetable for one lesson.
func lessonRowsOf(plans []PlanRow, lesson string) []PlanRow {
	var rows []PlanRow
	for _, row := range plans {
		if row.LessonName == lesson {
			rows = append(rows, row)
		}
	}
	return rows
}

// openTestDB replaces the global db with a temporary database for one test.
func openTestDB(t *testing.T) {
	t.Helper()
	oldDB := db
	t.Cleanup(func() { db = oldDB })

	var openError error
	db, openError = sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if openError != nil {
		t.Fatal(openError)
	}
	t.Cleanup(func() { db.Close() })

	createTables()
}

func execAll(t *testing.T, statements []string) {
	t.Helper()
	for _, statement := range statements {
		if _, execError := db.Exec(statement); execError != nil {
			t.Fatal(execError)
		}
	}
}

// ---------------------------------------------------------------------------
// Week configuration and availability decoding
// ---------------------------------------------------------------------------

func TestWeekIsSixDaysOfFourPeriods(t *testing.T) {
	if daysPerWeek != 6 || periodsPerDay != 4 || daysPerWeek*periodsPerDay != 24 {
		t.Fatalf("expected 6 days x 4 periods = 24 slots, got %d x %d", daysPerWeek, periodsPerDay)
	}
	if planDayName(0) != "شنبه" || planDayName(4) != "چهارشنبه" || planDayName(5) != "پنجشنبه" {
		t.Errorf("day mapping is wrong: 0 = %s, 4 = %s, 5 = %s", planDayName(0), planDayName(4), planDayName(5))
	}
	if planDayName(6) != "day 6" {
		t.Errorf("day 6 must not be a school day, got %s", planDayName(6))
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

func TestTeacherAvailableInAllPeriodsHasTwentyFourPeriods(t *testing.T) {
	teachers, warnings := buildPlanTeachers([]TeacherRow{teacherWithHours(1, "S1", 1234)})
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(teachers) != 1 || teachers[0].FreeCount != 24 {
		t.Fatalf("a teacher free in every period of 6 days should have 24 periods, got %+v", teachers)
	}
}

func TestThursdayAvailabilityIsUsed(t *testing.T) {
	row := makeTeacher(1, []string{"S1"}, TeacherTimes{Day: 5, Hour: 1234})
	teachers, warnings := buildPlanTeachers([]TeacherRow{row})
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if teachers[0].FreeCount != 4 {
		t.Fatalf("Thursday (day 5) must give 4 periods, got %d", teachers[0].FreeCount)
	}
	for p := 0; p < periodsPerDay; p++ {
		if !teachers[0].Free[5][p] {
			t.Errorf("Thursday period %d must be free", p+1)
		}
	}

	// a day that never existed is reported
	row.teacherTimes = append(row.teacherTimes, TeacherTimes{Day: 6, Hour: 1234})
	_, warnings = buildPlanTeachers([]TeacherRow{row})
	if len(warnings) != 1 {
		t.Errorf("day 6 should give exactly one warning, got: %v", warnings)
	}
}

func TestMultipleAvailabilityRecordsOfOneDayAreCombined(t *testing.T) {
	row := makeTeacher(1, []string{"S1"}, TeacherTimes{Day: 0, Hour: 12}, TeacherTimes{Day: 0, Hour: 34})
	teachers, _ := buildPlanTeachers([]TeacherRow{row})
	if teachers[0].FreeCount != 4 {
		t.Fatalf("12 and 34 on the same day must give 4 periods, got %d", teachers[0].FreeCount)
	}
}

func TestMalformedAvailabilityIsIgnoredWithWarning(t *testing.T) {
	row := makeTeacher(1, []string{"S1"}, TeacherTimes{Day: 0, Hour: 55}, TeacherTimes{Day: 1, Hour: 1})
	teachers, warnings := buildPlanTeachers([]TeacherRow{row})
	if !containsText(warnings, "cannot be read") {
		t.Errorf("expected a warning for 55, got %v", warnings)
	}
	if teachers[0].FreeCount != 1 {
		t.Errorf("only the valid record should count, got %d", teachers[0].FreeCount)
	}
}

// ---------------------------------------------------------------------------
// Teachers and subjects
// ---------------------------------------------------------------------------

func TestSubjectNormalization(t *testing.T) {
	if subjectKey("دانش فنی") != subjectKey("دانش فنی ") || subjectKey(" S1") != subjectKey("s1 ") {
		t.Error("trailing/leading spaces and case must not make subjects different")
	}
	if subjectKey("علي") != subjectKey("علی") {
		t.Error("Arabic and Persian yeh must be the same")
	}
	if subjectKey("Math") == subjectKey("Maths") {
		t.Error("different subjects must stay different")
	}
}

func TestOneTeacherWithSeveralSubjectsIsOneTeacher(t *testing.T) {
	// Two rows with the same ID (as a query with one row per subject returns) plus
	// a duplicate and a padded subject.
	rows := []TeacherRow{
		makeTeacher(1, []string{"S1", "S1 "}, TeacherTimes{Day: 0, Hour: 12}),
		makeTeacher(1, []string{" S2", ""}),
	}
	teachers, _ := buildPlanTeachers(rows)
	if len(teachers) != 1 {
		t.Fatalf("expected one teacher, got %d", len(teachers))
	}
	if len(teachers[0].Subject) != 2 || !teacherHasSubject(&teachers[0], "S1") || !teacherHasSubject(&teachers[0], "S2") {
		t.Errorf("expected subjects S1 and S2 once each, got %v", teachers[0].Subject)
	}
	if teachers[0].FreeCount != 2 {
		t.Errorf("availability must be kept, got %d", teachers[0].FreeCount)
	}
}

func TestEligibleTeachersAreListedInTasks(t *testing.T) {
	rows := []TeacherRow{
		makeTeacher(1, []string{"S1", "S2"}, TeacherTimes{Day: 0, Hour: 1234}),
		makeTeacher(2, []string{"S2 "}, TeacherTimes{Day: 0, Hour: 1234}),
		makeTeacher(3, []string{"S1"}),                                   // no availability: not eligible
		makeTeacher(4, []string{"S3"}, TeacherTimes{Day: 0, Hour: 1234}), // other subject
	}
	teachers, _ := buildPlanTeachers(rows)
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 1}, {Name: "S2", Grade: 10, Count: 1}}

	tasks, _, problems := buildPlanTasks(classes, lessons, teachers)
	if len(problems) != 0 || len(tasks) != 2 {
		t.Fatalf("unexpected result: %d tasks, problems %v", len(tasks), problems)
	}
	if len(tasks[0].Teachers) != 1 || teachers[tasks[0].Teachers[0]].ID != 1 {
		t.Errorf("S1 must be eligible for teacher 1 only, got %v", tasks[0].Teachers)
	}
	if len(tasks[1].Teachers) != 2 {
		t.Errorf("S2 must be eligible for teachers 1 and 2, got %v", tasks[1].Teachers)
	}
}

func TestLessonWithoutQualifiedTeacherIsRejected(t *testing.T) {
	rows := []TeacherRow{teacherWithHours(1, "S1", 1234)}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S9", Grade: 10, Count: 1}}

	result := generatePlanResult(rows, lessons, classes)
	if result.Status != planProvenImpossible || result.Rows != nil || !containsText(result.Problems, "no teacher is registered") {
		t.Fatalf("expected a clear 'no teacher' failure, got %v %v", result.Status, result.Problems)
	}
}

func TestDuplicateLessonDefinitionIsInvalidData(t *testing.T) {
	rows := []TeacherRow{teacherWithHours(1, "S1", 1234)}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 1}, {Name: "S1 ", Grade: 10, Count: 2}}

	result := generatePlanResult(rows, lessons, classes)
	if result.Status != planInvalidData || !containsText(result.Problems, "more than once") {
		t.Fatalf("expected invalid data, got %v %v", result.Status, result.Problems)
	}
}

func TestZeroLessonCountIsInvalidData(t *testing.T) {
	rows := []TeacherRow{teacherWithHours(1, "S1", 1234)}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 0}}

	result := generatePlanResult(rows, lessons, classes)
	if result.Status != planInvalidData || result.Rows != nil {
		t.Fatalf("expected invalid data, got %v %v", result.Status, result.Problems)
	}
}

func TestEmptyDataIsHandled(t *testing.T) {
	result := generatePlanResult(nil, nil, nil)
	if result.Status == planSucceeded || result.Rows != nil {
		t.Fatalf("an empty school must not produce a successful timetable, got %v", result.Status)
	}
}

func TestTeacherWithTwoSubjectsCannotTeachTwoClassesAtOnce(t *testing.T) {
	classes := []ClassRow{{Grade: 10, Grade_id: 1}, {Grade: 11, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 1}, {Name: "S2", Grade: 11, Count: 1}}

	// One free period only: both classes would need it at the same time.
	oneSlot := []TeacherRow{makeTeacher(1, []string{"S1", "S2"}, TeacherTimes{Day: 0, Hour: 1})}
	result := generatePlanResult(oneSlot, lessons, classes)
	if result.Status != planSearchExhausted || result.Rows != nil {
		t.Fatalf("one teacher with one period cannot teach two classes, got %v %v", result.Status, result.Problems)
	}
	if !containsText(result.Problems, "every possible arrangement") {
		t.Errorf("a complete search must say so, got %v", result.Problems)
	}

	// Two free periods: possible, in two different periods, by the same teacher.
	twoSlots := []TeacherRow{makeTeacher(1, []string{"S1", "S2"}, TeacherTimes{Day: 0, Hour: 12})}
	result = generatePlanResult(twoSlots, lessons, classes)
	if result.Status != planSucceeded {
		t.Fatalf("expected success, got %v %v", result.Status, result.Problems)
	}
	if problems := validatePlan(result.Rows, twoSlots, lessons, classes); len(problems) != 0 {
		t.Fatalf("validatePlan failed: %v", problems)
	}
	first := lessonRowsOf(result.Rows, "S1")
	second := lessonRowsOf(result.Rows, "S2")
	if len(first) != 1 || len(second) != 1 || first[0].TeacherID != 1 || second[0].TeacherID != 1 {
		t.Fatalf("both lessons must be taught by teacher 1: %v %v", first, second)
	}
	if first[0].Day == second[0].Day && first[0].Hour == second[0].Hour {
		t.Fatalf("the teacher was placed twice in the same period: %v %v", first, second)
	}
}

// ---------------------------------------------------------------------------
// Capacity and preflight checks
// ---------------------------------------------------------------------------

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
	// Give every S1 teacher one period per day: 3 teachers x 6 = 18 < 27.
	for i := range teacherRows {
		if len(teacherRows[i].Subject) > 0 && teacherRows[i].Subject[0] == "S1" {
			teacherRows[i] = teacherWithHours(teacherRows[i].ID, "S1", 1)
		}
	}
	result := generatePlanResult(teacherRows, lessonRows, classRows)
	if result.Status != planProvenImpossible || result.Rows != nil || !containsText(result.Problems, "at most") {
		t.Fatalf("expected proven insufficient capacity, got %v %v", result.Status, result.Problems)
	}
}

func TestTeacherOnlyOnThursdayIsScheduledOnThursday(t *testing.T) {
	rows := []TeacherRow{makeTeacher(1, []string{"S1"}, TeacherTimes{Day: 5, Hour: 1234})}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 3}}

	result := generatePlanResult(rows, lessons, classes)
	if result.Status != planSucceeded {
		t.Fatalf("expected success, got %v %v", result.Status, result.Problems)
	}
	found := lessonRowsOf(result.Rows, "S1")
	if len(found) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(found))
	}
	for _, row := range found {
		if row.Day != 5 {
			t.Errorf("the teacher is only available on Thursday, got %+v", row)
		}
	}
}

// ---------------------------------------------------------------------------
// Complete timetables
// ---------------------------------------------------------------------------

func checkScenarioPlan(t *testing.T, continuous bool) []PlanRow {
	t.Helper()
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

	// only days 0..5 and periods 1..4, every class has exactly 24 slots, no slot twice
	slotsPerClass := map[[2]int]int{}
	seenSlot := map[[4]int]bool{}
	for _, row := range plans {
		if row.Day < 0 || row.Day > 5 || row.Hour < 1 || row.Hour > 4 {
			t.Fatalf("row outside the six-day week: %+v", row)
		}
		slotsPerClass[[2]int{row.Grade, row.ClassID}]++
		key := [4]int{row.Grade, row.ClassID, row.Day, row.Hour}
		if seenSlot[key] {
			t.Fatalf("slot used twice: %v", key)
		}
		seenSlot[key] = true
	}
	for _, classRow := range classRows {
		if got := slotsPerClass[[2]int{classRow.Grade, classRow.Grade_id}]; got != 24 {
			t.Errorf("grade %d class %d has %d slots, want 24", classRow.Grade, classRow.Grade_id, got)
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

func TestClassWithExactlyTwentyFourPeriodsIsAccepted(t *testing.T) {
	teacherRows, lessonRows, classRows := scenarioRows(false)
	lessonRows, classRows = gradeTenOnly(lessonRows, classRows)
	setLessonCounts(lessonRows, []int{4}) // 6 x 4 = 24 periods per class

	plans, _, problems := generatePlan(teacherRows, lessonRows, classRows)
	if len(problems) != 0 {
		t.Fatalf("generatePlan failed: %v", problems)
	}
	for _, row := range plans {
		if row.LessonName == "" {
			t.Fatalf("a class with 24 periods must have no empty slot: %+v", row)
		}
	}
	if problems := validatePlan(plans, teacherRows, lessonRows, classRows); len(problems) != 0 {
		t.Fatalf("validatePlan failed: %v", problems)
	}
}

func TestClassWithMoreThanTwentyFourPeriodsIsRejected(t *testing.T) {
	for _, counts := range [][]int{{5, 4, 4, 4, 4, 4}, {5, 5, 5, 5, 5, 5}} { // 25 and 30 periods
		teacherRows, lessonRows, classRows := scenarioRows(false)
		lessonRows, classRows = gradeTenOnly(lessonRows, classRows)
		setLessonCounts(lessonRows, counts)

		result := generatePlanResult(teacherRows, lessonRows, classRows)
		if result.Rows != nil || result.Status != planProvenImpossible || !containsText(result.Problems, "a week has only 24") {
			t.Errorf("counts %v: expected a rejection mentioning 24 periods, got %v %v", counts, result.Status, result.Problems)
		}
	}
}

func TestContinuousBlocksFit(t *testing.T) {
	tests := []struct {
		lengths []int
		want    bool
	}{
		{nil, true},
		{[]int{3, 3, 3, 3, 3, 3}, true},         // 6 blocks of 3 fit in 6 days
		{[]int{3, 3, 3, 3, 3, 3, 3}, false},     // 7 blocks of 3 need 7 days
		{[]int{3, 3, 3, 3, 3, 3, 2}, false},     // no day has room for 2 more
		{[]int{3, 3, 3, 3, 3, 3, 1, 1, 1, 1, 1, 1}, true},
		{[]int{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}, true}, // 12 blocks of 2 fill 24 periods
		{[]int{4, 4, 4, 4, 4, 4}, true},
		{[]int{4, 4, 4, 4, 4, 4, 1}, false},
		{[]int{4, 4, 4, 4, 4, 2}, true},
	}
	for _, test := range tests {
		if got := continuousBlocksFit(test.lengths); got != test.want {
			t.Errorf("continuousBlocksFit(%v) = %v, want %v", test.lengths, got, test.want)
		}
	}
}

func TestContinuousLessonsThatCannotFitAreReportedDirectly(t *testing.T) {
	teacherRows, lessonRows, classRows := scenarioRows(false)
	lessonRows, classRows = gradeTenOnly(lessonRows, classRows)
	setLessonCounts(lessonRows, []int{3})
	lessonRows = append(lessonRows, LessonsRow{Name: "S1", Grade: 11, Count: 1}) // other grade, has no class here
	for i := range lessonRows {
		if lessonRows[i].Grade == 10 {
			lessonRows[i].ScheduleType = true // 6 continuous lessons of 3 fit; make a 7th below
		}
	}
	teacherRows = append(teacherRows, teacherWithHours(100, "S7", 1234))
	lessonRows = append(lessonRows, LessonsRow{Name: "S7", Grade: 10, Count: 3, ScheduleType: true}) // 7 blocks of 3, only 6 days

	result := generatePlanResult(teacherRows, lessonRows, classRows)
	if result.Rows != nil || !containsText(result.Problems, "continuous lessons") {
		t.Fatalf("expected a direct continuous-lesson message, got %v %v", result.Status, result.Problems)
	}
	if containsText(result.Problems, "search limit") {
		t.Errorf("the impossibility must be found before the search, got %v", result.Problems)
	}
}

func TestContinuousLessonLongerThanADayIsRejected(t *testing.T) {
	rows := []TeacherRow{teacherWithHours(1, "S1", 1234)}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 5, ScheduleType: true}}

	result := generatePlanResult(rows, lessons, classes)
	if result.Rows != nil || !containsText(result.Problems, "a day has only 4") {
		t.Fatalf("expected a rejection, got %v %v", result.Status, result.Problems)
	}
}

func TestContinuousBlockOfFourPeriods(t *testing.T) {
	rows := []TeacherRow{teacherWithHours(1, "S1", 1234)}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 4, ScheduleType: true}}

	result := generatePlanResult(rows, lessons, classes)
	if result.Status != planSucceeded {
		t.Fatalf("expected success, got %v %v", result.Status, result.Problems)
	}
	found := lessonRowsOf(result.Rows, "S1")
	if len(found) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(found))
	}
	hours := map[int]bool{}
	for _, row := range found {
		if row.Day != found[0].Day {
			t.Fatalf("a continuous block must stay on one day: %v", found)
		}
		hours[row.Hour] = true
	}
	if len(hours) != 4 {
		t.Fatalf("the block must cover periods 1 to 4, got %v", found)
	}
}

func TestFallbackAllowsATeacherChange(t *testing.T) {
	// Neither teacher has two free periods, but together they have two.
	rows := []TeacherRow{
		makeTeacher(1, []string{"A"}, TeacherTimes{Day: 0, Hour: 1}),
		makeTeacher(2, []string{"A"}, TeacherTimes{Day: 0, Hour: 2}),
	}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "A", Grade: 10, Count: 2}}

	result := generatePlanResult(rows, lessons, classes)
	if result.Status != planSucceeded {
		t.Fatalf("expected success with a teacher change, got %v %v", result.Status, result.Problems)
	}
	if !containsText(result.Warnings, "different teachers") {
		t.Errorf("the teacher change must be reported as a warning, got %v", result.Warnings)
	}
	if problems := validatePlan(result.Rows, rows, lessons, classes); len(problems) != 0 {
		t.Fatalf("validatePlan failed: %v", problems)
	}
}

func TestFallbackStillRejectsAnImpossibleTimetable(t *testing.T) {
	// One teacher for both subjects with only 2 free periods, but 2 classes need 2 periods each.
	rows := []TeacherRow{makeTeacher(1, []string{"S1", "S2"}, TeacherTimes{Day: 0, Hour: 12})}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}, {Grade: 10, Grade_id: 2}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 1}, {Name: "S2", Grade: 10, Count: 1}}

	result := generatePlanResult(rows, lessons, classes)
	if result.Status != planSearchExhausted || result.Rows != nil {
		t.Fatalf("expected a complete search without a result, got %v %v", result.Status, result.Problems)
	}
}

func TestSearchLimitIsNotReportedAsImpossible(t *testing.T) {
	old := maxSearchSteps
	maxSearchSteps = 1
	defer func() { maxSearchSteps = old }()

	teacherRows, lessonRows, classRows := scenarioRows(false)
	result := generatePlanResult(teacherRows, lessonRows, classRows)
	if result.Status != planLimitReached || result.Rows != nil {
		t.Fatalf("expected the limit status, got %v", result.Status)
	}
	if !containsText(result.Problems, "does not prove") || containsText(result.Problems, "every possible arrangement") {
		t.Errorf("the message must not claim impossibility, got %v", result.Problems)
	}
}

// ---------------------------------------------------------------------------
// Independent validation
// ---------------------------------------------------------------------------

func validationFixture() ([]TeacherRow, []LessonsRow, []ClassRow) {
	teachers := []TeacherRow{teacherWithHours(1, "S1", 1234), teacherWithHours(2, "S2", 1234)}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}, {Grade: 10, Grade_id: 2}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 1}}
	return teachers, lessons, classes
}

func validFixturePlan(t *testing.T, classes []ClassRow) []PlanRow {
	plans := emptyPlanRows(classes)
	setSlot(t, plans, 10, 1, 0, 1, "S1", 1)
	setSlot(t, plans, 10, 2, 0, 2, "S1", 1)
	return plans
}

func TestValidateAcceptsACorrectPlan(t *testing.T) {
	teachers, lessons, classes := validationFixture()
	if problems := validatePlan(validFixturePlan(t, classes), teachers, lessons, classes); len(problems) != 0 {
		t.Fatalf("a correct plan was rejected: %v", problems)
	}
}

func TestValidateDetectsTeacherConflict(t *testing.T) {
	teachers, lessons, classes := validationFixture()
	plans := validFixturePlan(t, classes)
	setSlot(t, plans, 10, 2, 0, 2, "", 0)
	setSlot(t, plans, 10, 2, 0, 1, "S1", 1) // teacher 1 now teaches both classes in day 0 period 1

	problems := validatePlan(plans, teachers, lessons, classes)
	if !containsText(problems, "already teaches") {
		t.Fatalf("teacher conflict was not detected, problems: %v", problems)
	}
}

func TestValidateDetectsClassConflict(t *testing.T) {
	teachers, lessons, classes := validationFixture()
	plans := validFixturePlan(t, classes)
	plans = append(plans, plans[0]) // the same class slot twice

	problems := validatePlan(plans, teachers, lessons, classes)
	if !containsText(problems, "more than one entry") {
		t.Fatalf("class conflict was not detected, problems: %v", problems)
	}
}

func TestValidateDetectsUnqualifiedTeacher(t *testing.T) {
	teachers, lessons, classes := validationFixture()
	plans := validFixturePlan(t, classes)
	setSlot(t, plans, 10, 1, 0, 1, "S1", 2) // teacher 2 teaches S2 only

	problems := validatePlan(plans, teachers, lessons, classes)
	if !containsText(problems, "does not teach") {
		t.Fatalf("unqualified teacher was not detected, problems: %v", problems)
	}
}

func TestValidateDetectsWrongLessonCount(t *testing.T) {
	teachers, lessons, classes := validationFixture()
	plans := validFixturePlan(t, classes)
	setSlot(t, plans, 10, 1, 0, 1, "", 0) // class 1 lost its lesson

	problems := validatePlan(plans, teachers, lessons, classes)
	if !containsText(problems, "requires 1 periods, but 0 were assigned") {
		t.Fatalf("missing lesson was not detected, problems: %v", problems)
	}
}

func TestValidateDetectsMissingSlot(t *testing.T) {
	teachers, lessons, classes := validationFixture()
	plans := validFixturePlan(t, classes)[1:]

	problems := validatePlan(plans, teachers, lessons, classes)
	if !containsText(problems, "missing from the timetable") {
		t.Fatalf("missing slot was not detected, problems: %v", problems)
	}
}

func TestValidateDetectsBrokenContinuousBlock(t *testing.T) {
	teachers := []TeacherRow{teacherWithHours(1, "S1", 1234)}
	classes := []ClassRow{{Grade: 10, Grade_id: 1}}
	lessons := []LessonsRow{{Name: "S1", Grade: 10, Count: 2, ScheduleType: true}}

	plans := emptyPlanRows(classes)
	setSlot(t, plans, 10, 1, 0, 1, "S1", 1)
	setSlot(t, plans, 10, 1, 0, 3, "S1", 1) // a gap in the block
	if problems := validatePlan(plans, teachers, lessons, classes); !containsText(problems, "must be consecutive") {
		t.Fatalf("gap in a continuous block was not detected, problems: %v", problems)
	}

	setSlot(t, plans, 10, 1, 0, 3, "", 0)
	setSlot(t, plans, 10, 1, 1, 2, "S1", 1) // another day
	if problems := validatePlan(plans, teachers, lessons, classes); !containsText(problems, "must be consecutive") {
		t.Fatalf("block over two days was not detected, problems: %v", problems)
	}

	setSlot(t, plans, 10, 1, 1, 2, "", 0)
	setSlot(t, plans, 10, 1, 0, 2, "S1", 1) // correct
	if problems := validatePlan(plans, teachers, lessons, classes); len(problems) != 0 {
		t.Fatalf("a correct block was rejected: %v", problems)
	}
}

func TestValidateDetectsTeacherOutsideAvailability(t *testing.T) {
	plans := checkScenarioPlan(t, false)
	teacherRows, lessonRows, classRows := scenarioRows(false)

	// Now every teacher is only available in periods 1 and 2, but the timetable uses periods 3 and 4 too.
	for i := range teacherRows {
		teacherRows[i].teacherTimes = nil
		for day := 0; day < daysPerWeek; day++ {
			teacherRows[i].teacherTimes = append(teacherRows[i].teacherTimes, TeacherTimes{Day: day, Hour: 12})
		}
	}
	problems := validatePlan(plans, teacherRows, lessonRows, classRows)
	if !containsText(problems, "not available") {
		t.Fatalf("assignment outside availability was not detected, problems: %v", problems)
	}
}

// ---------------------------------------------------------------------------
// Database safety (isolated temporary database)
// ---------------------------------------------------------------------------

func seedSavedTimetable(t *testing.T) {
	execAll(t, []string{
		"INSERT INTO teachers(id,name) VALUES(1,'T')",
		"INSERT INTO teacher_subject(teacher_id,subject) VALUES(1,'Math')",
		"INSERT INTO teacher_times(teacher_id,day,hours) VALUES(1,0,1234)",
		"INSERT INTO classes VALUES(1,10)",
		"INSERT INTO lessons VALUES('Math','10','8',0)", // needs 8 periods, but the teacher has only 4 in the whole week
		"INSERT INTO week(grade,class_id,day,hour,lesson_name,lesson_grade,teacher_id) VALUES(10,1,0,1,'Old','10',1)",
	})
}

func TestFailedPlanKeepsSavedTimetable(t *testing.T) {
	openTestDB(t)
	seedSavedTimetable(t)

	result := generateAndSavePlan() // must fail and must not touch the week table
	if result.Status == planSucceeded {
		t.Fatalf("the impossible timetable was reported as a success")
	}

	rows := collectWeekInformation()
	if len(rows) != 1 || rows[0].LessonName != "Old" {
		t.Fatalf("the saved timetable was changed: %+v", rows)
	}
}

func TestInsertIntoWeekReplacesTheTimetable(t *testing.T) {
	openTestDB(t)
	seedSavedTimetable(t)

	err := InsertIntoWeek([]PlanRow{{Grade: 10, ClassID: 1, Day: 2, Hour: 3, LessonName: "Math", TeacherID: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rows := collectWeekInformation()
	if len(rows) != 1 || rows[0].LessonName != "Math" {
		t.Fatalf("the timetable was not replaced: %+v", rows)
	}
}

func TestFailedInsertRollsBackTheReplacement(t *testing.T) {
	openTestDB(t)
	seedSavedTimetable(t)
	// The database itself refuses the second row, after the old rows were deleted and the first row inserted.
	execAll(t, []string{
		"CREATE TRIGGER fail_bad BEFORE INSERT ON week WHEN NEW.lesson_name = 'BAD' BEGIN SELECT RAISE(ABORT, 'forced failure'); END",
	})

	err := InsertIntoWeek([]PlanRow{
		{Grade: 10, ClassID: 1, Day: 2, Hour: 3, LessonName: "Math", TeacherID: 1},
		{Grade: 10, ClassID: 1, Day: 2, Hour: 4, LessonName: "BAD", TeacherID: 1},
	})
	if err == nil {
		t.Fatal("the forced failure was not reported")
	}
	rows := collectWeekInformation()
	if len(rows) != 1 || rows[0].LessonName != "Old" {
		t.Fatalf("the failed replacement was not rolled back: %+v", rows)
	}
}

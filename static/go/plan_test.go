package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// teacherWithHours makes a teacher who has the same "hours" value on all 6 days.
func teacherWithHours(id int, subject string, hours int) TeacherRow {
	row := TeacherRow{ID: id, Name: "Teacher" + subject, Subject: subject}
	for day := 0; day < daysPerWeek; day++ {
		row.teacherTimes = append(row.teacherTimes, TeacherTimes{Day: day, Hour: hours})
	}
	return row
}

// scenarioRows builds the test school: 3 grades x 3 classes, 6 subjects,
// 4 periods per subject per grade, 3 teachers per subject, all hours "1111".
func scenarioRows(continuous bool) ([]TeacherRow, []LessonsRow, []ClassRow) {
	subjects := []string{"S1", "S2", "S3", "S4", "S5", "S6"}

	var teachers []TeacherRow
	id := 1
	for _, subject := range subjects {
		for k := 0; k < 3; k++ {
			teachers = append(teachers, teacherWithHours(id, subject, 1111))
			id++
		}
	}

	var lessons []LessonsRow
	var classes []ClassRow
	for _, grade := range []int{10, 11, 12} {
		for classID := 1; classID <= 3; classID++ {
			classes = append(classes, ClassRow{Grade: grade, Grade_id: classID})
		}
		for _, subject := range subjects {
			lessons = append(lessons, LessonsRow{Name: subject, Grade: grade, Count: 4, ScheduleType: continuous})
		}
	}
	return teachers, lessons, classes
}

func TestDecodeHours(t *testing.T) {
	tests := []struct {
		value int
		flags bool
		want  []int // available periods, numbered 1..4
		ok    bool
	}{
		{1111, true, []int{1, 2, 3, 4}, true},
		{1111, false, []int{1, 2, 3, 4}, true},
		{1234, false, []int{1, 2, 3, 4}, true},
		{124, false, []int{1, 2, 4}, true},
		{1011, true, []int{1, 3, 4}, true},
		{111, true, []int{2, 3, 4}, true}, // the leading zero of 0111 is lost in an INTEGER
		{0, false, nil, true},
		{1, false, []int{1}, true}, // period-list style
		{1, true, []int{4}, true},  // flag style: 0001
		{55, false, nil, false},
		{11111, false, nil, false},
		{-1, false, nil, false},
	}

	for _, test := range tests {
		free, ok := decodeHours(test.value, test.flags)
		if ok != test.ok {
			t.Errorf("decodeHours(%d, %v): ok = %v, want %v", test.value, test.flags, ok, test.ok)
			continue
		}
		var got []int
		for p := 0; p < periodsPerDay; p++ {
			if free[p] {
				got = append(got, p+1)
			}
		}
		if len(got) != len(test.want) {
			t.Errorf("decodeHours(%d, %v) = %v, want %v", test.value, test.flags, got, test.want)
			continue
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Errorf("decodeHours(%d, %v) = %v, want %v", test.value, test.flags, got, test.want)
			}
		}
	}
}

func TestTeacherWithSixDaysOf1111Has24Periods(t *testing.T) {
	teachers, warnings := buildPlanTeachers([]TeacherRow{teacherWithHours(1, "S1", 1111)})
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(teachers) != 1 || teachers[0].FreeCount != 24 {
		t.Fatalf("teacher with six days of 1111 should have 24 periods, got %+v", teachers)
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
		if subject.Required != 36 {
			t.Errorf("subject %s: required = %d, want 36", key, subject.Required)
		}
		if subject.Capacity != 72 {
			t.Errorf("subject %s: capacity = %d, want 72", key, subject.Capacity)
		}
	}

	if problems := checkPlanResources(tasks, teachers); len(problems) != 0 {
		t.Errorf("the scenario must pass the resource checks, got: %v", problems)
	}
}

func TestInsufficientCapacityIsReported(t *testing.T) {
	teacherRows, lessonRows, classRows := scenarioRows(false)
	// Give every S1 teacher only period 1 on every day: 3 teachers x 6 = 18 < 36.
	for i := range teacherRows {
		if teacherRows[i].Subject == "S1" {
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
	return plans
}

func TestScenarioCanBeScheduled(t *testing.T) {
	checkScenarioPlan(t, false)
}

func TestScenarioWithContinuousLessonsCanBeScheduled(t *testing.T) {
	checkScenarioPlan(t, true)
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
			for _, message := range problems {
				if strings.Contains(message, "already teaches") {
					return
				}
			}
			t.Fatalf("teacher conflict was not detected, problems: %v", problems)
		}
	}
	t.Skip("the generated timetable had no two classes with the same subject in the same period")
}

func TestValidateDetectsClassConflict(t *testing.T) {
	plans := checkScenarioPlan(t, false)
	teacherRows, lessonRows, classRows := scenarioRows(false)

	plans = append(plans, plans[0]) // the same class slot twice
	problems := validatePlan(plans, teacherRows, lessonRows, classRows)
	for _, message := range problems {
		if strings.Contains(message, "more than one entry") {
			return
		}
	}
	t.Fatalf("class conflict was not detected, problems: %v", problems)
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
		"INSERT INTO teachers(id,name,subject) VALUES(1,'T','Math')",
		"INSERT INTO teacher_times(teacher_id,day,hours) VALUES(1,0,1111)",
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

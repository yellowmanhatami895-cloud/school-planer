package main

import (
	"fmt"
	"net/http"
)

type errLessons struct {
	Error   []string
	Lessons []LessonsRow
}
type LessonsRow struct {
	Name         string
	Grade        int
	Count        int
	ScheduleType bool
}

func insertIntoLessons(inputs []string) {

	var isContinuous bool
	if inputs[3] == "false" {
		isContinuous = false
	} else {
		isContinuous = true
	}
	_, err := db.Exec("INSERT INTO lessons VALUES(?,?,?,?)", inputs[0], inputs[1], inputs[2], isContinuous)
	if err != nil {
		fmt.Println(err)

	}

}

func getLessonsRow() []LessonsRow {
	row, err := db.Query("SELECT * FROM lessons")
	if err != nil {
		fmt.Println(err)
		return []LessonsRow{}
	}
	defer row.Close()
	var lessons []LessonsRow
	var name string
	var grade, count int
	var isContinuous bool
	for row.Next() {
		err := row.Scan(&name, &grade, &count, &isContinuous)
		if err != nil {
			fmt.Println(err)
			return []LessonsRow{}
		}
		lessons = append(lessons, LessonsRow{
			Name:         name,
			Grade:        grade,
			Count:        count,
			ScheduleType: isContinuous,
		})
	}
	err = row.Err()
	if err != nil {
		fmt.Println(err)
		return []LessonsRow{}
	}

	return lessons
}
func deleteLessons(w http.ResponseWriter, r *http.Request) {
	inputs := getInputs(r, []string{"grade", "name"})
	db.Exec("DELETE FROM lessons WHERE class_grade = ? AND name = ?", inputs[0], inputs[1])

	http.Redirect(w, r, "/lessons", http.StatusSeeOther)
}

func collectLessonsInformation() []LessonsRow {
	var lessonsRows []LessonsRow

	lessonsRow, err := db.Query("SELECT name,class_grade,times,is_continuous FROM lessons")

	if err != nil {
		fmt.Println(err)
		return []LessonsRow{}
	}

	for lessonsRow.Next() {
		var name string
		var classGrade, times int
		var ScheduleType bool
		lessonsRow.Scan(&name, &classGrade, &times, &ScheduleType)
		lessonsRows = append(lessonsRows, LessonsRow{
			Name:         name,
			Grade:        classGrade,
			Count:        times,
			ScheduleType: ScheduleType,
		})
	}

	err = lessonsRow.Err()
	if err != nil {
		fmt.Println(err)
	}
	return lessonsRows
}

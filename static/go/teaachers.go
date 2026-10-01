package main

import (
	"fmt"
)

func InsertIntoTeacher(name string, subject string, week [][]string) {
	result, err := db.Exec("INSERT INTO teachers(name,subject) VALUES(?,?)", name, subject)
	if err != nil {
		fmt.Println(err)
		return
	}
	teacherID, err := result.LastInsertId()

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(teacherID)
	InsertIntoTeacherTimes(int(teacherID), week)
}
func InsertIntoTeacherTimes(teacherID int, week [][]string) {

	for i, hours := range week {

		var day string

		switch i {
		case 0:
			day = "saturday"
		case 1:
			day = "sunday"
		case 2:
			day = "monday"
		case 3:
			day = "tuesday"
		case 4:
			day = "wednesday"
		case 5:
			day = "thursday"
		}

		var hour string
		for _, h := range hours {
			hour = hour + h
		}
		_, err := db.Exec("INSERT INTO teacher_times(teacher_id, day, hours) VALUES(?, ?, ?)", teacherID, day, hour)
		if err != nil {
			fmt.Println(err)
		}
	}
}

package main

import "fmt"

type WeekRow struct {
	ID          int
	Grade       int
	ClassID     int
	Day         string
	Hour        string
	LessonName  string
	LessonGrade string
	TeacherID   int
}

func collectWeekInformation() []WeekRow {
	var weekRows []WeekRow

	weekRow, err := db.Query("SELECT id,grade,class_id,day,hour,lesson_name,lesson_grade,teacher_id FROM week")
	if err != nil {
		fmt.Println(err)
		return []WeekRow{}
	}
	defer weekRow.Close()

	for weekRow.Next() {
		var id int
		var grade int
		var classID int
		var day string
		var hour string
		var lessonName string
		var lessonGrade string
		var teacherID int

		err := weekRow.Scan(&id, &grade, &classID, &day, &hour, &lessonName, &lessonGrade, &teacherID)

		if err != nil {
			fmt.Println(err)
			return []WeekRow{}
		}

		weekRows = append(weekRows, WeekRow{
			ID:          id,
			Grade:       grade,
			ClassID:     classID,
			Day:         day,
			Hour:        hour,
			LessonName:  lessonName,
			LessonGrade: lessonGrade,
			TeacherID:   teacherID,
		})
	}

	err = weekRow.Err()
	if err != nil {
		fmt.Println(err)
		return []WeekRow{}
	}

	return weekRows
}

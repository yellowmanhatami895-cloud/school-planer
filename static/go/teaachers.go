package main

import (
	"fmt"
	"net/http"
)

type TeacherRow struct {
	ID      int
	Name    string
	Subject string
}

func InsertIntoTeacher(w http.ResponseWriter, name string, subject string, week [][]string) {
	result, err := db.Exec("INSERT INTO teachers(name,subject) VALUES(?,?)", name, subject)
	if err != nil {
		fmt.Println(err)
		sendErr(w, "خطا در وارد کردن اطلاعات", "../html/teacher")
		return
	}
	teacherID, err := result.LastInsertId()

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(teacherID)
	InsertIntoTeacherTimes(w, int(teacherID), week)
}
func InsertIntoTeacherTimes(w http.ResponseWriter, teacherID int, week [][]string) {

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
			sendErr(w, "خطا در وارد کردن اطلاعات", "../html/teacher")
		}
	}

}
func getTeachersRow(w http.ResponseWriter) []TeacherRow {
	row, err := db.Query("SELECT id,name,subject FROM teachers")
	if err != nil {
		fmt.Println(err)
		sendErr(w, "خطا در گرفتن کردن اطلاعات", "../html/teacher")
		return []TeacherRow{}
	}
	defer row.Close()
	var id int
	var name string
	var subject string
	var teachers []TeacherRow
	for row.Next() {
		err := row.Scan(&id, &name, &subject)
		if err != nil {
			fmt.Println(err)
			sendErr(w, "خطا در اسکن کردن اطلاعات", "../html/teacher")
			return []TeacherRow{}
		}
		teachers = append(teachers, TeacherRow{
			ID:      id,
			Name:    name,
			Subject: subject,
		})
	}
	err = row.Err()
	if err != nil {
		fmt.Println(err)

		sendErr(w, "خطا در اسکنن کردن اطلاعات", "../html/teacher")
		return []TeacherRow{}
	}

	return teachers
}
func deleteTeacher(w http.ResponseWriter, r *http.Request) {

	id := r.FormValue("id")

	_, err := db.Exec("DELETE FROM teacher_times WHERE teacher_id = ?", id)
	if err != nil {
		fmt.Println(err)
		sendErr(w, "خطا در پاک کردن اطلاعات", "../html/teacher")
		return
	}

	_, err = db.Exec("DELETE FROM teachers WHERE id = ?", id)
	if err != nil {
		fmt.Println(err)
		sendErr(w, "خطا در پاک کردن اطلاعات", "../html/teacher")
		return
	}

	http.Redirect(w, r, "/teachers#teachers-list", http.StatusSeeOther)
	fmt.Println(id, " deleted")
}

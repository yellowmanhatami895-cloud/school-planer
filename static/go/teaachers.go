package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
)

type TeacherRow struct {
	ID           int
	Name         string
	Subject      []string
	teacherTimes []TeacherTimes
}

func InsertIntoTeacher(w http.ResponseWriter, name string, subjects []string, week [][]string) {
	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer tx.Rollback()

	result, err := tx.Exec("INSERT INTO teachers(name) VALUES(?)", name)
	if err != nil {
		fmt.Println(err)
		return
	}

	teacherID, err := result.LastInsertId()
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, s := range subjects {
		_, err = tx.Exec("INSERT INTO teacher_subject(subject,teacher_id) VALUES(?,?)", s, teacherID)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	err = InsertIntoTeacherTimesTx(tx, int(teacherID), week)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println(err)
		return
	}
}

func deleteTeacherByErr(teacherID int) {
	_, err := db.Exec("DELETE FROM teacher_subject WHERE teacher_id = ?", teacherID)
	if err != nil {
		fmt.Println(err)
	}
	_, err = db.Exec("DELETE FROM teacher_times WHERE teacher_id = ?", teacherID)
	if err != nil {
		fmt.Println(err)
	}
	_, err = db.Exec("DELETE FROM teachers WHERE id = ?", teacherID)
	if err != nil {
		fmt.Println(err)
	}

}

func InsertIntoTeacherTimesTx(tx *sql.Tx, teacherID int, week [][]string) error {
	if len(week) > 5 {
		return fmt.Errorf("تعداد روزها نمی‌تواند بیشتر از 5 باشد")
	}

	for i, hours := range week {
		var hour string
		for _, h := range hours {
			hour += h
		}

		var hourInt int
		if hour != "" {
			var err error
			hourInt, err = strconv.Atoi(hour)
			if err != nil {
				return err
			}
		}

		_, err := tx.Exec(
			"INSERT INTO teacher_times(teacher_id, day, hours) VALUES(?, ?, ?)",
			teacherID, i, hourInt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func getTeachersRow(w http.ResponseWriter) []TeacherRow {
	row, err := db.Query("SELECT id,name FROM teachers")
	if err != nil {
		fmt.Println(err)

		return []TeacherRow{}
	}

	defer row.Close()
	var id int
	var name string
	var teachers []TeacherRow

	for row.Next() {
		var subjects []string
		err := row.Scan(&id, &name)
		if err != nil {
			fmt.Println(err)

			return []TeacherRow{}
		}
		rowSubjects, err := db.Query("SELECT subject FROM teacher_subject WHERE teacher_id = ?", id)
		if err != nil {
			fmt.Println(err)
			return []TeacherRow{}
		}

		for rowSubjects.Next() {
			var subject string

			err := rowSubjects.Scan(&subject)
			if err != nil {
				fmt.Println(err)
				rowSubjects.Close()
				return []TeacherRow{}
			}

			subjects = append(subjects, subject)
		}
		err = rowSubjects.Err()
		if err != nil {
			fmt.Println(err)
			return []TeacherRow{}
		}
		teachers = append(teachers, TeacherRow{
			ID:      id,
			Name:    name,
			Subject: subjects,
		})
		rowSubjects.Close()
	}
	err = row.Err()
	if err != nil {
		fmt.Println(err)

		return []TeacherRow{}
	}

	return teachers
}
func deleteTeacherByClick(w http.ResponseWriter, r *http.Request) {

	id := r.FormValue("id")

	_, err := db.Exec("DELETE FROM teacher_times WHERE teacher_id = ?", id)
	if err != nil {
		fmt.Println(err)

		return
	}
	_, err = db.Exec("DELETE FROM teacher_subject WHERE teacher_id = ?", id)
	if err != nil {
		fmt.Println(err)

		return
	}
	_, err = db.Exec("DELETE FROM teachers WHERE id = ?", id)
	if err != nil {
		fmt.Println(err)

		return
	}

	http.Redirect(w, r, "/teachers#teachers-list", http.StatusSeeOther)
	fmt.Println(id, " deleted")
}
func collectTeacherInformation() []TeacherRow {
	var teacherRows []TeacherRow

	teacherRow, err := db.Query("SELECT id,name FROM teachers")
	if err != nil {
		fmt.Println(err)
		return []TeacherRow{}
	}
	defer teacherRow.Close()
	for teacherRow.Next() {
		var teacherTimes []TeacherTimes
		var teacherID int
		var teacherName string

		var teacherDay int
		var teacherHours int
		var subjects []string
		err := teacherRow.Scan(&teacherID, &teacherName)
		if err != nil {
			fmt.Println(err)
			return []TeacherRow{}
		}
		teacherTimesRow, err := db.Query("SELECT day,hours FROM teacher_times WHERE teacher_id = ? ", teacherID)
		if err != nil {
			fmt.Println(err)
			return []TeacherRow{}
		}

		subjectRow, err := db.Query("SELECT subject FROM teacher_subject WHERE teacher_id = ?", teacherID)
		if err != nil {
			fmt.Println(err)
			teacherTimesRow.Close()
			return []TeacherRow{}
		}

		for teacherTimesRow.Next() {
			err := teacherTimesRow.Scan(&teacherDay, &teacherHours)
			if err != nil {
				fmt.Println(err)
				teacherTimesRow.Close()
				subjectRow.Close()
				return []TeacherRow{}
			}
			teacherTimes = append(teacherTimes, TeacherTimes{
				Day:  teacherDay,
				Hour: teacherHours,
			})
		}
		err = teacherTimesRow.Err()
		if err != nil {
			fmt.Println(err)
			teacherTimesRow.Close()
			subjectRow.Close()
			return []TeacherRow{}
		}

		for subjectRow.Next() {
			var subject string
			err := subjectRow.Scan(&subject)
			if err != nil {
				fmt.Println(err)
				teacherTimesRow.Close()
				subjectRow.Close()
				return []TeacherRow{}
			}
			subjects = append(subjects, subject)
		}
		err = subjectRow.Err()
		if err != nil {
			fmt.Println(err)
			teacherTimesRow.Close()
			subjectRow.Close()
			return []TeacherRow{}
		}
		teacherRows = append(teacherRows, TeacherRow{
			Name:         teacherName,
			Subject:      subjects,
			ID:           teacherID,
			teacherTimes: teacherTimes,
		})

		teacherTimesRow.Close()
		subjectRow.Close()

	}
	err = teacherRow.Err()
	if err != nil {
		fmt.Println(err)
		return []TeacherRow{}
	}
	return teacherRows
}

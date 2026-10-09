package main

import (
	"fmt"
	"net/http"
)

type errClass struct {
	Error []string
	Class []ClassRow
}
type ClassRow struct {
	Grade    int
	Grade_id int
}

func insertIntoClass(grade int, count int) {
	for i := 1; i <= count; i++ {
		_, err := db.Exec("INSERT INTO classes VALUES(?,?)", i, grade)
		if err != nil {
			fmt.Println(err)

			return
		}
	}
}
func getClassRow() []ClassRow {
	rows, err := db.Query("SELECT grade, grade_id FROM classes ORDER BY grade, grade_id")
	if err != nil {
		fmt.Println(err)
		return []ClassRow{}
	}
	defer rows.Close()

	var classes []ClassRow

	for rows.Next() {

		var grade int
		var gradeID int

		err := rows.Scan(&grade, &gradeID)
		if err != nil {
			fmt.Println(err)
			continue
		}

		classes = append(classes, ClassRow{
			Grade:    grade,
			Grade_id: gradeID,
		})
	}

	return classes
}
func deleteClass(w http.ResponseWriter, r *http.Request) {

	grade := r.FormValue("grade")
	gradeID := r.FormValue("id")
	_, err := db.Exec("DELETE FROM classes WHERE grade = ? AND grade_id = ?", grade, gradeID)
	if err != nil {
		fmt.Println(err)
		return
	}

	http.Redirect(w, r, "/class#class-list", http.StatusSeeOther)
	fmt.Println(grade, gradeID, " deleted")
}
func collectClassInformation() []ClassRow {
	var classRows []ClassRow

	classRow, err := db.Query("SELECT grade,grade_id FROM classes")
	if err != nil {
		fmt.Println(err)
		return []ClassRow{}
	}
	defer classRow.Close()

	for classRow.Next() {
		var grade, gradeID int
		classRow.Scan(&grade, &gradeID)
		classRows = append(classRows, ClassRow{
			Grade:    grade,
			Grade_id: gradeID,
		})
	}
	err = classRow.Err()

	if err != nil {
		fmt.Println(err)
		return []ClassRow{}
	}
	return classRows
}

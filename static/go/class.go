package main

import (
	"fmt"
	"net/http"
	"strconv"
)

type errClass struct {
	Error []string
	Class []ClassRow
}
type ClassRow struct {
	Grade    string
	Grade_id string
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
			Grade:    strconv.Itoa(grade),
			Grade_id: strconv.Itoa(gradeID),
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

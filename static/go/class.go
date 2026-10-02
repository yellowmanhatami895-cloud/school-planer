package main

import (
	"fmt"
	"net/http"
	"strconv"
	"text/template"
)

type errClass struct {
	Error string
	Class ClassRow
}
type ClassRow struct {
	Grade    string
	Grade_id string
}

func insertIntoClass(w http.ResponseWriter, grade int, count int) {
	for i := 1; i <= count; i++ {
		_, err := db.Exec("INSERT INTO class VALUE(?,?)", i, grade)
		if err != nil {
			fmt.Println(err)
			sendErr(w, "نمیتوان کلاس را وارد دیتا بیس کرد", "../html/class")
		}
	}
}
func getClassRow(w http.ResponseWriter) []ClassRow {
	row, err := db.Query("SELECT * FROM classes")
	if err != nil {
		fmt.Println(err)
		sendErr(w, "نمی توان از دیتا بیس کلاس ها را دریافت کرد", "../html/class")
		return []ClassRow{}
	}
	defer row.Close()
	var gradeID, grade int
	var gradeIDStr, gradeStr string
	var class []ClassRow
	for row.Next() {
		err := row.Scan(&gradeID, &grade)
		if err != nil {
			fmt.Println(err)
			sendErr(w, "مشکل دز اسکن اطلاعات داخل دیتا بیس", "../html/class")
			return []ClassRow{}
		}
		switch grade {
		case 10:
			gradeStr = "دهم"
		case 11:
			gradeStr = "یازدهم"
		case 12:
			gradeStr = "دوازدهم"
		}
		gradeIDStr = strconv.Itoa(gradeID)
		if err != nil {
			fmt.Println(err)
			sendErr(w, "مشکل در اسکن اطاعات داخل دیتا بیس", "../html/class")
		}
		class = append(class, ClassRow{
			Grade_id: gradeIDStr,
			Grade:    gradeStr,
		})
	}
	err = row.Err()

	if err != nil {
		fmt.Println(err)
		sendErr(w, "مشکل در اسکن اطاعات داخل دیتا بیس", "../html/class")
		return []ClassRow{}
	}

	return class
}
func sendErr(w http.ResponseWriter, errText string, file string) {
	tmpl, err := template.ParseFiles(file)

	if err != nil {
		fmt.Println(err)
		return
	}

	err = tmpl.Execute(w, errText)

	if err != nil {
		fmt.Println(err)
	}
}

package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"text/template"
)

var db *sql.DB
var err error

type TeacherERR struct {
	Error   []string
	Teacher []TeacherRow
}

type TeacherTimes struct {
	Day  string
	Hour string
}

func getInputs(r *http.Request, names []string) []string {
	var values []string
	for _, name := range names {
		value := r.FormValue(name)
		values = append(values, value)
	}
	return values
}
func getMultipleInputs(r *http.Request, names []string) [][]string {
	var values [][]string
	for _, name := range names {
		value := r.Form[name]
		values = append(values, value)
	}
	return values
}
func hasNoInput(inputs [][]string) bool {
	for _, input := range inputs {
		if len(input) > 0 {
			return false
		}
	}

	return true
}
func main() {
	db, err = sql.Open("sqlite", "../DB/DataBase.db")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	createTables()

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("../css"))))
	http.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir("../img"))))
	http.Handle("/html/", http.StripPrefix("/html/", http.FileServer(http.Dir("../html"))))

	http.HandleFunc("/teachers", teacher)
	http.HandleFunc("/class", class)
	http.HandleFunc("/plan", plan)
	http.HandleFunc("/lessons", lessons)
	http.HandleFunc("/home", Home)
	http.HandleFunc("/deleteTeacher", deleteTeacher)
	http.HandleFunc("/deleteClass", deleteClass)
	http.HandleFunc("/deleteLessons", deleteLessons)
	fmt.Println(":8080")
	http.ListenAndServe(":8080", nil)

}

func Home(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../html/main.html")
}

func teacher(w http.ResponseWriter, r *http.Request) {
	var data TeacherERR

	if r.Method == http.MethodPost {
		inputs := getInputs(r, []string{"name", "subject"})
		multipleInputs := getMultipleInputs(r, []string{"saturday", "sunday", "monday", "tuesday", "wednesday", "thursday"})

		if inputs[0] == "" {
			data.Error = append(data.Error, "نام معلم را وارد کنید")
		}

		if inputs[1] == "" {
			data.Error = append(data.Error, "موضوع درس را وارد کنید")
		}

		if hasNoInput(multipleInputs) {
			data.Error = append(data.Error, "حداقل یک ساعت را انتخواب کنید")
		}

		if len(data.Error) == 0 {
			InsertIntoTeacher(w, inputs[0], inputs[1], [][]string{multipleInputs[0], multipleInputs[1], multipleInputs[2], multipleInputs[3], multipleInputs[4], multipleInputs[5]})
			http.Redirect(w, r, "/teachers", http.StatusSeeOther)
			return
		}
	}

	data.Teacher = getTeachersRow(w)

	tmpl, err := template.ParseFiles("../html/teacher.html")
	if err != nil {
		fmt.Println(err)
		return
	}

	tmpl.Execute(w, data)

}

func class(w http.ResponseWriter, r *http.Request) {
	var data errClass

	if r.Method == http.MethodPost {

		inputs := getInputs(r, []string{"grade", "count"})

		gradeInt, err := strconv.Atoi(inputs[0])
		if err != nil {
			fmt.Println(err)
			data.Error = append(data.Error, "یک پایه را انتخاب کنید")
		}

		countInt, err := strconv.Atoi(inputs[1])
		if err != nil {
			fmt.Println(err)
			data.Error = append(data.Error, "تعداد را وارد کنید")
		}

		if len(data.Error) == 0 {
			insertIntoClass(gradeInt, countInt)
		}
	}

	data.Class = getClassRow()

	funcMap := template.FuncMap{
		"firstGrade": func(classes []ClassRow, index int) bool {
			if index == 0 {
				return true
			}

			return classes[index-1].Grade != classes[index].Grade
		},
	}

	tmpl, err := template.New("class.html").Funcs(funcMap).ParseFiles("../html/class.html")
	if err != nil {
		fmt.Println(err)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println(err)
	}
}

func plan(w http.ResponseWriter, r *http.Request) {
	makePlan()
	http.ServeFile(w, r, "../html/plan.html")
}
func lessons(w http.ResponseWriter, r *http.Request) {
	var data errLessons
	if r.Method == http.MethodPost {

		inputs := getInputs(r, []string{"name", "grade", "count", "scheduleType"})
		if inputs[0] == "" {
			data.Error = append(data.Error, "اسم درس را وارد کنید")
		}
		if inputs[1] == "" {
			data.Error = append(data.Error, "پایه را انتخواب کنید")
		}
		if inputs[2] == "" {
			data.Error = append(data.Error, "تعداد زنگ های هفته را وارد کنید")
		}
		if len(data.Error) == 0 {
			insertIntoLessons(inputs)
			http.Redirect(w, r, "/lessons", http.StatusSeeOther)
			return
		}
	}

	tmpl, err := template.ParseFiles("../html/lessons.html")
	if err != nil {
		fmt.Println(err)
		return
	}
	data.Lessons = getLessonsRow()
	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println(err)

	}
}
func makePlan() {
	// teacherRows := collectTeacherInformation()
}

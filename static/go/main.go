package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"text/template"
)

var db *sql.DB
var err error

type ERR struct {
	Error   []string
	Teacher []Teacher
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

	fmt.Println(":8080")
	http.ListenAndServe(":8080", noCache(http.DefaultServeMux))

}

func Home(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../html/main.html")
}

func teacher(w http.ResponseWriter, r *http.Request) {
	var data ERR

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
			InsertIntoTeacher(inputs[0], inputs[1], [][]string{multipleInputs[0], multipleInputs[1], multipleInputs[2], multipleInputs[3], multipleInputs[4], multipleInputs[5]})
			http.Redirect(w, r, "/teachers", http.StatusSeeOther)
			return
		}
	}

	data.Teacher = getTeachersRow()

	tmpl, err := template.ParseFiles("../html/teacher.html")
	if err != nil {
		fmt.Println(err)
		return
	}

	tmpl.Execute(w, data)

}

func class(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		grade := r.FormValue("grade")
		count := r.FormValue("count")
		fmt.Println(grade, count)
	}

	http.ServeFile(w, r, "../html/class.html")

}

func plan(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../html/plan.html")
}
func lessons(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../html/lessons.html")
}
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		next.ServeHTTP(w, r)
	})
}

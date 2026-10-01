package main

import (
	"database/sql"
	"fmt"
	"net/http"
)

var db *sql.DB
var err error

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

	fmt.Println(":8080")
	http.ListenAndServe(":8080", noCache(http.DefaultServeMux))

}

func Home(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../html/main.html")
}

func teacher(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		subject := r.FormValue("subject")
		saturday := r.Form["saturday"]
		sunday := r.Form["sunday"]
		monday := r.Form["monday"]
		tuesday := r.Form["tuesday"]
		wednesday := r.Form["wednesday"]
		thursday := r.Form["thursday"]
		InsertIntoTeacher(name, subject, [][]string{saturday, sunday, monday, tuesday, wednesday, thursday})
	}

	http.ServeFile(w, r, "../html/teacher.html")
}

func class(w http.ResponseWriter, r *http.Request) {
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

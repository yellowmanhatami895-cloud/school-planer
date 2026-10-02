package main

import (
	"fmt"

	_ "github.com/glebarez/go-sqlite"
)

func createTables() {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS teachers (id INTEGER PRIMARY KEY,name TEXT NOT NULL,subject TEXT NOT NULL);")
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS teacher_times (id INTEGER PRIMARY KEY,teacher_id INTEGER,day TEXT NOT NULL,hours TEXT NOT NULL,FOREIGN KEY (teacher_id) REFERENCES teachers(id));")

	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS classes (grade_id INTEGER,grade INTEGER NOT NULL,PRIMARY KEY (grade, grade_id));")

	if err != nil {
		fmt.Println(err)
		return
	}
}

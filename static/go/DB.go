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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS lessons (name TEXT NOT NULL,class_grade TEXT,times TEXT,is_continuous bool,PRIMARY KEY(name,class_grade));")

	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS week (id INTEGER PRIMARY KEY,grade INTEGER,class_id INTEGER,day TEXT NOT NULL,hour TEXT,lesson_name TEXT,lesson_grade TEXT,teacher_id INTEGER,FOREIGN KEY (grade, class_id) REFERENCES classes(grade, grade_id),FOREIGN KEY (lesson_name, lesson_grade) REFERENCES lessons(name, class_grade),FOREIGN KEY (teacher_id) REFERENCES teachers(id));")

	if err != nil {
		fmt.Println(err)
		return
	}
}

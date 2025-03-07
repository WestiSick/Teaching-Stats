package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// Application constants related to the database
const (
	DBConnectionString = "./teaching_stats.db?_busy_timeout=5000"
)

// InitDB initializes the database and creates tables if they don't exist
func InitDB() *sql.DB {
	db, err := sql.Open("sqlite3", DBConnectionString)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fio TEXT NOT NULL,
			login TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			role TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS lessons (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			teacher_id INTEGER,
			group_name TEXT NOT NULL,
			subject TEXT NOT NULL,
			topic TEXT NOT NULL,
			hours INTEGER NOT NULL,
			date TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'Лекция',
			FOREIGN KEY (teacher_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			action TEXT NOT NULL,
			details TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS students (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			teacher_id INTEGER,
			group_name TEXT NOT NULL,
			student_fio TEXT NOT NULL,
			FOREIGN KEY (teacher_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS attendance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			lesson_id INTEGER,
			student_id INTEGER,
			attended INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (lesson_id) REFERENCES lessons(id),
			FOREIGN KEY (student_id) REFERENCES students(id),
			UNIQUE (lesson_id, student_id)
		);
		CREATE TABLE IF NOT EXISTS lab_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			teacher_id INTEGER,
			group_name TEXT NOT NULL,
			subject TEXT NOT NULL,
			total_labs INTEGER NOT NULL DEFAULT 5,
			FOREIGN KEY (teacher_id) REFERENCES users(id),
			UNIQUE (teacher_id, group_name, subject)
		);
		CREATE TABLE IF NOT EXISTS lab_grades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			student_id INTEGER,
			teacher_id INTEGER,
			subject TEXT NOT NULL,
			lab_number INTEGER NOT NULL,
			grade INTEGER NOT NULL,
			FOREIGN KEY (student_id) REFERENCES students(id),
			FOREIGN KEY (teacher_id) REFERENCES users(id),
			UNIQUE (student_id, subject, lab_number)
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_attendance_lesson_student ON attendance (lesson_id, student_id)")
	if err != nil {
		log.Printf("Error creating unique index: %v", err)
	}

	return db
}

// ExecuteQuery executes a database query and logs any errors
func ExecuteQuery(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	result, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("Database error: %v\nQuery: %s\nArgs: %v", err, query, args)
	}
	return result, err
}

// LogAction records an action in the system log
func LogAction(db *sql.DB, userID int, action, details string) {
	_, err := ExecuteQuery(db, "INSERT INTO logs (user_id, action, details) VALUES (?, ?, ?)",
		userID, action, details)
	if err != nil {
		log.Printf("Error logging action: %v", err)
	}
}

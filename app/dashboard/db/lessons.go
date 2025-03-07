package db

import (
	"database/sql"
)

// LessonData stores information about a lesson
type LessonData struct {
	ID      int
	Group   string
	Subject string
	Topic   string
	Hours   int
	Date    string
	Type    string
}

// GetLessonsBySubject retrieves lessons for a specific teacher and subject
func GetLessonsBySubject(db *sql.DB, teacherID int, subject string) ([]LessonData, error) {
	rows, err := db.Query(`
		SELECT id, group_name, topic, hours, date, type 
		FROM lessons 
		WHERE teacher_id = ? AND subject = ? 
		ORDER BY date`, teacherID, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []LessonData
	for rows.Next() {
		var l LessonData
		var dateStr string
		rows.Scan(&l.ID, &l.Group, &l.Topic, &l.Hours, &dateStr, &l.Type)
		l.Subject = subject
		l.Date = dateStr
		lessons = append(lessons, l)
	}
	return lessons, nil
}

// GetLessonByID retrieves a specific lesson by ID
func GetLessonByID(db *sql.DB, lessonID int, teacherID int) (LessonData, error) {
	var lesson LessonData
	err := db.QueryRow(`
		SELECT id, group_name, subject, topic, hours, date, type 
		FROM lessons 
		WHERE id = ? AND teacher_id = ?`, lessonID, teacherID).
		Scan(&lesson.ID, &lesson.Group, &lesson.Subject, &lesson.Topic, &lesson.Hours, &lesson.Date, &lesson.Type)
	return lesson, err
}

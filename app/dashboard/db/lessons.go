package db

import (
	"TeacherJournal/app/dashboard/models"

	"gorm.io/gorm"
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
func GetLessonsBySubject(db *gorm.DB, teacherID int, subject string) ([]LessonData, error) {
	var lessons []LessonData

	err := db.Model(&models.Lesson{}).
		Select("id, group_name as `group`, subject, topic, hours, date, type").
		Where("teacher_id = ? AND subject = ?", teacherID, subject).
		Order("date").
		Find(&lessons).Error

	return lessons, err
}

// GetLessonByID retrieves a specific lesson by ID
func GetLessonByID(db *gorm.DB, lessonID int, teacherID int) (LessonData, error) {
	var lesson LessonData

	err := db.Model(&models.Lesson{}).
		Select("id, group_name as `group`, subject, topic, hours, date, type").
		Where("id = ? AND teacher_id = ?", lessonID, teacherID).
		First(&lesson).Error

	return lesson, err
}

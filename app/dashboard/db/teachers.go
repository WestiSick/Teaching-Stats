package db

import (
	"TeacherJournal/app/dashboard/models"

	"gorm.io/gorm"
)

// GetTeacherGroups gets a list of groups for the specified teacher
func GetTeacherGroups(db *gorm.DB, teacherID int) ([]string, error) {
	var groups []string

	// Use raw SQL for complex union query
	err := db.Raw(`
		SELECT DISTINCT group_name 
		FROM (
			SELECT group_name FROM lessons WHERE teacher_id = ? 
			UNION 
			SELECT group_name FROM students WHERE teacher_id = ?
		) AS combined_groups 
		ORDER BY group_name
	`, teacherID, teacherID).Scan(&groups).Error

	return groups, err
}

// GetTeacherSubjects gets a list of subjects for the specified teacher
func GetTeacherSubjects(db *gorm.DB, teacherID int) ([]string, error) {
	var subjects []string

	err := db.Model(&models.Lesson{}).
		Distinct("subject").
		Where("teacher_id = ?", teacherID).
		Order("subject").
		Pluck("subject", &subjects).Error

	return subjects, err
}

// TeacherStats stores statistics about a teacher
type TeacherStats struct {
	ID       int
	FIO      string
	Lessons  int
	Hours    int
	Subjects map[string]int
}

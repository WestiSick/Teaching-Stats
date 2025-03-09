package db

import (
	"TeacherJournal/app/dashboard/models"

	"gorm.io/gorm"
)

// StudentData stores information about a student
type StudentData struct {
	ID  int
	FIO string
}

// GroupData stores information about a group
type GroupData struct {
	Name         string
	StudentCount int
}

// GetStudentsInGroup retrieves all students in a specific group
func GetStudentsInGroup(db *gorm.DB, teacherID int, groupName string) ([]StudentData, error) {
	var students []StudentData

	err := db.Model(&models.Student{}).
		Select("id, student_fio as fio").
		Where("teacher_id = ? AND group_name = ?", teacherID, groupName).
		Order("student_fio").
		Find(&students).Error

	return students, err
}

// GetGroupsByTeacher retrieves all groups for a specific teacher
func GetGroupsByTeacher(db *gorm.DB, teacherID int) ([]GroupData, error) {
	var groups []GroupData

	// Get unique group names from both lessons and students tables
	var groupNames []string
	subQuery := db.Raw(`
		SELECT DISTINCT group_name FROM (
			SELECT group_name FROM lessons WHERE teacher_id = ?
			UNION
			SELECT group_name FROM students WHERE teacher_id = ?
		) AS combined_groups
		ORDER BY group_name
	`, teacherID, teacherID)

	err := subQuery.Scan(&groupNames).Error
	if err != nil {
		return nil, err
	}

	// For each group, count students
	for _, groupName := range groupNames {
		var count int64
		err := db.Model(&models.Student{}).
			Where("teacher_id = ? AND group_name = ?", teacherID, groupName).
			Count(&count).Error

		if err != nil {
			return nil, err
		}

		groups = append(groups, GroupData{
			Name:         groupName,
			StudentCount: int(count),
		})
	}

	return groups, nil
}

package db

import (
	"database/sql"
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
func GetStudentsInGroup(db *sql.DB, teacherID int, groupName string) ([]StudentData, error) {
	rows, err := db.Query(
		"SELECT id, student_fio FROM students WHERE teacher_id = ? AND group_name = ? ORDER BY student_fio",
		teacherID, groupName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []StudentData
	for rows.Next() {
		var s StudentData
		rows.Scan(&s.ID, &s.FIO)
		students = append(students, s)
	}
	return students, nil
}

// GetGroupsByTeacher retrieves all groups for a specific teacher
func GetGroupsByTeacher(db *sql.DB, teacherID int) ([]GroupData, error) {
	rows, err := db.Query(`
		SELECT DISTINCT group_name 
		FROM (
			SELECT group_name FROM lessons WHERE teacher_id = ? 
			UNION 
			SELECT group_name FROM students WHERE teacher_id = ?
		) ORDER BY group_name`,
		teacherID, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []GroupData
	for rows.Next() {
		var groupName string
		rows.Scan(&groupName)

		// Count students in this group
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM students WHERE teacher_id = ? AND group_name = ?",
			teacherID, groupName).Scan(&count)
		if err != nil {
			return nil, err
		}

		groups = append(groups, GroupData{Name: groupName, StudentCount: count})
	}
	return groups, nil
}

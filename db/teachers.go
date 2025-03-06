package db

import (
	"database/sql"
)

// GetTeacherGroups gets a list of groups for the specified teacher
func GetTeacherGroups(db *sql.DB, teacherID int) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT group_name 
		FROM (
			SELECT group_name FROM lessons WHERE teacher_id = ? 
			UNION 
			SELECT group_name FROM students WHERE teacher_id = ?
		) ORDER BY group_name`,
		teacherID, teacherID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var group string
		rows.Scan(&group)
		groups = append(groups, group)
	}
	return groups, nil
}

// GetTeacherSubjects gets a list of subjects for the specified teacher
func GetTeacherSubjects(db *sql.DB, teacherID int) ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT subject FROM lessons WHERE teacher_id = ? ORDER BY subject", teacherID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	defer rows.Close()

	var subjects []string
	for rows.Next() {
		var subject string
		rows.Scan(&subject)
		subjects = append(subjects, subject)
	}
	return subjects, nil
}

// TeacherStats stores statistics about a teacher
type TeacherStats struct {
	ID       int
	FIO      string
	Lessons  int
	Hours    int
	Subjects map[string]int
}

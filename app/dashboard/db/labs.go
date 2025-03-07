package db

import (
	"database/sql"
)

// LabSettings stores settings for labs by subject and group
type LabSettings struct {
	ID        int
	TeacherID int
	GroupName string
	Subject   string
	TotalLabs int
}

// StudentLabGrade stores a student's grade for a specific lab
type StudentLabGrade struct {
	StudentID  int
	StudentFIO string
	LabNumber  int
	Grade      int
}

// StudentLabSummary summarizes a student's lab grades
type StudentLabSummary struct {
	StudentID  int
	StudentFIO string
	Grades     []int   // Grades for each lab, index = lab number - 1
	Average    float64 // Average grade
}

// GroupLabSummary summarizes a group's lab performance
type GroupLabSummary struct {
	GroupName    string
	Subject      string
	TotalLabs    int
	Students     []StudentLabSummary
	GroupAverage float64 // Average grade for the group
}

// GetLabSettings retrieves lab settings for a teacher, group and subject
func GetLabSettings(db *sql.DB, teacherID int, groupName, subject string) (LabSettings, error) {
	var settings LabSettings
	err := db.QueryRow(
		"SELECT id, teacher_id, group_name, subject, total_labs FROM lab_settings WHERE teacher_id = ? AND group_name = ? AND subject = ?",
		teacherID, groupName, subject).Scan(&settings.ID, &settings.TeacherID, &settings.GroupName, &settings.Subject, &settings.TotalLabs)

	if err == sql.ErrNoRows {
		// If no settings exist, return defaults
		settings = LabSettings{
			TeacherID: teacherID,
			GroupName: groupName,
			Subject:   subject,
			TotalLabs: 5, // Default to 5 labs
		}
		return settings, nil
	}

	return settings, err
}

// SaveLabSettings saves lab settings for a teacher, group and subject
func SaveLabSettings(db *sql.DB, settings LabSettings) error {
	// Check if settings already exist
	var exists int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM lab_settings WHERE teacher_id = ? AND group_name = ? AND subject = ?",
		settings.TeacherID, settings.GroupName, settings.Subject).Scan(&exists)

	if err != nil {
		return err
	}

	if exists > 0 {
		// Update existing settings
		_, err = db.Exec(
			"UPDATE lab_settings SET total_labs = ? WHERE teacher_id = ? AND group_name = ? AND subject = ?",
			settings.TotalLabs, settings.TeacherID, settings.GroupName, settings.Subject)
	} else {
		// Insert new settings
		_, err = db.Exec(
			"INSERT INTO lab_settings (teacher_id, group_name, subject, total_labs) VALUES (?, ?, ?, ?)",
			settings.TeacherID, settings.GroupName, settings.Subject, settings.TotalLabs)
	}

	return err
}

// SaveLabGrade saves a lab grade for a student
func SaveLabGrade(db *sql.DB, teacherID, studentID int, subject string, labNumber, grade int) error {
	// Check if grade already exists
	var exists int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM lab_grades WHERE student_id = ? AND subject = ? AND lab_number = ?",
		studentID, subject, labNumber).Scan(&exists)

	if err != nil {
		return err
	}

	if exists > 0 {
		// Update existing grade
		_, err = db.Exec(
			"UPDATE lab_grades SET grade = ? WHERE student_id = ? AND subject = ? AND lab_number = ?",
			grade, studentID, subject, labNumber)
	} else {
		// Insert new grade
		_, err = db.Exec(
			"INSERT INTO lab_grades (student_id, teacher_id, subject, lab_number, grade) VALUES (?, ?, ?, ?, ?)",
			studentID, teacherID, subject, labNumber, grade)
	}

	return err
}

// GetGroupLabSummary gets a summary of lab grades for a group in a subject
func GetGroupLabSummary(db *sql.DB, teacherID int, groupName, subject string) (GroupLabSummary, error) {
	// Get lab settings
	settings, err := GetLabSettings(db, teacherID, groupName, subject)
	if err != nil {
		return GroupLabSummary{}, err
	}

	// Get all students in the group
	students, err := GetStudentsInGroup(db, teacherID, groupName)
	if err != nil {
		return GroupLabSummary{}, err
	}

	summary := GroupLabSummary{
		GroupName: groupName,
		Subject:   subject,
		TotalLabs: settings.TotalLabs,
	}

	// Get grades for each student
	var totalGradeSum float64
	var totalGradeCount int

	for _, student := range students {
		// Get student's lab grades
		labGrades, err := db.Query(
			"SELECT lab_number, grade FROM lab_grades WHERE student_id = ? AND subject = ? ORDER BY lab_number",
			student.ID, subject)
		if err != nil {
			return GroupLabSummary{}, err
		}

		// Create a map of lab number to grade
		gradeMap := make(map[int]int)
		var gradeSum int
		var gradeCount int

		for labGrades.Next() {
			var labNumber, grade int
			if err := labGrades.Scan(&labNumber, &grade); err != nil {
				labGrades.Close()
				return GroupLabSummary{}, err
			}
			gradeMap[labNumber] = grade
			gradeSum += grade
			gradeCount++

			totalGradeSum += float64(grade)
			totalGradeCount++
		}
		labGrades.Close()

		// Create grades array filled with zeros initially
		grades := make([]int, settings.TotalLabs)
		for i := 0; i < settings.TotalLabs; i++ {
			labNumber := i + 1
			if grade, ok := gradeMap[labNumber]; ok {
				grades[i] = grade
			} else {
				grades[i] = 0 // No grade yet
			}
		}

		// Calculate average
		studentAverage := 0.0
		if gradeCount > 0 {
			studentAverage = float64(gradeSum) / float64(gradeCount)
		}

		studentSummary := StudentLabSummary{
			StudentID:  student.ID,
			StudentFIO: student.FIO,
			Grades:     grades,
			Average:    studentAverage,
		}

		summary.Students = append(summary.Students, studentSummary)
	}

	// Calculate group average
	if totalGradeCount > 0 {
		summary.GroupAverage = totalGradeSum / float64(totalGradeCount)
	}

	return summary, nil
}

package db

import (
	"TeacherJournal/app/dashboard/models"

	"gorm.io/gorm"
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
func GetLabSettings(db *gorm.DB, teacherID int, groupName, subject string) (LabSettings, error) {
	var settings LabSettings

	result := db.Model(&models.LabSettings{}).
		Select("id, teacher_id, group_name, subject, total_labs").
		Where("teacher_id = ? AND group_name = ? AND subject = ?", teacherID, groupName, subject).
		First(&settings)

	if result.Error != nil {
		// If no settings exist, return defaults
		if result.Error == gorm.ErrRecordNotFound {
			settings = LabSettings{
				TeacherID: teacherID,
				GroupName: groupName,
				Subject:   subject,
				TotalLabs: 5, // Default to 5 labs
			}
			return settings, nil
		}
		return settings, result.Error
	}

	return settings, nil
}

// SaveLabSettings saves lab settings for a teacher, group and subject
func SaveLabSettings(db *gorm.DB, settings LabSettings) error {
	// Check if settings already exist
	var count int64
	db.Model(&models.LabSettings{}).
		Where("teacher_id = ? AND group_name = ? AND subject = ?",
			settings.TeacherID, settings.GroupName, settings.Subject).
		Count(&count)

	if count > 0 {
		// Update existing settings
		return db.Model(&models.LabSettings{}).
			Where("teacher_id = ? AND group_name = ? AND subject = ?",
				settings.TeacherID, settings.GroupName, settings.Subject).
			Update("total_labs", settings.TotalLabs).Error
	} else {
		// Insert new settings
		newSettings := models.LabSettings{
			TeacherID: settings.TeacherID,
			GroupName: settings.GroupName,
			Subject:   settings.Subject,
			TotalLabs: settings.TotalLabs,
		}
		return db.Create(&newSettings).Error
	}
}

// SaveLabGrade saves a lab grade for a student
func SaveLabGrade(db *gorm.DB, teacherID, studentID int, subject string, labNumber, grade int) error {
	// Check if grade already exists
	var count int64
	db.Model(&models.LabGrade{}).
		Where("student_id = ? AND subject = ? AND lab_number = ?",
			studentID, subject, labNumber).
		Count(&count)

	if count > 0 {
		// Update existing grade
		return db.Model(&models.LabGrade{}).
			Where("student_id = ? AND subject = ? AND lab_number = ?",
				studentID, subject, labNumber).
			Update("grade", grade).Error
	} else {
		// Insert new grade
		newGrade := models.LabGrade{
			StudentID: studentID,
			TeacherID: teacherID,
			Subject:   subject,
			LabNumber: labNumber,
			Grade:     grade,
		}
		return db.Create(&newGrade).Error
	}
}

// GetGroupLabSummary gets a summary of lab grades for a group in a subject
func GetGroupLabSummary(db *gorm.DB, teacherID int, groupName, subject string) (GroupLabSummary, error) {
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
		var labGrades []struct {
			LabNumber int
			Grade     int
		}

		err := db.Model(&models.LabGrade{}).
			Select("lab_number, grade").
			Where("student_id = ? AND subject = ?", student.ID, subject).
			Order("lab_number").
			Find(&labGrades).Error

		if err != nil {
			return GroupLabSummary{}, err
		}

		// Create a map of lab number to grade
		gradeMap := make(map[int]int)
		var gradeSum int
		var gradeCount int

		for _, lg := range labGrades {
			gradeMap[lg.LabNumber] = lg.Grade
			gradeSum += lg.Grade
			gradeCount++

			totalGradeSum += float64(lg.Grade)
			totalGradeCount++
		}

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

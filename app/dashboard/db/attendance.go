package db

import (
	"TeacherJournal/app/dashboard/models"
	"fmt"
	"time"

	"github.com/tealeg/xlsx"
	"gorm.io/gorm"
)

// AttendanceData stores attendance information
type AttendanceData struct {
	LessonID         int
	Date             string
	Subject          string
	GroupName        string
	TotalStudents    int
	AttendedStudents int
}

// StudentAttendance represents a student's attendance record
type StudentAttendance struct {
	ID       int
	FIO      string
	Attended bool
}

// GetAttendanceForLesson retrieves attendance records for a specific lesson
func GetAttendanceForLesson(db *gorm.DB, lessonID int, teacherID int) ([]StudentAttendance, error) {
	// Verify the lesson belongs to this teacher
	var groupName string
	err := db.Model(&models.Lesson{}).
		Select("group_name").
		Where("id = ? AND teacher_id = ?", lessonID, teacherID).
		Pluck("group_name", &groupName).Error

	if err != nil {
		return nil, err
	}

	// Get all students in the group with their attendance status
	var studentsAttendance []StudentAttendance

	// Note the use of "attendances" table name instead of "attendance"
	err = db.Raw(`
		SELECT s.id, s.student_fio as fio, COALESCE(a.attended, 0) as attended
		FROM students s
		LEFT JOIN attendances a ON s.id = a.student_id AND a.lesson_id = ?
		WHERE s.teacher_id = ? AND s.group_name = ?
		ORDER BY s.student_fio
	`, lessonID, teacherID, groupName).Scan(&studentsAttendance).Error

	// Convert attendance integer to boolean
	for i := range studentsAttendance {
		studentsAttendance[i].Attended = studentsAttendance[i].Attended == true
	}

	return studentsAttendance, err
}

// GetTeacherAttendanceRecords retrieves all attendance records for a teacher
func GetTeacherAttendanceRecords(db *gorm.DB, teacherID int) ([]AttendanceData, error) {
	var attendances []AttendanceData

	// Note the use of "attendances" table name instead of "attendance"
	err := db.Raw(`
		SELECT l.id as lesson_id, l.date, l.subject, l.group_name, 
			(SELECT COUNT(*) FROM students s WHERE s.teacher_id = ? AND s.group_name = l.group_name) as total_students,
			(SELECT COUNT(*) FROM attendances a WHERE a.lesson_id = l.id AND a.attended = 1) as attended_students
		FROM lessons l
		WHERE l.teacher_id = ? AND EXISTS (SELECT 1 FROM attendances a WHERE a.lesson_id = l.id)
		ORDER BY l.date DESC
	`, teacherID, teacherID).Scan(&attendances).Error

	return attendances, err
}

// SaveAttendance saves attendance records for a lesson
func SaveAttendance(db *gorm.DB, lessonID int, teacherID int, attendedStudentIDs []int) error {
	// Verify the lesson belongs to this teacher
	var groupName string
	err := db.Model(&models.Lesson{}).
		Select("group_name").
		Where("id = ? AND teacher_id = ?", lessonID, teacherID).
		Pluck("group_name", &groupName).Error

	if err != nil {
		return err
	}

	// Start transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Delete existing attendance records - note the table name "attendances"
		if err := tx.Where("lesson_id = ?", lessonID).Delete(&models.Attendance{}).Error; err != nil {
			return err
		}

		// Create a map for faster lookup
		attendedMap := make(map[int]bool)
		for _, id := range attendedStudentIDs {
			attendedMap[id] = true
		}

		// Get all students in this group
		var students []models.Student
		if err := tx.Where("teacher_id = ? AND group_name = ?", teacherID, groupName).Find(&students).Error; err != nil {
			return err
		}

		// Insert new attendance records
		var attendanceRecords []models.Attendance

		for _, student := range students {
			attended := 0
			if attendedMap[student.ID] {
				attended = 1
			}

			attendanceRecords = append(attendanceRecords, models.Attendance{
				LessonID:  lessonID,
				StudentID: student.ID,
				Attended:  attended,
			})
		}

		// Bulk insert all attendance records
		if len(attendanceRecords) > 0 {
			return tx.Create(&attendanceRecords).Error
		}

		return nil
	})
}

// ExportAttendanceByGroup exports attendance data grouped by subject
func ExportAttendanceByGroup(db *gorm.DB, teacherID int, file *xlsx.File) error {
	// Get all subjects taught by this teacher with attendance
	var subjects []string
	err := db.Raw(`
		SELECT DISTINCT l.subject
		FROM lessons l
		JOIN attendance a ON l.id = a.lesson_id
		WHERE l.teacher_id = ?
		ORDER BY l.subject
	`, teacherID).Scan(&subjects).Error

	if err != nil {
		return err
	}

	// Process each subject
	for _, subject := range subjects {
		// Create a worksheet for this subject
		sheet, err := file.AddSheet(subject)
		if err != nil {
			return err
		}

		// Get all lessons for this subject with attendance
		type LessonInfo struct {
			ID      int
			Date    string
			DateFmt string
			Group   string
			Topic   string
		}

		var lessons []LessonInfo
		err = db.Raw(`
			SELECT l.id, l.date, l.group_name as 'group', l.topic
			FROM lessons l
			WHERE l.teacher_id = ? AND l.subject = ? AND EXISTS (
				SELECT 1 FROM attendance a WHERE a.lesson_id = l.id
			)
			ORDER BY l.date
		`, teacherID, subject).Scan(&lessons).Error

		if err != nil {
			return err
		}

		// Format dates
		for i := range lessons {
			date, err := time.Parse("2006-01-02", lessons[i].Date)
			if err == nil {
				lessons[i].DateFmt = date.Format("02.01.2006")
			} else {
				lessons[i].DateFmt = lessons[i].Date
			}
		}

		if len(lessons) == 0 {
			// No lessons with attendance for this subject
			headerRow := sheet.AddRow()
			headerRow.AddCell().SetString("No attendance data available for this subject")
			continue
		}

		// Group lessons by date
		lessonsByDate := make(map[string][]LessonInfo)
		var dates []string
		var formattedDates []string

		for _, lesson := range lessons {
			// Keep track of unique dates in order
			if _, exists := lessonsByDate[lesson.Date]; !exists {
				dates = append(dates, lesson.Date)
				formattedDates = append(formattedDates, lesson.DateFmt)
			}
			lessonsByDate[lesson.Date] = append(lessonsByDate[lesson.Date], lesson)
		}

		// Create header row
		headerRow := sheet.AddRow()
		headerRow.AddCell().SetString("Group")
		headerRow.AddCell().SetString("Student")

		for _, formattedDate := range formattedDates {
			headerCell := headerRow.AddCell()
			headerCell.SetString(formattedDate)
		}

		// Get all groups for this subject
		var groups []string
		err = db.Raw(`
			SELECT DISTINCT l.group_name
			FROM lessons l
			WHERE l.teacher_id = ? AND l.subject = ? AND EXISTS (
				SELECT 1 FROM attendance a WHERE a.lesson_id = l.id
			)
			ORDER BY l.group_name
		`, teacherID, subject).Scan(&groups).Error

		if err != nil {
			return err
		}

		// For each group
		for _, group := range groups {
			// Get all students in this group
			var students []struct {
				ID         int
				StudentFIO string
			}

			err = db.Raw(`
				SELECT id, student_fio
				FROM students
				WHERE teacher_id = ? AND group_name = ?
				ORDER BY student_fio
			`, teacherID, group).Scan(&students).Error

			if err != nil {
				return err
			}

			var firstStudent = true
			// For each student
			for _, student := range students {
				row := sheet.AddRow()

				// Only show group name for first student in group
				if firstStudent {
					row.AddCell().SetString(group)
					firstStudent = false
				} else {
					row.AddCell().SetString("")
				}

				row.AddCell().SetString(student.StudentFIO)

				// For each date
				for _, dateStr := range dates {
					// Find all lessons for this date and group
					var attended bool
					var lessonFound bool

					for _, lesson := range lessonsByDate[dateStr] {
						if lesson.Group == group {
							lessonFound = true

							// Check attendance
							var attendanceValue int
							err := db.Raw(`
								SELECT COALESCE(attended, 0)
								FROM attendance
								WHERE lesson_id = ? AND student_id = ?
							`, lesson.ID, student.ID).Scan(&attendanceValue).Error

							if err == nil && attendanceValue == 1 {
								attended = true
								break
							}
						}
					}

					attendanceCell := row.AddCell()
					if lessonFound {
						if attended {
							attendanceCell.SetString("✓") // Present
						} else {
							attendanceCell.SetString("✗") // Absent
						}
					} else {
						attendanceCell.SetString("-") // No lesson for this group on this date
					}
				}
			}
		}
	}

	return nil
}

// ExportAttendanceByLesson exports attendance data grouped by group
func ExportAttendanceByLesson(db *gorm.DB, teacherID int, file *xlsx.File) error {
	// Get all groups for this teacher with attendance data
	var groups []string
	err := db.Raw(`
		SELECT DISTINCT l.group_name
		FROM lessons l
		JOIN attendance a ON l.id = a.lesson_id
		WHERE l.teacher_id = ?
		ORDER BY l.group_name
	`, teacherID).Scan(&groups).Error

	if err != nil {
		return err
	}

	// Process each group
	for _, group := range groups {
		// Create a worksheet for this group
		sheet, err := file.AddSheet(group)
		if err != nil {
			return err
		}

		// Get all lessons for this group with attendance
		type LessonInfo struct {
			ID      int
			Subject string
			Topic   string
			Date    string
			DateFmt string
		}

		var lessons []LessonInfo
		err = db.Raw(`
			SELECT l.id, l.subject, l.topic, l.date
			FROM lessons l
			WHERE l.teacher_id = ? AND l.group_name = ? AND EXISTS (
				SELECT 1 FROM attendance a WHERE a.lesson_id = l.id
			)
			ORDER BY l.date
		`, teacherID, group).Scan(&lessons).Error

		if err != nil {
			return err
		}

		// Format dates
		for i := range lessons {
			date, err := time.Parse("2006-01-02", lessons[i].Date)
			if err == nil {
				lessons[i].DateFmt = date.Format("02.01.2006")
			} else {
				lessons[i].DateFmt = lessons[i].Date
			}
		}

		if len(lessons) == 0 {
			// No lessons with attendance for this group
			headerRow := sheet.AddRow()
			headerRow.AddCell().SetString("No attendance data available for this group")
			continue
		}

		// Create header row with student name and lesson details
		headerRow := sheet.AddRow()
		headerRow.AddCell().SetString("Student")

		for _, lesson := range lessons {
			headerCell := headerRow.AddCell()
			headerCell.SetString(fmt.Sprintf("%s: %s (%s)", lesson.Subject, lesson.Topic, lesson.DateFmt))
		}

		// Get all students in this group
		var students []struct {
			ID         int
			StudentFIO string
		}

		err = db.Raw(`
			SELECT id, student_fio
			FROM students
			WHERE teacher_id = ? AND group_name = ?
			ORDER BY student_fio
		`, teacherID, group).Scan(&students).Error

		if err != nil {
			return err
		}

		// For each student, create a row with attendance for each lesson
		for _, student := range students {
			row := sheet.AddRow()
			row.AddCell().SetString(student.StudentFIO)

			// Add attendance for each lesson
			for _, lesson := range lessons {
				var attended int
				err := db.Raw(`
					SELECT COALESCE(attended, 0)
					FROM attendance
					WHERE lesson_id = ? AND student_id = ?
				`, lesson.ID, student.ID).Scan(&attended).Error

				attendanceCell := row.AddCell()
				if err == nil && attended == 1 {
					attendanceCell.SetString("✓") // Present
				} else {
					attendanceCell.SetString("✗") // Absent
				}
			}
		}
	}

	return nil
}

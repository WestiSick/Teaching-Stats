package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/tealeg/xlsx"
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
func GetAttendanceForLesson(db *sql.DB, lessonID int, teacherID int) ([]StudentAttendance, error) {
	// Get lesson details to verify ownership
	var group string
	err := db.QueryRow(
		"SELECT group_name FROM lessons WHERE id = ? AND teacher_id = ?",
		lessonID, teacherID).Scan(&group)
	if err != nil {
		return nil, err
	}

	// Get all students in the group with their attendance status
	rows, err := db.Query(`
		SELECT s.id, s.student_fio, IFNULL(a.attended, 0) as attended
		FROM students s
		LEFT JOIN attendance a ON s.id = a.student_id AND a.lesson_id = ?
		WHERE s.teacher_id = ? AND s.group_name = ?
		ORDER BY s.student_fio`,
		lessonID, teacherID, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []StudentAttendance
	for rows.Next() {
		var s StudentAttendance
		var attended int
		err = rows.Scan(&s.ID, &s.FIO, &attended)
		if err != nil {
			return nil, err
		}
		s.Attended = attended == 1
		students = append(students, s)
	}
	return students, nil
}

// GetTeacherAttendanceRecords retrieves all attendance records for a teacher
func GetTeacherAttendanceRecords(db *sql.DB, teacherID int) ([]AttendanceData, error) {
	rows, err := db.Query(`
		SELECT l.id, l.date, l.subject, l.group_name, 
			(SELECT COUNT(*) FROM students s WHERE s.teacher_id = ? AND s.group_name = l.group_name) as total_students,
			(SELECT COUNT(*) FROM attendance a WHERE a.lesson_id = l.id AND a.attended = 1) as attended_students
		FROM lessons l
		WHERE l.teacher_id = ? AND EXISTS (SELECT 1 FROM attendance a WHERE a.lesson_id = l.id)
		ORDER BY l.date DESC`, teacherID, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendances []AttendanceData
	for rows.Next() {
		var a AttendanceData
		var dateStr string
		err := rows.Scan(&a.LessonID, &dateStr, &a.Subject, &a.GroupName, &a.TotalStudents, &a.AttendedStudents)
		if err != nil {
			return nil, err
		}
		a.Date = dateStr
		attendances = append(attendances, a)
	}
	return attendances, nil
}

// SaveAttendance saves attendance records for a lesson
func SaveAttendance(db *sql.DB, lessonID int, teacherID int, attendedStudentIDs []int) error {
	// Verify the lesson belongs to this teacher
	var groupName string
	err := db.QueryRow("SELECT group_name FROM lessons WHERE id = ? AND teacher_id = ?",
		lessonID, teacherID).Scan(&groupName)
	if err != nil {
		return err
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Delete existing attendance records
	_, err = tx.Exec("DELETE FROM attendance WHERE lesson_id = ?", lessonID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Create a map for faster lookup
	attendedMap := make(map[int]bool)
	for _, id := range attendedStudentIDs {
		attendedMap[id] = true
	}

	// Get all students in this group
	studentRows, err := db.Query(
		"SELECT id FROM students WHERE teacher_id = ? AND group_name = ?",
		teacherID, groupName)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer studentRows.Close()

	// Insert new attendance records
	for studentRows.Next() {
		var studentID int
		err = studentRows.Scan(&studentID)
		if err != nil {
			tx.Rollback()
			return err
		}

		attended := 0
		if attendedMap[studentID] {
			attended = 1
		}

		_, err = tx.Exec(
			"INSERT INTO attendance (lesson_id, student_id, attended) VALUES (?, ?, ?)",
			lessonID, studentID, attended)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// Commit transaction
	return tx.Commit()
}

// ExportAttendanceByGroup exports attendance data grouped by subject
func ExportAttendanceByGroup(db *sql.DB, teacherID int, file *xlsx.File) error {
	// Get all subjects taught by this teacher with attendance
	subjectQuery := `
		SELECT DISTINCT l.subject
		FROM lessons l
		JOIN attendance a ON l.id = a.lesson_id
		WHERE l.teacher_id = ?
		ORDER BY l.subject`

	subjectRows, err := db.Query(subjectQuery, teacherID)
	if err != nil {
		return err
	}
	defer subjectRows.Close()

	var subjects []string
	for subjectRows.Next() {
		var subject string
		if err := subjectRows.Scan(&subject); err != nil {
			return err
		}
		subjects = append(subjects, subject)
	}

	// Process each subject
	for _, subject := range subjects {
		// Create a worksheet for this subject
		sheet, err := file.AddSheet(subject)
		if err != nil {
			return err
		}

		// Get all lessons for this subject with attendance
		lessonQuery := `
			SELECT l.id, l.date, l.group_name, l.topic
			FROM lessons l
			WHERE l.teacher_id = ? AND l.subject = ? AND EXISTS (
				SELECT 1 FROM attendance a WHERE a.lesson_id = l.id
			)
			ORDER BY l.date`

		lessonRows, err := db.Query(lessonQuery, teacherID, subject)
		if err != nil {
			return err
		}

		type LessonInfo struct {
			ID      int
			Date    string
			DateFmt string
			Group   string
			Topic   string
		}
		var lessons []LessonInfo

		for lessonRows.Next() {
			var lesson LessonInfo
			err := lessonRows.Scan(&lesson.ID, &lesson.Date, &lesson.Group, &lesson.Topic)
			if err != nil {
				lessonRows.Close()
				return err
			}

			// Format the date
			date, err := time.Parse("2006-01-02", lesson.Date)
			if err == nil {
				lesson.DateFmt = date.Format("02.01.2006")
			} else {
				lesson.DateFmt = lesson.Date
			}

			lessons = append(lessons, lesson)
		}
		lessonRows.Close()

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
		groupQuery := `
			SELECT DISTINCT l.group_name
			FROM lessons l
			WHERE l.teacher_id = ? AND l.subject = ? AND EXISTS (
				SELECT 1 FROM attendance a WHERE a.lesson_id = l.id
			)
			ORDER BY l.group_name`

		groupRows, err := db.Query(groupQuery, teacherID, subject)
		if err != nil {
			return err
		}

		var groups []string
		for groupRows.Next() {
			var group string
			if err := groupRows.Scan(&group); err != nil {
				groupRows.Close()
				return err
			}
			groups = append(groups, group)
		}
		groupRows.Close()

		// For each group
		for _, group := range groups {
			// Get all students in this group
			studentQuery := `
				SELECT id, student_fio
				FROM students
				WHERE teacher_id = ? AND group_name = ?
				ORDER BY student_fio`

			studentRows, err := db.Query(studentQuery, teacherID, group)
			if err != nil {
				return err
			}

			var firstStudent = true
			// For each student
			for studentRows.Next() {
				var studentID int
				var studentFIO string
				if err := studentRows.Scan(&studentID, &studentFIO); err != nil {
					studentRows.Close()
					return err
				}

				row := sheet.AddRow()

				// Only show group name for first student in group
				if firstStudent {
					row.AddCell().SetString(group)
					firstStudent = false
				} else {
					row.AddCell().SetString("")
				}

				row.AddCell().SetString(studentFIO)

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
							attendanceQuery := `
								SELECT IFNULL(attended, 0)
								FROM attendance
								WHERE lesson_id = ? AND student_id = ?`

							err := db.QueryRow(attendanceQuery, lesson.ID, studentID).Scan(&attendanceValue)
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
			studentRows.Close()
		}
	}

	return nil
}

// ExportAttendanceByLesson exports attendance data grouped by group
func ExportAttendanceByLesson(db *sql.DB, teacherID int, file *xlsx.File) error {
	// Get all groups for this teacher with attendance data
	groupQuery := `
		SELECT DISTINCT l.group_name
		FROM lessons l
		JOIN attendance a ON l.id = a.lesson_id
		WHERE l.teacher_id = ?
		ORDER BY l.group_name`

	groupRows, err := db.Query(groupQuery, teacherID)
	if err != nil {
		return err
	}
	defer groupRows.Close()

	var groups []string
	for groupRows.Next() {
		var group string
		if err := groupRows.Scan(&group); err != nil {
			return err
		}
		groups = append(groups, group)
	}

	// Process each group
	for _, group := range groups {
		// Create a worksheet for this group
		sheet, err := file.AddSheet(group)
		if err != nil {
			return err
		}

		// Get all lessons for this group with attendance
		lessonQuery := `
			SELECT l.id, l.subject, l.topic, l.date
			FROM lessons l
			WHERE l.teacher_id = ? AND l.group_name = ? AND EXISTS (
				SELECT 1 FROM attendance a WHERE a.lesson_id = l.id
			)
			ORDER BY l.date`

		lessonRows, err := db.Query(lessonQuery, teacherID, group)
		if err != nil {
			return err
		}

		type LessonInfo struct {
			ID      int
			Subject string
			Topic   string
			Date    string
			DateFmt string
		}
		var lessons []LessonInfo

		for lessonRows.Next() {
			var lesson LessonInfo
			err := lessonRows.Scan(&lesson.ID, &lesson.Subject, &lesson.Topic, &lesson.Date)
			if err != nil {
				lessonRows.Close()
				return err
			}

			// Format the date
			date, err := time.Parse("2006-01-02", lesson.Date)
			if err == nil {
				lesson.DateFmt = date.Format("02.01.2006")
			} else {
				lesson.DateFmt = lesson.Date
			}

			lessons = append(lessons, lesson)
		}
		lessonRows.Close()

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
		studentQuery := `
			SELECT id, student_fio
			FROM students
			WHERE teacher_id = ? AND group_name = ?
			ORDER BY student_fio`

		studentRows, err := db.Query(studentQuery, teacherID, group)
		if err != nil {
			return err
		}

		// For each student, create a row with attendance for each lesson
		for studentRows.Next() {
			var studentID int
			var studentFIO string
			if err := studentRows.Scan(&studentID, &studentFIO); err != nil {
				studentRows.Close()
				return err
			}

			row := sheet.AddRow()
			row.AddCell().SetString(studentFIO)

			// Add attendance for each lesson
			for _, lesson := range lessons {
				var attended int
				attendanceQuery := `
					SELECT IFNULL(attended, 0)
					FROM attendance
					WHERE lesson_id = ? AND student_id = ?`

				err := db.QueryRow(attendanceQuery, lesson.ID, studentID).Scan(&attended)

				attendanceCell := row.AddCell()
				if err == nil && attended == 1 {
					attendanceCell.SetString("✓") // Present
				} else {
					attendanceCell.SetString("✗") // Absent
				}
			}
		}
		studentRows.Close()
	}

	return nil
}

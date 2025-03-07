package handlers

import (
	"TeacherJournal/utils"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tealeg/xlsx"

	"TeacherJournal/config"
	"TeacherJournal/db"
)

// AttendanceHandler handles attendance-related routes
type AttendanceHandler struct {
	DB   *sql.DB
	Tmpl *template.Template
}

// NewAttendanceHandler creates a new AttendanceHandler
func NewAttendanceHandler(database *sql.DB, tmpl *template.Template) *AttendanceHandler {
	return &AttendanceHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// EditAttendanceHandler handles editing attendance records
func (h *AttendanceHandler) EditAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	lessonID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid lesson ID", http.StatusBadRequest)
		return
	}

	// Verify the lesson belongs to this teacher
	var lesson db.LessonData
	err = h.DB.QueryRow(`
		SELECT l.id, l.group_name, l.subject, l.topic, l.date, l.type
		FROM lessons l
		WHERE l.id = ? AND l.teacher_id = ?`,
		lessonID, userInfo.ID).Scan(&lesson.ID, &lesson.Group, &lesson.Subject, &lesson.Topic, &lesson.Date, &lesson.Type)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Lesson not found or doesn't belong to you", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving lesson", http.StatusInternalServerError)
		}
		return
	}

	// Format the date for display
	lesson.Date = utils.FormatDate(lesson.Date)

	// Handle form submission
	if r.Method == "POST" {
		// Parse the form data
		if err := r.ParseForm(); err != nil {
			HandleError(w, err, "Error parsing form data", http.StatusBadRequest)
			return
		}

		// Get attended student IDs
		attendedStudents := r.Form["attended"]
		attendedIDs := make([]int, 0)
		for _, idStr := range attendedStudents {
			id, err := strconv.Atoi(idStr)
			if err == nil {
				attendedIDs = append(attendedIDs, id)
			}
		}

		// Save attendance using our DB function
		err = db.SaveAttendance(h.DB, lessonID, userInfo.ID, attendedIDs)
		if err != nil {
			HandleError(w, err, "Error updating attendance", http.StatusInternalServerError)
			return
		}

		db.LogAction(h.DB, userInfo.ID, "Edit Attendance",
			fmt.Sprintf("Updated attendance for lesson ID %d, group %s", lessonID, lesson.Group))

		http.Redirect(w, r, "/attendance", http.StatusSeeOther)
		return
	}

	// For GET requests, display the form
	// Get student attendance records using our DB function
	students, err := db.GetAttendanceForLesson(h.DB, lessonID, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
		return
	}

	data := struct {
		User     db.UserInfo
		Lesson   db.LessonData
		Students []db.StudentAttendance
	}{
		User:     userInfo,
		Lesson:   lesson,
		Students: students,
	}
	renderTemplate(w, h.Tmpl, "edit_attendance.html", data)
}

// AttendanceHandler handles viewing and managing attendance records
func (h *AttendanceHandler) AttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Handle attendance deletion
	if r.Method == "POST" {
		attendanceID := r.FormValue("attendance_id")
		if attendanceID == "" {
			http.Error(w, "Attendance ID not specified", http.StatusBadRequest)
			return
		}

		// Delete attendance records
		_, err := db.ExecuteQuery(h.DB,
			"DELETE FROM attendance WHERE lesson_id = ? AND EXISTS (SELECT 1 FROM lessons WHERE id = ? AND teacher_id = ?)",
			attendanceID, attendanceID, userInfo.ID)
		if err != nil {
			HandleError(w, err, "Error deleting attendance", http.StatusInternalServerError)
			return
		}

		db.LogAction(h.DB, userInfo.ID, "Delete Attendance",
			fmt.Sprintf("Deleted attendance for lesson ID %s", attendanceID))

		http.Redirect(w, r, "/attendance", http.StatusSeeOther)
		return
	}

	// Get attendance records
	attendances, err := db.GetTeacherAttendanceRecords(h.DB, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving attendance list", http.StatusInternalServerError)
		return
	}

	// Format dates
	for i := range attendances {
		attendances[i].Date = utils.FormatDate(attendances[i].Date)
	}

	data := struct {
		User        db.UserInfo
		Attendances []db.AttendanceData
	}{
		User:        userInfo,
		Attendances: attendances,
	}
	renderTemplate(w, h.Tmpl, "attendance.html", data)
}

// AddAttendanceHandler handles creating/recording attendance
func (h *AttendanceHandler) AddAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if r.Method == "GET" {
		// Get subjects for selection
		subjects, err := db.GetTeacherSubjects(h.DB, userInfo.ID)
		if err != nil {
			HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
			return
		}

		data := struct {
			User     db.UserInfo
			Subjects []string
		}{
			User:     userInfo,
			Subjects: subjects,
		}
		renderTemplate(w, h.Tmpl, "add_attendance.html", data)
		return
	}

	// Process form submission
	lessonID, _ := strconv.Atoi(r.FormValue("lesson_id"))
	if lessonID == 0 {
		http.Error(w, "Lesson not selected", http.StatusBadRequest)
		return
	}

	// Parse the form data
	if err := r.ParseForm(); err != nil {
		HandleError(w, err, "Error parsing form data", http.StatusBadRequest)
		return
	}

	// Get attended student IDs
	attendedStudents := r.Form["attended"]
	attendedIDs := make([]int, 0)
	for _, idStr := range attendedStudents {
		id, err := strconv.Atoi(idStr)
		if err == nil {
			attendedIDs = append(attendedIDs, id)
		}
	}

	// Save attendance using our DB function
	err = db.SaveAttendance(h.DB, lessonID, userInfo.ID, attendedIDs)
	if err != nil {
		HandleError(w, err, "Error saving attendance", http.StatusInternalServerError)
		return
	}

	// Get group name for logging
	var groupName string
	h.DB.QueryRow("SELECT group_name FROM lessons WHERE id = ?", lessonID).Scan(&groupName)

	db.LogAction(h.DB, userInfo.ID, "Create Attendance",
		fmt.Sprintf("Added attendance for lesson ID %d, group %s", lessonID, groupName))

	http.Redirect(w, r, "/attendance", http.StatusSeeOther)
}

// ExportAttendanceExcelHandler handles exporting attendance data to Excel
func (h *AttendanceHandler) ExportAttendanceExcelHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	exportMode := r.URL.Query().Get("mode")
	if exportMode != "group" && exportMode != "lesson" {
		http.Error(w, "Invalid export mode. Use 'group' or 'lesson'", http.StatusBadRequest)
		return
	}

	// Create Excel file
	file := xlsx.NewFile()

	var err2 error
	if exportMode == "group" {
		err2 = db.ExportAttendanceByGroup(h.DB, userInfo.ID, file)
	} else {
		err2 = db.ExportAttendanceByLesson(h.DB, userInfo.ID, file)
	}

	if err2 != nil {
		HandleError(w, err2, fmt.Sprintf("Error exporting attendance by %s", exportMode), http.StatusInternalServerError)
		return
	}

	db.LogAction(h.DB, userInfo.ID, "Export Attendance",
		fmt.Sprintf("Exported attendance data in %s mode", exportMode))

	// Send file to user
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=attendance.xlsx")
	file.Write(w)
}

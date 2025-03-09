package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/app/dashboard/utils"
	"TeacherJournal/config"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tealeg/xlsx"
	"gorm.io/gorm"
)

// LessonHandler handles lesson-related routes
type LessonHandler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

// NewLessonHandler creates a new LessonHandler
func NewLessonHandler(database *gorm.DB, tmpl *template.Template) *LessonHandler {
	return &LessonHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// AddLessonHandler handles adding a new lesson
func (h *LessonHandler) AddLessonHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if r.Method == "GET" {
		// Get groups for selection
		groups, err := db.GetTeacherGroups(h.DB, userInfo.ID)
		if err != nil {
			HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		// Get subjects for selection
		subjects, err := db.GetTeacherSubjects(h.DB, userInfo.ID)
		if err != nil {
			HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
			return
		}

		data := struct {
			User     db.UserInfo
			Groups   []string
			Subjects []string
		}{
			User:     userInfo,
			Groups:   groups,
			Subjects: subjects,
		}
		renderTemplate(w, h.Tmpl, "add_lesson.html", data)
		return
	}

	// Process form submission
	group := r.FormValue("group")
	subject := r.FormValue("subject")
	topic := r.FormValue("topic")
	hours, _ := strconv.Atoi(r.FormValue("hours"))
	date := r.FormValue("date")
	lessonType := r.FormValue("type")

	// Validate inputs
	if group == "" || subject == "" || topic == "" || hours <= 0 || date == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	// Normalize lesson type
	if lessonType != "Лекция" && lessonType != "Лабораторная работа" && lessonType != "Практика" {
		lessonType = "Лекция"
	}

	// Insert lesson
	lesson := models.Lesson{
		TeacherID: userInfo.ID,
		GroupName: group,
		Subject:   subject,
		Topic:     topic,
		Hours:     hours,
		Date:      date,
		Type:      lessonType,
	}

	if err := h.DB.Create(&lesson).Error; err != nil {
		HandleError(w, err, "Error adding lesson", http.StatusInternalServerError)
		return
	}

	db.LogAction(h.DB, userInfo.ID, "Add Lesson",
		fmt.Sprintf("Added %s: %s, %s, %s, %d hours, %s", lessonType, subject, group, topic, hours, date))

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// SubjectLessonsHandler handles viewing and managing lessons by subject
func (h *LessonHandler) SubjectLessonsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	subject := r.URL.Query().Get("subject")
	if subject == "" {
		http.Error(w, "Subject not specified", http.StatusBadRequest)
		return
	}

	// Handle lesson deletion
	if r.Method == "POST" {
		lessonID := r.FormValue("lesson_id")
		if lessonID == "" {
			http.Error(w, "Lesson ID not specified", http.StatusBadRequest)
			return
		}

		// Get lesson details for logging
		var lesson db.LessonData
		lessonIDInt, _ := strconv.Atoi(lessonID)

		lesson, err = db.GetLessonByID(h.DB, lessonIDInt, userInfo.ID)
		if err != nil {
			http.Error(w, "Lesson not found", http.StatusNotFound)
			return
		}

		// Delete lesson
		if err := h.DB.Where("id = ? AND teacher_id = ?", lessonIDInt, userInfo.ID).Delete(&models.Lesson{}).Error; err != nil {
			HandleError(w, err, "Error deleting lesson", http.StatusInternalServerError)
			return
		}

		formattedDate := utils.FormatDate(lesson.Date)
		db.LogAction(h.DB, userInfo.ID, "Delete Lesson",
			fmt.Sprintf("Deleted lesson ID %s: %s, %s, %s, %d hours, %s, type: %s",
				lessonID, subject, lesson.Group, lesson.Topic, lesson.Hours, formattedDate, lesson.Type))

		http.Redirect(w, r, fmt.Sprintf("/lessons/subject?subject=%s", subject), http.StatusSeeOther)
		return
	}

	// Get lessons for this subject using our DB function
	lessons, err := db.GetLessonsBySubject(h.DB, userInfo.ID, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lessons", http.StatusInternalServerError)
		return
	}

	// Process lessons - format dates
	for i := range lessons {
		lessons[i].Date = utils.FormatDate(lessons[i].Date)
	}

	data := struct {
		User    db.UserInfo
		Subject string
		Lessons []db.LessonData
	}{
		User:    userInfo,
		Subject: subject,
		Lessons: lessons,
	}
	renderTemplate(w, h.Tmpl, "subject_lessons.html", data)
}

// EditLessonHandler handles editing a lesson
func (h *LessonHandler) EditLessonHandler(w http.ResponseWriter, r *http.Request) {
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

	if r.Method == "GET" {
		// Get lesson details
		lesson, err := db.GetLessonByID(h.DB, lessonID, userInfo.ID)
		if err != nil {
			http.Error(w, "Lesson not found", http.StatusNotFound)
			return
		}

		// Get groups for selection
		groups, err := db.GetTeacherGroups(h.DB, userInfo.ID)
		if err != nil {
			HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		// Get subjects for selection
		subjects, err := db.GetTeacherSubjects(h.DB, userInfo.ID)
		if err != nil {
			HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
			return
		}

		data := struct {
			User     db.UserInfo
			Lesson   db.LessonData
			Groups   []string
			Subjects []string
		}{
			User:     userInfo,
			Lesson:   lesson,
			Groups:   groups,
			Subjects: subjects,
		}
		renderTemplate(w, h.Tmpl, "edit_lesson.html", data)
		return
	}

	// Process form submission
	group := r.FormValue("group")
	subject := r.FormValue("subject")
	topic := r.FormValue("topic")
	hours, _ := strconv.Atoi(r.FormValue("hours"))
	date := r.FormValue("date")
	lessonType := r.FormValue("type")

	// Validate inputs
	if group == "" || subject == "" || topic == "" || hours <= 0 || date == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	// Normalize lesson type
	if lessonType != "Лекция" && lessonType != "Лабораторная работа" && lessonType != "Практика" {
		lessonType = "Лекция"
	}

	// Update lesson
	if err := h.DB.Model(&models.Lesson{}).
		Where("id = ? AND teacher_id = ?", lessonID, userInfo.ID).
		Updates(map[string]interface{}{
			"group_name": group,
			"subject":    subject,
			"topic":      topic,
			"hours":      hours,
			"date":       date,
			"type":       lessonType,
		}).Error; err != nil {
		HandleError(w, err, "Error updating lesson", http.StatusInternalServerError)
		return
	}

	db.LogAction(h.DB, userInfo.ID, "Edit Lesson",
		fmt.Sprintf("Edited lesson ID %d: %s, %s, %s, %d hours, %s, type: %s",
			lessonID, subject, group, topic, hours, date, lessonType))

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// ExportExcelHandler handles the export functionality
func (h *LessonHandler) ExportExcelHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	subject := r.URL.Query().Get("subject")
	group := r.URL.Query().Get("group")

	// Build query with filters
	lessonQuery := h.DB.Model(&models.Lesson{}).
		Select("date, group_name, subject, topic, hours, type").
		Where("teacher_id = ? AND subject = ?", userInfo.ID, subject)

	if group != "" {
		lessonQuery = lessonQuery.Where("group_name = ?", group)
	}

	lessonQuery = lessonQuery.Order("date")

	// Execute query
	type LessonExport struct {
		Date      string
		GroupName string
		Subject   string
		Topic     string
		Hours     int
		Type      string
	}

	var lessons []LessonExport
	if err := lessonQuery.Find(&lessons).Error; err != nil {
		HandleError(w, err, "Error retrieving data", http.StatusInternalServerError)
		return
	}

	// Create Excel file
	file := xlsx.NewFile()
	sheet, _ := file.AddSheet("Statistics")
	header := sheet.AddRow()
	header.WriteSlice(&[]string{"Date", "Group", "Subject", "Topic", "Hours", "Type"}, -1)

	// Populate data
	for _, lesson := range lessons {
		formattedDate := utils.FormatDate(lesson.Date)

		row := sheet.AddRow()
		row.WriteSlice(&[]interface{}{formattedDate, lesson.GroupName, lesson.Subject, lesson.Topic, lesson.Hours, lesson.Type}, -1)
	}

	// Send file to user
	w.Header().Set("Content-Disposition", "attachment; filename=stats.xlsx")
	file.Write(w)
}

package handlers

import (
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/app/dashboard/utils"
	"net/http"
	"strconv"

	"gorm.io/gorm"
)

// APIHandler handles API-related routes
type APIHandler struct {
	DB *gorm.DB
}

// NewAPIHandler creates a new APIHandler
func NewAPIHandler(database *gorm.DB) *APIHandler {
	return &APIHandler{
		DB: database,
	}
}

// APILessonsHandler handles the API endpoint for getting lessons by subject
func (h *APIHandler) APILessonsHandler(w http.ResponseWriter, r *http.Request) {
	teacherID, err := strconv.Atoi(r.Header.Get("X-Teacher-ID"))
	if err != nil {
		http.Error(w, "Unauthorized access", http.StatusUnauthorized)
		return
	}

	subject := r.URL.Query().Get("subject")
	if subject == "" {
		http.Error(w, "Subject not specified", http.StatusBadRequest)
		return
	}

	// Get lessons for this subject
	type LessonAPIData struct {
		ID        int    `json:"id"`
		Date      string `json:"date"`
		GroupName string `json:"group_name"`
	}

	var lessons []LessonAPIData

	err = h.DB.Model(&models.Lesson{}).
		Select("id, date, group_name").
		Where("teacher_id = ? AND subject = ?", teacherID, subject).
		Order("date").
		Find(&lessons).Error

	if err != nil {
		HandleError(w, err, "Error retrieving lessons", http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, lessons)
}

// APIStudentsHandler handles the API endpoint for getting students by lesson
func (h *APIHandler) APIStudentsHandler(w http.ResponseWriter, r *http.Request) {
	teacherID, err := strconv.Atoi(r.Header.Get("X-Teacher-ID"))
	if err != nil {
		http.Error(w, "Unauthorized access", http.StatusUnauthorized)
		return
	}

	lessonID, err := strconv.Atoi(r.URL.Query().Get("lesson_id"))
	if err != nil || lessonID == 0 {
		http.Error(w, "Lesson not specified", http.StatusBadRequest)
		return
	}

	// Get group for this lesson
	var groupName string
	err = h.DB.Model(&models.Lesson{}).
		Select("group_name").
		Where("id = ? AND teacher_id = ?", lessonID, teacherID).
		Pluck("group_name", &groupName).Error

	if err != nil {
		http.Error(w, "Lesson not found", http.StatusBadRequest)
		return
	}

	// Get students in this group
	type StudentAPIData struct {
		ID  int    `json:"id"`
		FIO string `json:"fio"`
	}

	var students []StudentAPIData

	err = h.DB.Model(&models.Student{}).
		Select("id, student_fio as fio").
		Where("teacher_id = ? AND group_name = ?", teacherID, groupName).
		Find(&students).Error

	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, students)
}

package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/utils"
	"database/sql"
	"net/http"
	"strconv"
)

// APIHandler handles API-related routes
type APIHandler struct {
	DB *sql.DB
}

// NewAPIHandler creates a new APIHandler
func NewAPIHandler(database *sql.DB) *APIHandler {
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
	rows, err := h.DB.Query(
		"SELECT id, date, group_name FROM lessons WHERE teacher_id = ? AND subject = ? ORDER BY date",
		teacherID, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lessons", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Process lessons
	type LessonAPIData struct {
		ID        int    `json:"id"`
		Date      string `json:"date"`
		GroupName string `json:"group_name"`
	}
	var lessons []LessonAPIData
	for rows.Next() {
		var l LessonAPIData
		rows.Scan(&l.ID, &l.Date, &l.GroupName)
		lessons = append(lessons, l)
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
	err = h.DB.QueryRow("SELECT group_name FROM lessons WHERE id = ? AND teacher_id = ?",
		lessonID, teacherID).Scan(&groupName)
	if err != nil {
		http.Error(w, "Lesson not found", http.StatusBadRequest)
		return
	}

	// Get students in this group
	students, err := db.GetStudentsInGroup(h.DB, teacherID, groupName)
	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
		return
	}

	type StudentAPIData struct {
		ID  int    `json:"id"`
		FIO string `json:"fio"`
	}
	var apiStudents []StudentAPIData
	for _, s := range students {
		apiStudents = append(apiStudents, StudentAPIData{ID: s.ID, FIO: s.FIO})
	}

	utils.RespondJSON(w, apiStudents)
}

package handlers

import (
	"database/sql"
	"html/template"
	"net/http"

	"TeacherJournal/config"
	"TeacherJournal/db"
)

// DashboardHandler handles dashboard-related routes
type DashboardHandler struct {
	DB   *sql.DB
	Tmpl *template.Template
}

// NewDashboardHandler creates a new DashboardHandler
func NewDashboardHandler(database *sql.DB, tmpl *template.Template) *DashboardHandler {
	return &DashboardHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// DashboardHandler handles the dashboard page
func (h *DashboardHandler) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get lesson statistics
	var totalLessons, totalHours sql.NullInt64
	err = h.DB.QueryRow(
		"SELECT COUNT(*) as total_lessons, SUM(hours) as total_hours FROM lessons WHERE teacher_id = ?",
		userInfo.ID).Scan(&totalLessons, &totalHours)

	lessonsExist := true
	if err != nil {
		if err == sql.ErrNoRows {
			totalLessons = sql.NullInt64{Int64: 0, Valid: true}
			totalHours = sql.NullInt64{Int64: 0, Valid: true}
			lessonsExist = false
		} else {
			HandleError(w, err, "Error retrieving statistics", http.StatusInternalServerError)
			return
		}
	}

	// Get subject statistics
	subjects := make(map[string]int)
	rows, err := h.DB.Query("SELECT subject, COUNT(*) as count FROM lessons WHERE teacher_id = ? GROUP BY subject", userInfo.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var subject string
			var count int
			rows.Scan(&subject, &count)
			subjects[subject] = count
		}
	} else if err != sql.ErrNoRows {
		HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
		return
	}

	// Get groups
	groups, err := db.GetTeacherGroups(h.DB, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	// Render dashboard template
	data := struct {
		User         db.UserInfo
		TotalLessons int
		TotalHours   int
		Subjects     map[string]int
		Groups       []string
		HasLessons   bool
	}{
		User:         userInfo,
		TotalLessons: int(totalLessons.Int64),
		TotalHours:   int(totalHours.Int64),
		Subjects:     subjects,
		Groups:       groups,
		HasLessons:   lessonsExist,
	}
	renderTemplate(w, h.Tmpl, "dashboard.html", data)
}

package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/config"
	"html/template"
	"net/http"

	"gorm.io/gorm"
)

// DashboardHandler handles dashboard-related routes
type DashboardHandler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

// NewDashboardHandler creates a new DashboardHandler
func NewDashboardHandler(database *gorm.DB, tmpl *template.Template) *DashboardHandler {
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
	var totalLessons int64
	var totalHours int64

	// Check if any lessons exist
	lessonsExist := true

	// Count total lessons
	h.DB.Model(&models.Lesson{}).
		Where("teacher_id = ?", userInfo.ID).
		Count(&totalLessons)

	// Sum total hours
	h.DB.Model(&models.Lesson{}).
		Where("teacher_id = ?", userInfo.ID).
		Select("COALESCE(SUM(hours), 0) as total_hours").
		Pluck("total_hours", &totalHours)

	// If no lessons found
	if totalLessons == 0 {
		lessonsExist = false
	}

	// Get subject statistics
	subjects := make(map[string]int)

	// Get subject counts
	var subjectCounts []struct {
		Subject string
		Count   int
	}

	h.DB.Model(&models.Lesson{}).
		Select("subject, COUNT(*) as count").
		Where("teacher_id = ?", userInfo.ID).
		Group("subject").
		Find(&subjectCounts)

	// Convert to map
	for _, sc := range subjectCounts {
		subjects[sc.Subject] = sc.Count
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
		TotalLessons: int(totalLessons),
		TotalHours:   int(totalHours),
		Subjects:     subjects,
		Groups:       groups,
		HasLessons:   lessonsExist,
	}
	renderTemplate(w, h.Tmpl, "dashboard.html", data)
}

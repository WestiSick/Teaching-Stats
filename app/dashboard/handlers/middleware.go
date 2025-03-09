package handlers

import (
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/config"
	"net/http"

	"gorm.io/gorm"
)

// AuthMiddleware verifies user authentication
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := config.Store.Get(r, config.SessionName)
		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// AdminMiddleware verifies admin role
func AdminMiddleware(database *gorm.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := config.Store.Get(r, config.SessionName)
		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var user models.User
		err := database.Model(&models.User{}).
			Select("role").
			Where("id = ?", userID).
			First(&user).Error

		if err != nil || user.Role != "admin" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// SubscriberMiddleware verifies the user has a paid subscription (not "free" role)
func SubscriberMiddleware(database *gorm.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := config.Store.Get(r, config.SessionName)
		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var user models.User
		err := database.Model(&models.User{}).
			Select("role").
			Where("id = ?", userID).
			First(&user).Error

		if err != nil || user.Role == "free" {
			http.Redirect(w, r, "/subscription", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	}
}

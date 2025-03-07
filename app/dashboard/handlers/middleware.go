package handlers

import (
	"TeacherJournal/config"
	"database/sql"
	"net/http"
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
func AdminMiddleware(database *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := config.Store.Get(r, config.SessionName)
		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var role string
		err := database.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
		if err != nil || role != "admin" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// SubscriberMiddleware verifies the user has a paid subscription (not "free" role)
func SubscriberMiddleware(database *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := config.Store.Get(r, config.SessionName)
		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var role string
		err := database.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
		if err != nil || role == "free" {
			http.Redirect(w, r, "/subscription", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	}
}

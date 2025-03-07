package handlers

import (
	"database/sql"
	"net/http"

	"TeacherJournal/config"
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

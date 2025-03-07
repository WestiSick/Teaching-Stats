package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"

	"TeacherJournal/app/TeacherTickets/config"
	"TeacherJournal/app/TeacherTickets/models"
	"TeacherJournal/app/dashboard/db"
)

// HandleError handles HTTP errors with logging
func HandleError(w http.ResponseWriter, err error, message string, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, statusCode)
}

// ConvertUserInfo конвертирует db.UserInfo в models.UserInfo
func ConvertUserInfo(dbUser db.UserInfo) models.UserInfo {
	return models.UserInfo{
		ID:  dbUser.ID,
		FIO: dbUser.FIO,
	}
}

// renderTemplate renders a template with standard data structure
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) {
	err := tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		HandleError(w, err, "Template rendering error", http.StatusInternalServerError)
	}
}

// CreateTemplateHelperFunctions creates a map of functions to pass to templates
func CreateTemplateHelperFunctions() template.FuncMap {
	return template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
		"inc": func(i int) int {
			return i + 1
		},
		"divideAndMultiply": func(a, b int, multiplier float64) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b) * multiplier
		},
		"ge": func(a, b float64) bool {
			return a >= b
		},
		"iter": func(count int) []int {
			var result []int
			for i := 0; i < count; i++ {
				result = append(result, i)
			}
			return result
		},
		"deref": func(i *int) int {
			if i == nil {
				return 0
			}
			return *i
		},
	}
}

// AuthMiddleware verifies user authentication
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := config.Store.Get(r, config.SessionName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

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
		session, err := config.Store.Get(r, config.SessionName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var role string
		err = database.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
		if err != nil || role != "admin" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
}

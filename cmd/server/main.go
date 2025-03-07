package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"TeacherJournal/db"
	"TeacherJournal/handlers"
)

// Renders a template with standard data structure
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) {
	err := tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		handlers.HandleError(w, err, "Template rendering error", http.StatusInternalServerError)
	}
}

// Helper functions for templates
func createTemplateHelperFunctions() template.FuncMap {
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
	}
}

func main() {
	database := db.InitDB()
	defer database.Close()

	// Create a new template with the function map
	tmpl := template.New("")

	// Register the functions
	tmpl = tmpl.Funcs(createTemplateHelperFunctions())

	// Parse the templates
	tmpl, err := tmpl.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(database, tmpl)
	dashboardHandler := handlers.NewDashboardHandler(database, tmpl)
	lessonHandler := handlers.NewLessonHandler(database, tmpl)
	groupHandler := handlers.NewGroupHandler(database, tmpl)
	attendanceHandler := handlers.NewAttendanceHandler(database, tmpl)
	adminHandler := handlers.NewAdminHandler(database, tmpl)
	apiHandler := handlers.NewAPIHandler(database)
	labHandler := handlers.NewLabHandler(database, tmpl)

	// Basic routes
	router.HandleFunc("/", authHandler.IndexHandler).Methods("GET")
	router.HandleFunc("/register", authHandler.RegisterHandler).Methods("GET", "POST")
	router.HandleFunc("/login", authHandler.LoginHandler).Methods("GET", "POST")
	router.HandleFunc("/logout", authHandler.LogoutHandler).Methods("GET")
	router.HandleFunc("/dashboard", handlers.AuthMiddleware(dashboardHandler.DashboardHandler)).Methods("GET")

	// Lesson management
	router.HandleFunc("/lesson/add", handlers.AuthMiddleware(lessonHandler.AddLessonHandler)).Methods("GET", "POST")
	router.HandleFunc("/lessons/subject", handlers.AuthMiddleware(lessonHandler.SubjectLessonsHandler)).Methods("GET", "POST")
	router.HandleFunc("/lesson/edit/{id}", handlers.AuthMiddleware(lessonHandler.EditLessonHandler)).Methods("GET", "POST")
	router.HandleFunc("/export", handlers.AuthMiddleware(lessonHandler.ExportExcelHandler)).Methods("GET")

	// Group management
	router.HandleFunc("/groups", handlers.AuthMiddleware(groupHandler.GroupsHandler)).Methods("GET", "POST")
	router.HandleFunc("/groups/edit/{groupName}", handlers.AuthMiddleware(groupHandler.EditGroupHandler)).Methods("GET", "POST")
	router.HandleFunc("/groups/add", handlers.AuthMiddleware(groupHandler.AddGroupHandler)).Methods("GET", "POST")

	// Attendance management
	router.HandleFunc("/attendance/add", handlers.AuthMiddleware(attendanceHandler.AddAttendanceHandler)).Methods("GET", "POST")
	router.HandleFunc("/attendance", handlers.AuthMiddleware(attendanceHandler.AttendanceHandler)).Methods("GET", "POST")
	router.HandleFunc("/attendance/edit/{id}", handlers.AuthMiddleware(attendanceHandler.EditAttendanceHandler)).Methods("GET", "POST")
	router.HandleFunc("/export/attendance", handlers.AuthMiddleware(attendanceHandler.ExportAttendanceExcelHandler)).Methods("GET")

	// Admin routes
	router.HandleFunc("/admin", handlers.AdminMiddleware(database, adminHandler.AdminDashboardHandler)).Methods("GET")
	router.HandleFunc("/admin/", handlers.AdminMiddleware(database, adminHandler.AdminDashboardHandler)).Methods("GET")
	router.HandleFunc("/admin/users", handlers.AdminMiddleware(database, adminHandler.AdminUsersHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/logs", handlers.AdminMiddleware(database, adminHandler.AdminLogsHandler)).Methods("GET")
	router.HandleFunc("/admin/groups", handlers.AdminMiddleware(database, adminHandler.AdminTeacherGroupsHandler)).Methods("GET")
	router.HandleFunc("/admin/groups/add/{teacherID}", handlers.AdminMiddleware(database, adminHandler.AdminAddGroupHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/groups/edit/{teacherID}/{groupName}", handlers.AdminMiddleware(database, adminHandler.AdminEditGroupHandler)).Methods("GET", "POST")

	// Admin attendance management routes
	router.HandleFunc("/admin/attendance", handlers.AdminMiddleware(database, adminHandler.AdminAttendanceHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/attendance/view/{id}", handlers.AdminMiddleware(database, adminHandler.AdminViewAttendanceHandler)).Methods("GET")
	router.HandleFunc("/admin/attendance/edit/{id}", handlers.AdminMiddleware(database, adminHandler.AdminEditAttendanceHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/attendance/export", handlers.AdminMiddleware(database, adminHandler.AdminExportAttendanceHandler)).Methods("GET")
	router.HandleFunc("/admin/attendance/{teacherID}/{groupName}", handlers.AdminMiddleware(database, adminHandler.AdminAttendanceHandler)).Methods("GET")

	// API endpoints
	router.HandleFunc("/api/lessons", apiHandler.APILessonsHandler).Methods("GET")
	router.HandleFunc("/api/students", apiHandler.APIStudentsHandler).Methods("GET")

	// Lab grades management
	router.HandleFunc("/labs", handlers.AuthMiddleware(labHandler.GroupLabsHandler)).Methods("GET")
	router.HandleFunc("/labs/grades/{subject}/{group}", handlers.AuthMiddleware(labHandler.LabGradesHandler)).Methods("GET", "POST")
	router.HandleFunc("/labs/export/{subject}/{group}", handlers.AuthMiddleware(labHandler.ExportLabGradesHandler)).Methods("GET")

	// Admin labs management routes
	router.HandleFunc("/admin/labs", handlers.AdminMiddleware(database, adminHandler.AdminLabsHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/labs/view/{teacherID}/{subject}/{group}", handlers.AdminMiddleware(database, adminHandler.AdminViewLabGradesHandler)).Methods("GET")
	router.HandleFunc("/admin/labs/edit/{teacherID}/{subject}/{group}", handlers.AdminMiddleware(database, adminHandler.AdminEditLabGradesHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/labs/export/{teacherID}/{subject}/{group}", handlers.AdminMiddleware(database, adminHandler.AdminExportLabGradesHandler)).Methods("GET")

	log.Println("Server started on :8080")
	http.ListenAndServe(":8080", router)
}

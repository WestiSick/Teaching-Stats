package main

import (
	"TeacherJournal/app/dashboard/db"
	handlers2 "TeacherJournal/app/dashboard/handlers"
	ticketMiddleware "TeacherJournal/app/tickets/middleware"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// Renders a template with standard data structure
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) {
	err := tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		handlers2.HandleError(w, err, "Template rendering error", http.StatusInternalServerError)
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
	// Initialize the database
	database := db.InitDB()

	// Get the underlying sql.DB for defer Close
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatal("Failed to get SQL DB for closing:", err)
	}
	defer sqlDB.Close()

	// Create a new template with the function map
	tmpl := template.New("")

	// Register the functions
	tmpl = tmpl.Funcs(createTemplateHelperFunctions())

	// Parse the templates
	tmpl, err = tmpl.ParseGlob("app/dashboard/templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	// Initialize handlers
	authHandler := handlers2.NewAuthHandler(database, tmpl)
	dashboardHandler := handlers2.NewDashboardHandler(database, tmpl)
	lessonHandler := handlers2.NewLessonHandler(database, tmpl)
	groupHandler := handlers2.NewGroupHandler(database, tmpl)
	attendanceHandler := handlers2.NewAttendanceHandler(database, tmpl)
	adminHandler := handlers2.NewAdminHandler(database, tmpl)
	apiHandler := handlers2.NewAPIHandler(database)
	labHandler := handlers2.NewLabHandler(database, tmpl)

	// Basic routes
	router.HandleFunc("/", authHandler.IndexHandler).Methods("GET")
	router.HandleFunc("/register", authHandler.RegisterHandler).Methods("GET", "POST")
	router.HandleFunc("/login", authHandler.LoginHandler).Methods("GET", "POST")
	router.HandleFunc("/logout", authHandler.LogoutHandler).Methods("GET")

	// Subscription page - accessible to logged in users, including free users
	router.HandleFunc("/subscription", handlers2.AuthMiddleware(authHandler.SubscriptionHandler)).Methods("GET")

	// Protected routes - require a paid subscription
	router.HandleFunc("/dashboard", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, dashboardHandler.DashboardHandler))).Methods("GET")

	// Lesson management
	router.HandleFunc("/lesson/add", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, lessonHandler.AddLessonHandler))).Methods("GET", "POST")
	router.HandleFunc("/lessons/subject", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, lessonHandler.SubjectLessonsHandler))).Methods("GET", "POST")
	router.HandleFunc("/lesson/edit/{id}", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, lessonHandler.EditLessonHandler))).Methods("GET", "POST")
	router.HandleFunc("/export", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, lessonHandler.ExportExcelHandler))).Methods("GET")

	// Group management
	router.HandleFunc("/groups", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, groupHandler.GroupsHandler))).Methods("GET", "POST")
	router.HandleFunc("/groups/edit/{groupName}", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, groupHandler.EditGroupHandler))).Methods("GET", "POST")
	router.HandleFunc("/groups/add", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, groupHandler.AddGroupHandler))).Methods("GET", "POST")

	// Attendance management
	router.HandleFunc("/attendance/add", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, attendanceHandler.AddAttendanceHandler))).Methods("GET", "POST")
	router.HandleFunc("/attendance", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, attendanceHandler.AttendanceHandler))).Methods("GET", "POST")
	router.HandleFunc("/attendance/view/{id}", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, attendanceHandler.ViewAttendanceHandler))).Methods("GET")
	router.HandleFunc("/attendance/edit/{id}", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, attendanceHandler.EditAttendanceHandler))).Methods("GET", "POST")
	router.HandleFunc("/export/attendance", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, attendanceHandler.ExportAttendanceExcelHandler))).Methods("GET")

	// Admin routes - admin middleware already checks role
	router.HandleFunc("/admin", handlers2.AdminMiddleware(database, adminHandler.AdminDashboardHandler)).Methods("GET")
	router.HandleFunc("/admin/", handlers2.AdminMiddleware(database, adminHandler.AdminDashboardHandler)).Methods("GET")
	router.HandleFunc("/admin/users", handlers2.AdminMiddleware(database, adminHandler.AdminUsersHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/logs", handlers2.AdminMiddleware(database, adminHandler.AdminLogsHandler)).Methods("GET")
	router.HandleFunc("/admin/groups", handlers2.AdminMiddleware(database, adminHandler.AdminTeacherGroupsHandler)).Methods("GET")
	router.HandleFunc("/admin/groups/add/{teacherID}", handlers2.AdminMiddleware(database, adminHandler.AdminAddGroupHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/groups/edit/{teacherID}/{groupName}", handlers2.AdminMiddleware(database, adminHandler.AdminEditGroupHandler)).Methods("GET", "POST")

	// Admin attendance management routes
	router.HandleFunc("/admin/attendance", handlers2.AdminMiddleware(database, adminHandler.AdminAttendanceHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/attendance/view/{id}", handlers2.AdminMiddleware(database, adminHandler.AdminViewAttendanceHandler)).Methods("GET")
	router.HandleFunc("/admin/attendance/edit/{id}", handlers2.AdminMiddleware(database, adminHandler.AdminEditAttendanceHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/attendance/export", handlers2.AdminMiddleware(database, adminHandler.AdminExportAttendanceHandler)).Methods("GET")
	router.HandleFunc("/admin/attendance/{teacherID}/{groupName}", handlers2.AdminMiddleware(database, adminHandler.AdminAttendanceHandler)).Methods("GET")

	// API endpoints
	router.HandleFunc("/api/lessons", apiHandler.APILessonsHandler).Methods("GET")
	router.HandleFunc("/api/students", apiHandler.APIStudentsHandler).Methods("GET")

	// Lab grades management
	router.HandleFunc("/labs", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, labHandler.GroupLabsHandler))).Methods("GET")
	router.HandleFunc("/labs/view/{subject}/{group}", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, labHandler.ViewLabGradesHandler))).Methods("GET")
	router.HandleFunc("/labs/grades/{subject}/{group}", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, labHandler.LabGradesHandler))).Methods("GET", "POST")
	router.HandleFunc("/labs/export/{subject}/{group}", handlers2.AuthMiddleware(handlers2.SubscriberMiddleware(database, labHandler.ExportLabGradesHandler))).Methods("GET")

	// Admin labs management routes
	router.HandleFunc("/admin/labs", handlers2.AdminMiddleware(database, adminHandler.AdminLabsHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/labs/view/{teacherID}/{subject}/{group}", handlers2.AdminMiddleware(database, adminHandler.AdminViewLabGradesHandler)).Methods("GET")
	router.HandleFunc("/admin/labs/edit/{teacherID}/{subject}/{group}", handlers2.AdminMiddleware(database, adminHandler.AdminEditLabGradesHandler)).Methods("GET", "POST")
	router.HandleFunc("/admin/labs/export/{teacherID}/{subject}/{group}", handlers2.AdminMiddleware(database, adminHandler.AdminExportLabGradesHandler)).Methods("GET")

	// Initialize ticket system middleware
	router.Use(ticketMiddleware.TicketSystemMiddleware)

	log.Println("Server started on :8080")
	http.ListenAndServe(":8080", router)
}

package main

import (
	"TeacherJournal/app/dashboard/handlers"
	"TeacherJournal/app/tickets/db"
	ticketHandlers "TeacherJournal/app/tickets/handlers"
	"TeacherJournal/config"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
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
		"formatTimeAgo": func(t interface{}) string {
			switch v := t.(type) {
			case string:
				// Parse time from string if needed
				return v
			default:
				return fmt.Sprintf("%v", v)
			}
		},
		"statusClass": func(status string) string {
			switch status {
			case "New":
				return "status-new"
			case "Open":
				return "status-open"
			case "InProgress":
				return "status-progress"
			case "Resolved":
				return "status-resolved"
			case "Closed":
				return "status-closed"
			default:
				return ""
			}
		},
		"priorityClass": func(priority string) string {
			switch priority {
			case "Low":
				return "priority-low"
			case "Medium":
				return "priority-medium"
			case "High":
				return "priority-high"
			case "Critical":
				return "priority-critical"
			default:
				return ""
			}
		},
	}
}

func main() {
	// Initialize the database
	database := db.InitTicketDB()

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
	templatePath := filepath.Join("app", "tickets", "templates", "*.html")
	tmpl, err = tmpl.ParseGlob(templatePath)
	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	// Initialize ticket handler
	ticketHandler := ticketHandlers.NewTicketHandler(database, tmpl)

	// Ticket system routes
	router.HandleFunc("/tickets", handlers.AuthMiddleware(ticketHandler.TicketDashboardHandler)).Methods("GET")
	router.HandleFunc("/tickets/create", handlers.AuthMiddleware(ticketHandler.CreateTicketHandler)).Methods("GET", "POST")
	router.HandleFunc("/tickets/view/{id:[0-9]+}", handlers.AuthMiddleware(ticketHandler.ViewTicketHandler)).Methods("GET")
	router.HandleFunc("/tickets/update/{id:[0-9]+}", handlers.AuthMiddleware(ticketHandler.UpdateTicketHandler)).Methods("POST")
	router.HandleFunc("/tickets/comment/{id:[0-9]+}", handlers.AuthMiddleware(ticketHandler.AddCommentHandler)).Methods("POST")
	router.HandleFunc("/tickets/download/{id:[0-9]+}", handlers.AuthMiddleware(ticketHandler.DownloadAttachmentHandler)).Methods("GET")
	router.HandleFunc("/tickets/api/{action}", handlers.AuthMiddleware(ticketHandler.TicketAPIHandler)).Methods("GET", "POST")

	// Set up static file server for ticket attachments
	fileServer := http.FileServer(http.Dir(config.AttachmentStoragePath))
	router.PathPrefix("/attachments/").Handler(http.StripPrefix("/attachments/", fileServer))

	log.Printf("Ticket system server started on :%d\n", config.TicketSystemPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", config.TicketSystemPort), router); err != nil {
		log.Fatal(err)
	}
}

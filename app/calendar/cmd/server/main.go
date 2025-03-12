package main

import (
	"TeacherJournal/app/calendar/db"
	"TeacherJournal/app/calendar/handlers"
	"TeacherJournal/app/calendar/utils"
	dashboardDB "TeacherJournal/app/dashboard/db"
	dashboardMiddleware "TeacherJournal/app/dashboard/handlers"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

const (
	templatesDir = "app/calendar/templates"
	staticDir    = "app/calendar/templates/static"
	port         = 8092
)

func main() {
	// Initialize dashboard database
	database := dashboardDB.InitDB()

	// Initialize calendar database
	err := db.InitCalendarDB(database)
	if err != nil {
		log.Fatalf("Failed to initialize calendar database: %v", err)
	}

	// Create uploads directory
	os.MkdirAll(filepath.Join("uploads", "calendar"), 0755)

	// Load templates with custom functions
	tmpl := template.New("").Funcs(utils.GetTemplateFuncMap())
	tmpl = template.Must(tmpl.ParseGlob(filepath.Join(templatesDir, "*.html")))

	// Create handlers
	calendarHandler := handlers.NewCalendarHandler(database, tmpl)

	// Create router
	r := mux.NewRouter()

	// Configure routes
	r.HandleFunc("/calendar", dashboardMiddleware.AuthMiddleware(calendarHandler.IndexHandler)).Methods("GET")
	r.HandleFunc("/calendar/events", dashboardMiddleware.AuthMiddleware(calendarHandler.GetEventsJSON)).Methods("GET")
	r.HandleFunc("/calendar/create", dashboardMiddleware.AuthMiddleware(calendarHandler.CreateEventHandler)).Methods("GET", "POST")
	r.HandleFunc("/calendar/event/{id:[0-9]+}", dashboardMiddleware.AuthMiddleware(calendarHandler.ViewEventHandler)).Methods("GET")
	r.HandleFunc("/calendar/event/{id:[0-9]+}/edit", dashboardMiddleware.AuthMiddleware(calendarHandler.EditEventHandler)).Methods("GET", "POST")
	r.HandleFunc("/calendar/event/{id:[0-9]+}/delete", dashboardMiddleware.AuthMiddleware(calendarHandler.DeleteEventHandler)).Methods("POST")
	r.HandleFunc("/calendar/attachment/{id:[0-9]+}", dashboardMiddleware.AuthMiddleware(calendarHandler.DownloadAttachmentHandler)).Methods("GET")
	r.HandleFunc("/calendar/attachment/{id:[0-9]+}/delete", dashboardMiddleware.AuthMiddleware(calendarHandler.DeleteAttachmentHandler)).Methods("POST")

	// Serve static files
	r.PathPrefix("/static/calendar/").Handler(http.StripPrefix("/static/calendar/", http.FileServer(http.Dir(staticDir))))

	// Start server
	fmt.Printf("Calendar service running at http://localhost:%d/calendar\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), r))
}

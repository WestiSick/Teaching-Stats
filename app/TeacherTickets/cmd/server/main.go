package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"TeacherJournal/app/TeacherTickets/config"
	"TeacherJournal/app/TeacherTickets/db"
	"TeacherJournal/app/TeacherTickets/handlers"
)

func main() {
	// Initialize database
	database := db.InitDB()
	defer database.Close()

	// Parse templates
	tmpl := template.New("")
	tmpl = tmpl.Funcs(handlers.CreateTemplateHelperFunctions())
	tmpl, err := tmpl.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	// Initialize router
	router := mux.NewRouter()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(database, tmpl)
	ticketHandler := handlers.NewTicketHandler(database, tmpl)
	apiHandler := handlers.NewAPIHandler(database)
	adminTicketHandler := handlers.NewAdminTicketHandler(database, tmpl)

	// Static files
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Auth routes (reused from Teaching Stats)
	router.HandleFunc("/", authHandler.IndexHandler).Methods("GET")
	router.HandleFunc("/register", authHandler.RegisterHandler).Methods("GET", "POST")
	router.HandleFunc("/login", authHandler.LoginHandler).Methods("GET", "POST")
	router.HandleFunc("/logout", authHandler.LogoutHandler).Methods("GET")

	// Ticket routes
	router.HandleFunc("/tickets", handlers.AuthMiddleware(ticketHandler.ListTicketsHandler)).Methods("GET")
	router.HandleFunc("/tickets/create", handlers.AuthMiddleware(ticketHandler.CreateTicketHandler)).Methods("GET", "POST")
	router.HandleFunc("/tickets/{id:[0-9]+}", handlers.AuthMiddleware(ticketHandler.ViewTicketHandler)).Methods("GET")
	router.HandleFunc("/tickets/{id:[0-9]+}/update", handlers.AuthMiddleware(ticketHandler.UpdateTicketHandler)).Methods("GET", "POST")
	router.HandleFunc("/tickets/{id:[0-9]+}/comment", handlers.AuthMiddleware(ticketHandler.AddCommentHandler)).Methods("POST")
	router.HandleFunc("/tickets/{ticket_id:[0-9]+}/attachments", handlers.AuthMiddleware(ticketHandler.UploadAttachmentHandler)).Methods("POST")
	router.HandleFunc("/tickets/{ticket_id:[0-9]+}/attachments/{attachment_id:[0-9]+}", handlers.AuthMiddleware(ticketHandler.DownloadAttachmentHandler)).Methods("GET")

	// Admin routes
	router.HandleFunc("/admin/tickets", handlers.AdminMiddleware(database, adminTicketHandler.AdminTicketsHandler)).Methods("GET")
	router.HandleFunc("/admin/tickets/statistics", handlers.AdminMiddleware(database, adminTicketHandler.AdminTicketStatsHandler)).Methods("GET")
	router.HandleFunc("/admin/tickets/{id:[0-9]+}/assign", handlers.AdminMiddleware(database, adminTicketHandler.AdminAssignTicketHandler)).Methods("POST")

	// API routes
	router.HandleFunc("/api/tickets", handlers.AuthMiddleware(apiHandler.APITicketsHandler)).Methods("GET", "POST")
	router.HandleFunc("/api/tickets/{id:[0-9]+}", handlers.AuthMiddleware(apiHandler.APITicketHandler)).Methods("GET", "PUT", "DELETE")
	router.HandleFunc("/api/tickets/{id:[0-9]+}/status", handlers.AuthMiddleware(apiHandler.APITicketStatusHandler)).Methods("PUT")
	router.HandleFunc("/api/tickets/{id:[0-9]+}/comments", handlers.AuthMiddleware(apiHandler.APITicketCommentsHandler)).Methods("GET", "POST")
	router.HandleFunc("/api/tickets/{ticket_id:[0-9]+}/comments/{comment_id:[0-9]+}", handlers.AuthMiddleware(apiHandler.APITicketCommentHandler)).Methods("GET", "DELETE")
	router.HandleFunc("/api/user/notification-settings", handlers.AuthMiddleware(apiHandler.APINotificationSettingsHandler)).Methods("GET", "PUT")

	// Start server
	log.Println("Ticket System server started on :8090")
	log.Fatal(http.ListenAndServe(":8090", router))
}

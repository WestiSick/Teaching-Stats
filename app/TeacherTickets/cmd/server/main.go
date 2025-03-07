package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"TeacherJournal/app/TeacherTickets/handlers"
	"TeacherJournal/app/dashboard/db"
	handlers2 "TeacherJournal/app/dashboard/handlers"
)

func main() {
	// Initialize database
	database := db.InitDB()
	defer database.Close()

	// Parse templates
	tmpl := template.New("")
	tmpl = tmpl.Funcs(handlers.CreateTemplateHelperFunctions())
	tmpl, err := tmpl.ParseGlob("../../templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	// Initialize router
	router := mux.NewRouter()

	// Initialize handlers
	authHandler := handlers2.NewAuthHandler(database, tmpl)
	ticketHandler := handlers.NewTicketHandler(database, tmpl)
	apiHandler := handlers.NewAPIHandler(database) // Use our custom API handler
	adminTicketHandler := handlers.NewAdminTicketHandler(database, tmpl)

	// Static files
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Auth routes (reused from Teaching Stats)
	router.HandleFunc("/", authHandler.IndexHandler).Methods("GET")
	router.HandleFunc("/register", authHandler.RegisterHandler).Methods("GET", "POST")
	router.HandleFunc("/login", authHandler.LoginHandler).Methods("GET", "POST")
	router.HandleFunc("/logout", authHandler.LogoutHandler).Methods("GET")

	// Ticket routes
	router.HandleFunc("/tickets", handlers2.AuthMiddleware(ticketHandler.ListTicketsHandler)).Methods("GET")
	router.HandleFunc("/tickets/create", handlers2.AuthMiddleware(ticketHandler.CreateTicketHandler)).Methods("GET", "POST")
	router.HandleFunc("/tickets/{id:[0-9]+}", handlers2.AuthMiddleware(ticketHandler.ViewTicketHandler)).Methods("GET")
	router.HandleFunc("/tickets/{id:[0-9]+}/update", handlers2.AuthMiddleware(ticketHandler.UpdateTicketHandler)).Methods("GET", "POST")
	router.HandleFunc("/tickets/{id:[0-9]+}/comment", handlers2.AuthMiddleware(ticketHandler.AddCommentHandler)).Methods("POST")
	router.HandleFunc("/tickets/{ticket_id:[0-9]+}/attachments", handlers2.AuthMiddleware(ticketHandler.UploadAttachmentHandler)).Methods("POST")
	router.HandleFunc("/tickets/{ticket_id:[0-9]+}/attachments/{attachment_id:[0-9]+}", handlers2.AuthMiddleware(ticketHandler.DownloadAttachmentHandler)).Methods("GET")

	// Admin routes
	router.HandleFunc("/admin/tickets", handlers2.AdminMiddleware(database, adminTicketHandler.AdminTicketsHandler)).Methods("GET")
	router.HandleFunc("/admin/tickets/statistics", handlers2.AdminMiddleware(database, adminTicketHandler.AdminTicketStatsHandler)).Methods("GET")
	router.HandleFunc("/admin/tickets/{id:[0-9]+}/assign", handlers2.AdminMiddleware(database, adminTicketHandler.AdminAssignTicketHandler)).Methods("POST")

	// API routes - using our custom implementation
	router.HandleFunc("/api/tickets", handlers2.AuthMiddleware(apiHandler.APITicketsHandler)).Methods("GET", "POST")
	router.HandleFunc("/api/tickets/{id:[0-9]+}", handlers2.AuthMiddleware(apiHandler.APITicketHandler)).Methods("GET", "PUT", "DELETE")
	router.HandleFunc("/api/tickets/{id:[0-9]+}/status", handlers2.AuthMiddleware(apiHandler.APITicketStatusHandler)).Methods("PUT")
	router.HandleFunc("/api/tickets/{id:[0-9]+}/comments", handlers2.AuthMiddleware(apiHandler.APITicketCommentsHandler)).Methods("GET", "POST")
	router.HandleFunc("/api/tickets/{ticket_id:[0-9]+}/comments/{comment_id:[0-9]+}", handlers2.AuthMiddleware(apiHandler.APITicketCommentHandler)).Methods("GET", "DELETE")
	router.HandleFunc("/api/user/notification-settings", handlers2.AuthMiddleware(apiHandler.APINotificationSettingsHandler)).Methods("GET", "PUT")

	// Start server
	log.Println("Ticket System server started on :8090")
	log.Fatal(http.ListenAndServe(":8090", router))
}

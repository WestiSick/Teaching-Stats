package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"TeacherJournal/app/TeacherTickets/config"
	"TeacherJournal/app/TeacherTickets/db"
	"TeacherJournal/app/TeacherTickets/models"
)

// TicketHandler handles ticket-related HTTP requests
type TicketHandler struct {
	DB   *sql.DB
	Tmpl *template.Template
}

// NewTicketHandler creates a new TicketHandler
func NewTicketHandler(database *sql.DB, tmpl *template.Template) *TicketHandler {
	return &TicketHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// ListTicketsHandler handles listing tickets
func (h *TicketHandler) ListTicketsHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if pageNum, err := strconv.Atoi(pageStr); err == nil && pageNum > 0 {
			page = pageNum
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limitNum, err := strconv.Atoi(limitStr); err == nil && limitNum > 0 {
			limit = limitNum
		}
	}

	// Get filters
	filters := make(map[string]string)
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}
	if priority := r.URL.Query().Get("priority"); priority != "" {
		filters["priority"] = priority
	}
	if category := r.URL.Query().Get("category"); category != "" {
		filters["category"] = category
	}

	// Determine if the user is an admin
	isAdmin := userInfo.Role == "admin"

	// Get tickets
	paginatedTickets, err := db.GetTickets(h.DB, userInfo.ID, isAdmin, filters, page, limit)
	if err != nil {
		HandleError(w, err, "Error retrieving tickets", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := struct {
		User           models.UserInfo
		Tickets        []models.Ticket
		Pagination     models.Pagination
		Filters        map[string]string
		StatusValues   []string
		PriorityValues []string
		CategoryValues []string
	}{
		User:           userInfo,
		Tickets:        paginatedTickets.Tickets,
		Pagination:     paginatedTickets.Pagination,
		Filters:        filters,
		StatusValues:   config.TicketStatusValues,
		PriorityValues: config.TicketPriorityValues,
		CategoryValues: config.TicketCategoryValues,
	}

	// Render template
	renderTemplate(w, h.Tmpl, "ticket_list.html", data)
}

// CreateTicketHandler handles ticket creation
func (h *TicketHandler) CreateTicketHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Handle form submission
	if r.Method == "POST" {
		// Parse form values
		title := r.FormValue("title")
		description := r.FormValue("description")
		priority := r.FormValue("priority")
		category := r.FormValue("category")

		// Validate inputs
		if title == "" || description == "" || priority == "" || category == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		// Create ticket
		ticket := &models.Ticket{
			CreatorID:   userInfo.ID,
			Title:       title,
			Description: description,
			Status:      "New", // Initial status
			Priority:    priority,
			Category:    category,
		}

		// Save ticket to database
		err := db.CreateTicket(h.DB, ticket)
		if err != nil {
			HandleError(w, err, "Error creating ticket", http.StatusInternalServerError)
			return
		}

		// Log action
		db.LogAction(h.DB, userInfo.ID, "Create Ticket", fmt.Sprintf("Created ticket #%d: %s", ticket.ID, title))

		// Redirect to ticket detail page
		http.Redirect(w, r, fmt.Sprintf("/tickets/%d", ticket.ID), http.StatusSeeOther)
		return
	}

	// For GET requests, display the form
	data := struct {
		User           models.UserInfo
		PriorityValues []string
		CategoryValues []string
	}{
		User:           userInfo,
		PriorityValues: config.TicketPriorityValues,
		CategoryValues: config.TicketCategoryValues,
	}

	// Render template
	renderTemplate(w, h.Tmpl, "ticket_create.html", data)
}

// ViewTicketHandler handles viewing a ticket
func (h *TicketHandler) ViewTicketHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get ticket ID from URL
	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Get ticket details
	ticket, err := db.GetTicketDetails(h.DB, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Determine if user can edit the ticket (admin or creator)
	canEdit := userInfo.Role == "admin" || ticket.CreatorID == userInfo.ID

	// Prepare template data
	data := struct {
		User           models.UserInfo
		Ticket         *models.Ticket
		StatusValues   []string
		PriorityValues []string
		CategoryValues []string
		CanEdit        bool
	}{
		User:           userInfo,
		Ticket:         ticket,
		StatusValues:   config.TicketStatusValues,
		PriorityValues: config.TicketPriorityValues,
		CategoryValues: config.TicketCategoryValues,
		CanEdit:        canEdit,
	}

	// Render template
	renderTemplate(w, h.Tmpl, "ticket_detail.html", data)
}

// UpdateTicketHandler handles updating a ticket
func (h *TicketHandler) UpdateTicketHandler(w http.ResponseWriter, r *http.Request) {
	// Implementation would be similar to ViewTicketHandler but with form processing
}

// AddCommentHandler handles adding a comment to a ticket
func (h *TicketHandler) AddCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Implementation would handle form submission for adding comments
}

// UploadAttachmentHandler handles file uploads to tickets
func (h *TicketHandler) UploadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	// Implementation would handle file uploads
}

// DownloadAttachmentHandler handles downloading attachments
func (h *TicketHandler) DownloadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	// Implementation would handle file downloads
}

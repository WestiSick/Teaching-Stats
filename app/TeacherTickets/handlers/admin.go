package handlers

import (
	"TeacherJournal/config"
	"database/sql"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"TeacherJournal/app/TeacherTickets/models"
	"TeacherJournal/app/dashboard/db"
)

// AdminTicketHandler handles admin-specific ticket operations
type AdminTicketHandler struct {
	DB   *sql.DB
	Tmpl *template.Template
}

// NewAdminTicketHandler creates a new AdminTicketHandler
func NewAdminTicketHandler(database *sql.DB, tmpl *template.Template) *AdminTicketHandler {
	return &AdminTicketHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// AdminTicketsHandler displays all tickets for admin
func (h *AdminTicketHandler) AdminTicketsHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Convert to models.UserInfo
	userInfo := ConvertUserInfo(dbUserInfo)

	// Parse query parameters
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if pageNum, err := strconv.Atoi(pageStr); err == nil && pageNum > 0 {
			page = pageNum
		}
	}

	limit := 20
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
	if creatorID := r.URL.Query().Get("creator_id"); creatorID != "" {
		filters["creator_id"] = creatorID
	}
	if assigneeID := r.URL.Query().Get("assignee_id"); assigneeID != "" {
		filters["assignee_id"] = assigneeID
	}

	// Get all tickets (admin view)
	paginatedTickets, err := db.GetAllTickets(h.DB, filters, page, limit)
	if err != nil {
		HandleError(w, err, "Error retrieving tickets", http.StatusInternalServerError)
		return
	}

	// Get ticket statistics
	statistics, err := db.GetTicketStatistics(h.DB)
	if err != nil {
		statistics = &models.TicketStatistics{
			ByStatus:   make(map[string]int),
			ByPriority: make(map[string]int),
			ByCategory: make(map[string]int),
		}
	}

	// Get all users for filtering
	var users []models.UserInfo
	rows, err := h.DB.Query("SELECT id, fio FROM users ORDER BY fio")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var user models.UserInfo
			rows.Scan(&user.ID, &user.FIO)
			users = append(users, user)
		}
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
		Users          []models.UserInfo
		Statistics     *models.TicketStatistics
	}{
		User:           userInfo,
		Tickets:        paginatedTickets.Tickets,
		Pagination:     paginatedTickets.Pagination,
		Filters:        filters,
		StatusValues:   config.TicketStatusValues,
		PriorityValues: config.TicketPriorityValues,
		CategoryValues: config.TicketCategoryValues,
		Users:          users,
		Statistics:     statistics,
	}

	// Render template
	renderTemplate(w, h.Tmpl, "admin_tickets.html", data)
}

// AdminTicketStatsHandler displays ticket statistics for admin
func (h *AdminTicketHandler) AdminTicketStatsHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Convert to models.UserInfo
	userInfo := ConvertUserInfo(dbUserInfo)

	// Get detailed ticket statistics
	statistics, err := db.GetDetailedTicketStatistics(h.DB)
	if err != nil {
		HandleError(w, err, "Error retrieving ticket statistics", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := struct {
		User       models.UserInfo
		Statistics *models.TicketStatistics
	}{
		User:       userInfo,
		Statistics: statistics,
	}

	// Render template
	renderTemplate(w, h.Tmpl, "admin_ticket_stats.html", data)
}

// AdminAssignTicketHandler handles assignment of tickets to users
func (h *AdminTicketHandler) AdminAssignTicketHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

	// Get assignee ID from form
	assigneeIDStr := r.FormValue("assignee_id")

	// Get ticket from database
	ticket, err := db.GetTicket(h.DB, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Update the assignee
	if assigneeIDStr != "" {
		assigneeID, err := strconv.Atoi(assigneeIDStr)
		if err != nil {
			http.Error(w, "Invalid assignee ID", http.StatusBadRequest)
			return
		}

		// Set the assignee ID
		ticket.AssigneeID = &assigneeID

		// If ticket is in 'New' status, update to 'Open'
		if ticket.Status == "New" {
			ticket.Status = "Open"
		}

		// Update the ticket
		err = db.UpdateTicket(h.DB, ticket)
		if err != nil {
			HandleError(w, err, "Error updating ticket", http.StatusInternalServerError)
			return
		}

		// Log the assignment
		db.LogAction(h.DB, dbUserInfo.ID, "Assign Ticket",
			"Assigned ticket #"+strconv.Itoa(ticketID)+" to user #"+assigneeIDStr)
	}

	// Redirect back to admin tickets list
	http.Redirect(w, r, "/admin/tickets", http.StatusSeeOther)
}

package handlers

import (
	db2 "TeacherJournal/app/dashboard/db"
	"TeacherJournal/config"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"TeacherJournal/app/TeacherTickets/models"
	"TeacherJournal/app/dashboard/db"
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
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Преобразуем db.UserInfo в models.UserInfo
	userInfo := ConvertUserInfo(dbUserInfo)

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
	isAdmin := dbUserInfo.Role == "admin"

	// Get tickets
	paginatedTickets, err := db.GetTickets(h.DB, dbUserInfo.ID, isAdmin, filters, page, limit)
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
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Преобразуем db.UserInfo в models.UserInfo
	userInfo := ConvertUserInfo(dbUserInfo)

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
			CreatorID:   dbUserInfo.ID,
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
		db.LogAction(h.DB, dbUserInfo.ID, "Create Ticket", fmt.Sprintf("Created ticket #%d: %s", ticket.ID, title))

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
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Преобразуем db.UserInfo в models.UserInfo
	userInfo := ConvertUserInfo(dbUserInfo)

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
	canEdit := dbUserInfo.Role == "admin" || ticket.CreatorID == dbUserInfo.ID

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
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Convert db.UserInfo to models.UserInfo
	userInfo := ConvertUserInfo(dbUserInfo)

	// Get ticket ID from URL
	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Get ticket to be updated
	ticket, err := db2.GetTicket(h.DB, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Check if user has permission to edit this ticket
	// Only the creator, assignee, or admin can edit a ticket
	if dbUserInfo.Role != "admin" && dbUserInfo.ID != ticket.CreatorID &&
		(ticket.AssigneeID == nil || dbUserInfo.ID != *ticket.AssigneeID) {
		http.Error(w, "You don't have permission to edit this ticket", http.StatusForbidden)
		return
	}

	if r.Method == "POST" {
		// Parse form values
		title := r.FormValue("title")
		description := r.FormValue("description")
		priority := r.FormValue("priority")
		category := r.FormValue("category")
		status := r.FormValue("status")

		// Basic validation
		if title == "" || description == "" || priority == "" || category == "" || status == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		// Check if status has changed
		statusChanged := ticket.Status != status

		// Update ticket fields
		ticket.Title = title
		ticket.Description = description
		ticket.Priority = priority
		ticket.Category = category
		ticket.Status = status

		// Update assignee if admin and provided
		if dbUserInfo.Role == "admin" && r.FormValue("assignee_id") != "" {
			assigneeID, err := strconv.Atoi(r.FormValue("assignee_id"))
			if err == nil {
				ticket.AssigneeID = &assigneeID
			}
		}

		// Update ticket in database
		err = db2.UpdateTicket(h.DB, ticket)
		if err != nil {
			HandleError(w, err, "Error updating ticket", http.StatusInternalServerError)
			return
		}

		// If status changed, add to history
		if statusChanged {
			err = db2.UpdateTicketStatus(h.DB, ticketID, status, dbUserInfo.ID)
			if err != nil {
				log.Printf("Error updating ticket status history: %v", err)
			}
		}

		// Log action
		db.LogAction(h.DB, dbUserInfo.ID, "Update Ticket", fmt.Sprintf("Updated ticket #%d: %s", ticket.ID, title))

		// Redirect to ticket detail page
		http.Redirect(w, r, fmt.Sprintf("/tickets/%d", ticket.ID), http.StatusSeeOther)
		return
	}

	// Get all users for assignee selection (admin only)
	var users []models.UserInfo
	if dbUserInfo.Role == "admin" {
		rows, err := h.DB.Query("SELECT id, fio FROM users ORDER BY fio")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var user models.UserInfo
				rows.Scan(&user.ID, &user.FIO)
				users = append(users, user)
			}
		}
	}

	// Determine if user can edit status
	canEditStatus := dbUserInfo.Role == "admin" || (ticket.AssigneeID != nil && *ticket.AssigneeID == dbUserInfo.ID)

	// Prepare template data
	data := struct {
		User           models.UserInfo
		Ticket         *models.Ticket
		StatusValues   []string
		PriorityValues []string
		CategoryValues []string
		Users          []models.UserInfo
		CanEditStatus  bool
	}{
		User:           userInfo,
		Ticket:         ticket,
		StatusValues:   config.TicketStatusValues,
		PriorityValues: config.TicketPriorityValues,
		CategoryValues: config.TicketCategoryValues,
		Users:          users,
		CanEditStatus:  canEditStatus,
	}

	// Render template
	renderTemplate(w, h.Tmpl, "ticket_edit.html", data)
}

// AddCommentHandler handles adding a comment to a ticket
func (h *TicketHandler) AddCommentHandler(w http.ResponseWriter, r *http.Request) {
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

	// Verify ticket exists
	_, err = db2.GetTicket(h.DB, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Process form submission
	if r.Method == "POST" {
		// Parse form
		comment := r.FormValue("comment")

		// Validate
		if comment == "" {
			http.Error(w, "Comment cannot be empty", http.StatusBadRequest)
			return
		}

		// Create comment
		newComment := &models.TicketComment{
			TicketID: ticketID,
			UserID:   dbUserInfo.ID,
			Comment:  comment,
		}

		// Save to database
		err = db2.AddTicketComment(h.DB, newComment)
		if err != nil {
			HandleError(w, err, "Error adding comment", http.StatusInternalServerError)
			return
		}

		// Log action
		db.LogAction(h.DB, dbUserInfo.ID, "Add Comment",
			fmt.Sprintf("Added comment to ticket #%d", ticketID))

		// Redirect back to ticket detail page
		http.Redirect(w, r, fmt.Sprintf("/tickets/%d", ticketID), http.StatusSeeOther)
		return
	}

	// If not POST, redirect to ticket detail page
	http.Redirect(w, r, fmt.Sprintf("/tickets/%d", ticketID), http.StatusSeeOther)
}

// UploadAttachmentHandler handles file uploads for tickets
func (h *TicketHandler) UploadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get ticket ID from URL
	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["ticket_id"])
	if err != nil {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Verify ticket exists
	_, err = db2.GetTicket(h.DB, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Process form submission
	if r.Method == "POST" {
		// Parse multipart form (10 MB max memory)
		err = r.ParseMultipartForm(10 << 20)
		if err != nil {
			HandleError(w, err, "Error parsing form", http.StatusBadRequest)
			return
		}

		// Get uploaded file
		file, handler, err := r.FormFile("attachment")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Validate file size
		if handler.Size > config.MaxFileSize {
			http.Error(w, fmt.Sprintf("File too large (max %d MB)", config.MaxFileSize/(1024*1024)),
				http.StatusBadRequest)
			return
		}

		// Create directory if it doesn't exist
		os.MkdirAll(config.AttachmentStoragePath, 0755)

		// Generate unique filename to prevent collisions
		fileExt := filepath.Ext(handler.Filename)
		uniqueID := fmt.Sprintf("%d_%d_%s", ticketID, dbUserInfo.ID, time.Now().Format("20060102150405"))
		safeFilename := fmt.Sprintf("%s%s", uniqueID, fileExt)
		filePath := filepath.Join(config.AttachmentStoragePath, safeFilename)

		// Create file on server
		dst, err := os.Create(filePath)
		if err != nil {
			HandleError(w, err, "Error saving file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Copy uploaded file data to destination file
		_, err = io.Copy(dst, file)
		if err != nil {
			HandleError(w, err, "Error saving file", http.StatusInternalServerError)
			return
		}

		// Create attachment record in database
		attachment := &models.Attachment{
			TicketID:   ticketID,
			FileName:   handler.Filename,
			FilePath:   safeFilename,
			UploadedBy: dbUserInfo.ID,
		}

		err = db2.AddTicketAttachment(h.DB, attachment)
		if err != nil {
			// Try to remove the file if database insertion fails
			os.Remove(filePath)
			HandleError(w, err, "Error saving attachment record", http.StatusInternalServerError)
			return
		}

		// Log action
		db.LogAction(h.DB, dbUserInfo.ID, "Add Attachment",
			fmt.Sprintf("Added attachment '%s' to ticket #%d", handler.Filename, ticketID))

		// Redirect back to ticket detail page
		http.Redirect(w, r, fmt.Sprintf("/tickets/%d", ticketID), http.StatusSeeOther)
		return
	}

	// If not POST, redirect to ticket detail page
	http.Redirect(w, r, fmt.Sprintf("/tickets/%d", ticketID), http.StatusSeeOther)
}

// DownloadAttachmentHandler handles downloading ticket attachments
func (h *TicketHandler) DownloadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get ticket and attachment IDs from URL
	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["ticket_id"])
	if err != nil {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	attachmentID, err := strconv.Atoi(vars["attachment_id"])
	if err != nil {
		http.Error(w, "Invalid attachment ID", http.StatusBadRequest)
		return
	}

	// Get attachment information from database
	var filename, filePath string
	err = h.DB.QueryRow(`
		SELECT file_name, file_path 
		FROM ticket_attachments 
		WHERE id = ? AND ticket_id = ?`,
		attachmentID, ticketID).Scan(&filename, &filePath)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Attachment not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving attachment", http.StatusInternalServerError)
		}
		return
	}

	// Construct full path to file
	fullPath := filepath.Join(config.AttachmentStoragePath, filePath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "Attachment file not found", http.StatusNotFound)
		return
	}

	// Set headers for file download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Serve the file
	http.ServeFile(w, r, fullPath)

	// Log download (optional)
	db.LogAction(h.DB, dbUserInfo.ID, "Download Attachment",
		fmt.Sprintf("Downloaded attachment '%s' from ticket #%d", filename, ticketID))
}

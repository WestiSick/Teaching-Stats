package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/models"
	dbTicket "TeacherJournal/app/tickets/db"
	ticketModels "TeacherJournal/app/tickets/models"
	"TeacherJournal/app/tickets/utils"
	"TeacherJournal/config"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// TicketHandler handles ticket-related routes
type TicketHandler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

// NewTicketHandler creates a new TicketHandler
func NewTicketHandler(database *gorm.DB, tmpl *template.Template) *TicketHandler {
	return &TicketHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// TicketDashboardHandler displays the ticket system dashboard
func (h *TicketHandler) TicketDashboardHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get filter parameters
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}

	// Get tickets based on user role
	tickets, err := dbTicket.GetUserTickets(h.DB, userInfo.ID, status, userInfo.Role)
	if err != nil {
		utils.HandleError(w, err, "Error retrieving tickets", http.StatusInternalServerError)
		return
	}

	// Prepare ticket data for display
	type TicketDisplay struct {
		ID           int
		Title        string
		Status       string
		Priority     string
		Category     string
		CreatedBy    string
		AssignedTo   string
		CreatedAt    string
		LastActivity string
		DaysAgo      int
	}

	var ticketsDisplay []TicketDisplay

	// Get user data for display
	var userIDs []int
	for _, ticket := range tickets {
		userIDs = append(userIDs, ticket.CreatedBy)
		if ticket.AssignedTo != nil {
			userIDs = append(userIDs, *ticket.AssignedTo)
		}
	}

	// Get user names from DB
	var users []struct {
		ID  int
		FIO string
	}
	h.DB.Model(&models.User{}).Where("id IN ?", userIDs).Select("id, fio").Find(&users)

	// Create map of userID to FIO
	userNames := make(map[int]string)
	for _, user := range users {
		userNames[user.ID] = user.FIO
	}

	// Format tickets for display
	for _, ticket := range tickets {
		createdByName := userNames[ticket.CreatedBy]
		var assignedToName string
		if ticket.AssignedTo != nil {
			assignedToName = userNames[*ticket.AssignedTo]
		}

		// Calculate days ago for last activity
		daysAgo := int(time.Since(ticket.LastActivity).Hours() / 24)

		ticketsDisplay = append(ticketsDisplay, TicketDisplay{
			ID:           ticket.ID,
			Title:        ticket.Title,
			Status:       ticket.Status,
			Priority:     ticket.Priority,
			Category:     ticket.Category,
			CreatedBy:    createdByName,
			AssignedTo:   assignedToName,
			CreatedAt:    ticket.CreatedAt.Format("02.01.2006 15:04"),
			LastActivity: ticket.LastActivity.Format("02.01.2006 15:04"),
			DaysAgo:      daysAgo,
		})
	}

	data := struct {
		User          db.UserInfo
		Tickets       []TicketDisplay
		Status        string
		StatusOptions []string
	}{
		User:          userInfo,
		Tickets:       ticketsDisplay,
		Status:        status,
		StatusOptions: config.TicketStatusValues,
	}

	renderTemplate(w, h.Tmpl, "ticket_dashboard.html", data)
}

// CreateTicketHandler handles the creation of new tickets
func (h *TicketHandler) CreateTicketHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info from session
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if r.Method == "GET" {
		data := struct {
			User            db.UserInfo
			PriorityOptions []string
			CategoryOptions []string
		}{
			User:            userInfo,
			PriorityOptions: config.TicketPriorityValues,
			CategoryOptions: config.TicketCategoryValues,
		}
		renderTemplate(w, h.Tmpl, "create_ticket.html", data)
		return
	}

	// Process form submission
	err = r.ParseMultipartForm(config.MaxFileSize)
	if err != nil {
		utils.HandleError(w, err, "Error parsing form", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	priority := r.FormValue("priority")
	category := r.FormValue("category")

	// Validate inputs
	if title == "" || description == "" || priority == "" || category == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	// Create ticket method in CreateTicketHandler
	ticket := ticketModels.Ticket{
		Title:        title,
		Description:  description,
		Status:       "New",
		Priority:     priority,
		Category:     category,
		CreatedBy:    userInfo.ID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// Add extra logging to track the creation process
	log.Printf("Creating ticket - User ID: %d, Title: %s", userInfo.ID, title)

	// Save ticket with robust error handling
	if err := dbTicket.CreateTicket(h.DB, &ticket); err != nil {
		log.Printf("Ticket creation error: %v", err)
		utils.HandleError(w, err, "Error creating ticket", http.StatusInternalServerError)
		return
	}

	// Handle file attachments
	files := r.MultipartForm.File["attachments"]
	for _, fileHeader := range files {
		if err := h.saveAttachment(ticket.ID, 0, userInfo.ID, fileHeader); err != nil {
			log.Printf("Error saving attachment: %v", err)
		}
	}

	// Redirect to the new ticket
	http.Redirect(w, r, fmt.Sprintf("/tickets/view/%d", ticket.ID), http.StatusSeeOther)
}

// ViewTicketHandler displays a single ticket
func (h *TicketHandler) ViewTicketHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Get ticket
	ticket, err := dbTicket.GetTicketByID(h.DB, ticketID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			utils.HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Check if user has access (admins can see all tickets, users only their own)
	if userInfo.Role != "admin" && ticket.CreatedBy != userInfo.ID && (ticket.AssignedTo == nil || *ticket.AssignedTo != userInfo.ID) {
		http.Error(w, "You don't have permission to view this ticket", http.StatusForbidden)
		return
	}

	// Get users for comments and ticket
	var userIDs []int
	userIDs = append(userIDs, ticket.CreatedBy)
	if ticket.AssignedTo != nil {
		userIDs = append(userIDs, *ticket.AssignedTo)
	}

	// Get comments for this ticket (including internal ones if admin)
	comments, err := dbTicket.GetTicketComments(h.DB, ticketID, userInfo.Role == "admin")
	if err != nil {
		utils.HandleError(w, err, "Error retrieving comments", http.StatusInternalServerError)
		return
	}

	// Get attachments
	attachments, err := dbTicket.GetTicketAttachments(h.DB, ticketID)
	if err != nil {
		utils.HandleError(w, err, "Error retrieving attachments", http.StatusInternalServerError)
		return
	}

	// Add comment user IDs to the list
	for _, comment := range comments {
		userIDs = append(userIDs, comment.UserID)
	}

	// Get user names
	var users []struct {
		ID  int
		FIO string
	}
	h.DB.Model(&models.User{}).Where("id IN ?", userIDs).Select("id, fio").Find(&users)

	// Create map of userID to FIO
	userNames := make(map[int]string)
	for _, user := range users {
		userNames[user.ID] = user.FIO
	}

	// Format comments for display
	type CommentDisplay struct {
		ID          int
		UserName    string
		Content     string
		CreatedAt   string
		IsInternal  bool
		Attachments []ticketModels.TicketAttachment
	}

	var commentsDisplay []CommentDisplay
	for _, comment := range comments {
		// Get attachments for this comment
		var commentAttachments []ticketModels.TicketAttachment
		for _, a := range attachments {
			if a.CommentID != nil && *a.CommentID == comment.ID {
				commentAttachments = append(commentAttachments, a)
			}
		}

		commentsDisplay = append(commentsDisplay, CommentDisplay{
			ID:          comment.ID,
			UserName:    userNames[comment.UserID],
			Content:     comment.Content,
			CreatedAt:   comment.CreatedAt.Format("02.01.2006 15:04"),
			IsInternal:  comment.IsInternal,
			Attachments: commentAttachments,
		})
	}

	// Get ticket attachments (not attached to comments)
	var ticketAttachments []ticketModels.TicketAttachment
	for _, a := range attachments {
		if a.CommentID == nil {
			ticketAttachments = append(ticketAttachments, a)
		}
	}

	// Get all admin users for assignment
	var adminUsers []struct {
		ID  int
		FIO string
	}
	h.DB.Model(&models.User{}).Where("role = ?", "admin").Select("id, fio").Find(&adminUsers)

	// Get ticket history for admins
	var historyRecords []ticketModels.TicketHistory
	if userInfo.Role == "admin" {
		historyRecords, _ = dbTicket.GetTicketHistory(h.DB, ticketID)
	}

	// Format history records for display
	type HistoryDisplay struct {
		UserName   string
		FieldName  string
		OldValue   string
		NewValue   string
		ChangeTime string
	}

	var historyDisplay []HistoryDisplay
	for _, record := range historyRecords {
		historyDisplay = append(historyDisplay, HistoryDisplay{
			UserName:   userNames[record.UserID],
			FieldName:  record.FieldName,
			OldValue:   record.OldValue,
			NewValue:   record.NewValue,
			ChangeTime: record.ChangeTime.Format("02.01.2006 15:04"),
		})
	}

	// Get assignee name with proper null checking
	var ticketAssignee string
	var assignedToID int
	if ticket.AssignedTo != nil {
		if assigneeName, ok := userNames[*ticket.AssignedTo]; ok {
			ticketAssignee = assigneeName
			assignedToID = *ticket.AssignedTo
		}
	}

	data := struct {
		User              db.UserInfo
		Ticket            ticketModels.Ticket
		TicketCreator     string
		TicketAssignee    string
		AssignedToID      int
		Comments          []CommentDisplay
		TicketAttachments []ticketModels.TicketAttachment
		AdminUsers        []struct {
			ID  int
			FIO string
		}
		PriorityOptions []string
		StatusOptions   []string
		CategoryOptions []string
		History         []HistoryDisplay
	}{
		User:              userInfo,
		Ticket:            ticket,
		TicketCreator:     userNames[ticket.CreatedBy],
		TicketAssignee:    ticketAssignee,
		AssignedToID:      assignedToID,
		Comments:          commentsDisplay,
		TicketAttachments: ticketAttachments,
		AdminUsers:        adminUsers,
		PriorityOptions:   config.TicketPriorityValues,
		StatusOptions:     config.TicketStatusValues,
		CategoryOptions:   config.TicketCategoryValues,
		History:           historyDisplay,
	}

	renderTemplate(w, h.Tmpl, "view_ticket.html", data)
}

// UpdateTicketHandler handles updating ticket properties
func (h *TicketHandler) UpdateTicketHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Get ticket
	ticket, err := dbTicket.GetTicketByID(h.DB, ticketID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			utils.HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Verify permissions
	if userInfo.Role != "admin" && ticket.CreatedBy != userInfo.ID {
		http.Error(w, "You don't have permission to update this ticket", http.StatusForbidden)
		return
	}
	// Parse form
	if err := r.ParseForm(); err != nil {
		utils.HandleError(w, err, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Prepare update data
	updates := make(map[string]interface{})

	// Users can update some fields
	if userInfo.Role == "admin" || ticket.CreatedBy == userInfo.ID {
		// Regular users can only update certain fields
		if r.FormValue("title") != "" && r.FormValue("title") != ticket.Title {
			updates["title"] = r.FormValue("title")
		}

		if r.FormValue("description") != "" && r.FormValue("description") != ticket.Description {
			updates["description"] = r.FormValue("description")
		}

		if r.FormValue("category") != "" && r.FormValue("category") != ticket.Category {
			updates["category"] = r.FormValue("category")
		}

		if r.FormValue("priority") != "" && r.FormValue("priority") != ticket.Priority {
			updates["priority"] = r.FormValue("priority")
		}
	}

	// Admin-only updates
	if userInfo.Role == "admin" {
		if r.FormValue("status") != "" && r.FormValue("status") != ticket.Status {
			updates["status"] = r.FormValue("status")
		}

		if r.FormValue("assigned_to") != "" {
			assignedToID, err := strconv.Atoi(r.FormValue("assigned_to"))
			if err == nil {
				// Check if the assignment is changing
				if ticket.AssignedTo == nil || *ticket.AssignedTo != assignedToID {
					updates["assigned_to"] = assignedToID
				}
			}
		} else if ticket.AssignedTo != nil {
			// Unassign the ticket
			var nilAssignment *int = nil
			updates["assigned_to"] = nilAssignment
		}
	}

	// Apply updates if any
	if len(updates) > 0 {
		if err := dbTicket.UpdateTicket(h.DB, ticketID, userInfo.ID, updates); err != nil {
			utils.HandleError(w, err, "Error updating ticket", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to the ticket
	http.Redirect(w, r, fmt.Sprintf("/tickets/view/%d", ticketID), http.StatusSeeOther)
}

// AddCommentHandler handles adding comments to a ticket
func (h *TicketHandler) AddCommentHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Get ticket
	ticket, err := dbTicket.GetTicketByID(h.DB, ticketID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			utils.HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Check if user has access
	if userInfo.Role != "admin" && ticket.CreatedBy != userInfo.ID && (ticket.AssignedTo == nil || *ticket.AssignedTo != userInfo.ID) {
		http.Error(w, "You don't have permission to comment on this ticket", http.StatusForbidden)
		return
	}

	// Parse form
	err = r.ParseMultipartForm(config.MaxFileSize)
	if err != nil {
		utils.HandleError(w, err, "Error parsing form", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "Comment content is required", http.StatusBadRequest)
		return
	}

	// Check if this is an internal note (admin only)
	isInternal := false
	if userInfo.Role == "admin" && r.FormValue("internal") == "1" {
		isInternal = true
	}

	// Create comment
	comment := ticketModels.TicketComment{
		TicketID:   ticketID,
		UserID:     userInfo.ID,
		Content:    content,
		CreatedAt:  time.Now(),
		IsInternal: isInternal,
	}

	if err := dbTicket.AddTicketComment(h.DB, &comment); err != nil {
		utils.HandleError(w, err, "Error adding comment", http.StatusInternalServerError)
		return
	}

	// Handle file attachments
	files := r.MultipartForm.File["attachments"]
	for _, fileHeader := range files {
		if err := h.saveAttachment(ticketID, comment.ID, userInfo.ID, fileHeader); err != nil {
			log.Printf("Error saving attachment: %v", err)
		}
	}

	// Update ticket status if necessary (for regular users only)
	if userInfo.Role != "admin" && ticket.Status == "Resolved" {
		// If user replies to a resolved ticket, reopen it
		updates := map[string]interface{}{
			"status": "InProgress",
		}
		dbTicket.UpdateTicket(h.DB, ticketID, userInfo.ID, updates)
	}

	http.Redirect(w, r, fmt.Sprintf("/tickets/view/%d", ticketID), http.StatusSeeOther)
}

// DownloadAttachmentHandler handles downloading attachments
func (h *TicketHandler) DownloadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	attachmentID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid attachment ID", http.StatusBadRequest)
		return
	}

	// Get attachment
	var attachment ticketModels.TicketAttachment
	if err := h.DB.First(&attachment, attachmentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Attachment not found", http.StatusNotFound)
		} else {
			utils.HandleError(w, err, "Error retrieving attachment", http.StatusInternalServerError)
		}
		return
	}

	// Get ticket to verify access
	ticket, err := dbTicket.GetTicketByID(h.DB, attachment.TicketID)
	if err != nil {
		utils.HandleError(w, err, "Error retrieving ticket", http.StatusInternalServerError)
		return
	}

	// Check if user has access to the ticket
	if userInfo.Role != "admin" && ticket.CreatedBy != userInfo.ID && (ticket.AssignedTo == nil || *ticket.AssignedTo != userInfo.ID) {
		http.Error(w, "You don't have permission to download this attachment", http.StatusForbidden)
		return
	}

	// Open the file
	file, err := os.Open(attachment.FilePath)
	if err != nil {
		utils.HandleError(w, err, "Error opening file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set response headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", attachment.FileName))
	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(attachment.FileSize, 10))

	// Stream the file to the response
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("Error streaming file: %v", err)
	}
}

// TicketAPIHandler handles API requests for tickets
func (h *TicketHandler) TicketAPIHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Parse action from URL
	action := r.URL.Path[len("/tickets/api/"):]

	switch action {
	case "subscribe":
		// Subscribe to a ticket
		ticketID, err := strconv.Atoi(r.FormValue("ticket_id"))
		if err != nil {
			utils.JSONError(w, "Invalid ticket ID", http.StatusBadRequest)
			return
		}

		subscribed := r.FormValue("subscribed") == "true"
		if err := dbTicket.SubscribeToTicket(h.DB, ticketID, userInfo.ID, subscribed); err != nil {
			utils.JSONError(w, "Error updating subscription", http.StatusInternalServerError)
			return
		}

		response := ticketModels.APIResponse{
			Success: true,
			Message: "Subscription updated",
		}
		json.NewEncoder(w).Encode(response)

	default:
		utils.JSONError(w, "Unknown API action", http.StatusBadRequest)
	}
}

// saveAttachment saves a file attachment and creates a record in the database
func (h *TicketHandler) saveAttachment(ticketID int, commentID int, userID int, fileHeader *multipart.FileHeader) error {
	// Open the uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	// Create attachments directory if it doesn't exist
	if err := os.MkdirAll(config.AttachmentStoragePath, os.ModePerm); err != nil {
		return err
	}

	// Generate unique filename
	timestamp := time.Now().Unix()
	filename := filepath.Base(fileHeader.Filename)
	safeName := fmt.Sprintf("%d_%d_%s", ticketID, timestamp, strings.ReplaceAll(filename, " ", "_"))
	filePath := filepath.Join(config.AttachmentStoragePath, safeName)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy the file
	if _, err = io.Copy(dst, file); err != nil {
		return err
	}

	// Create attachment record
	var commentIDPtr *int
	if commentID > 0 {
		commentIDPtr = &commentID
	}

	attachment := ticketModels.TicketAttachment{
		TicketID:    ticketID,
		CommentID:   commentIDPtr,
		FileName:    filename,
		FilePath:    filePath,
		FileSize:    fileHeader.Size,
		ContentType: fileHeader.Header.Get("Content-Type"),
		UploadedBy:  userID,
		UploadedAt:  time.Now(),
	}

	return h.DB.Create(&attachment).Error
}

// renderTemplate renders a template with standard data structure
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) {
	err := tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		utils.HandleError(w, err, "Template rendering error", http.StatusInternalServerError)
	}
}

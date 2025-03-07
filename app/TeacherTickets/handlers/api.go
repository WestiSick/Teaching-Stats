package handlers

import (
	"TeacherJournal/config"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"TeacherJournal/app/TeacherTickets/models"
	"TeacherJournal/app/dashboard/db"
)

// APIHandler handles JSON API requests for tickets
type APIHandler struct {
	DB *sql.DB
}

// NewAPIHandler creates a new APIHandler
func NewAPIHandler(database *sql.DB) *APIHandler {
	return &APIHandler{
		DB: database,
	}
}

// APITicketsHandler handles GET and POST operations for multiple tickets
func (h *APIHandler) APITicketsHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Handle request based on method
	switch r.Method {
	case "GET":
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
			sendJSONError(w, "Error retrieving tickets", http.StatusInternalServerError)
			return
		}

		// Return tickets as JSON
		json.NewEncoder(w).Encode(paginatedTickets)

	case "POST":
		// Parse JSON request
		var ticket models.Ticket
		err := json.NewDecoder(r.Body).Decode(&ticket)
		if err != nil {
			sendJSONError(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Set creator ID and initial status
		ticket.CreatorID = dbUserInfo.ID
		ticket.Status = "New"

		// Save ticket to database
		err = db.CreateTicket(h.DB, &ticket)
		if err != nil {
			sendJSONError(w, "Error creating ticket", http.StatusInternalServerError)
			return
		}

		// Return created ticket
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ticket)

	default:
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// APITicketHandler handles GET, PUT, and DELETE operations for a single ticket
func (h *APIHandler) APITicketHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get ticket ID from URL
	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["id"])
	if err != nil {
		sendJSONError(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Handle request based on method
	switch r.Method {
	case "GET":
		// Get ticket details
		ticket, err := db.GetTicketDetails(h.DB, ticketID)
		if err != nil {
			if err == sql.ErrNoRows {
				sendJSONError(w, "Ticket not found", http.StatusNotFound)
			} else {
				sendJSONError(w, "Error retrieving ticket", http.StatusInternalServerError)
			}
			return
		}

		// Check if user has access to this ticket
		isAdmin := dbUserInfo.Role == "admin"
		if !isAdmin && ticket.CreatorID != dbUserInfo.ID &&
			(ticket.AssigneeID == nil || *ticket.AssigneeID != dbUserInfo.ID) {
			sendJSONError(w, "Access denied", http.StatusForbidden)
			return
		}

		// Return ticket as JSON
		json.NewEncoder(w).Encode(ticket)

	case "PUT":
		// Get existing ticket
		ticket, err := db.GetTicket(h.DB, ticketID)
		if err != nil {
			if err == sql.ErrNoRows {
				sendJSONError(w, "Ticket not found", http.StatusNotFound)
			} else {
				sendJSONError(w, "Error retrieving ticket", http.StatusInternalServerError)
			}
			return
		}

		// Check if user has permission
		isAdmin := dbUserInfo.Role == "admin"
		if !isAdmin && ticket.CreatorID != dbUserInfo.ID &&
			(ticket.AssigneeID == nil || *ticket.AssigneeID != dbUserInfo.ID) {
			sendJSONError(w, "Access denied", http.StatusForbidden)
			return
		}

		// Parse update data
		var updates models.Ticket
		err = json.NewDecoder(r.Body).Decode(&updates)
		if err != nil {
			sendJSONError(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Update fields
		if updates.Title != "" {
			ticket.Title = updates.Title
		}
		if updates.Description != "" {
			ticket.Description = updates.Description
		}
		if updates.Priority != "" {
			ticket.Priority = updates.Priority
		}
		if updates.Category != "" {
			ticket.Category = updates.Category
		}

		// Only admins or assignees can update status
		if isAdmin || (ticket.AssigneeID != nil && *ticket.AssigneeID == dbUserInfo.ID) {
			if updates.Status != "" && updates.Status != ticket.Status {
				// Update the status
				ticket.Status = updates.Status
				// Log status change
				db.UpdateTicketStatus(h.DB, ticketID, updates.Status, dbUserInfo.ID)
			}
		}

		// Only admins can update assignee
		if isAdmin && updates.AssigneeID != nil {
			ticket.AssigneeID = updates.AssigneeID
		}

		// Save changes
		err = db.UpdateTicket(h.DB, ticket)
		if err != nil {
			sendJSONError(w, "Error updating ticket", http.StatusInternalServerError)
			return
		}

		// Return updated ticket
		json.NewEncoder(w).Encode(ticket)

	case "DELETE":
		// Only admins can delete tickets
		if dbUserInfo.Role != "admin" {
			sendJSONError(w, "Access denied", http.StatusForbidden)
			return
		}

		// Delete ticket
		err := db.DeleteTicket(h.DB, ticketID)
		if err != nil {
			sendJSONError(w, "Error deleting ticket", http.StatusInternalServerError)
			return
		}

		// Return success message
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Ticket deleted successfully"})

	default:
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// APITicketStatusHandler handles status updates for a ticket
func (h *APIHandler) APITicketStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow PUT method
	if r.Method != "PUT" {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get ticket ID from URL
	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["id"])
	if err != nil {
		sendJSONError(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Get existing ticket
	ticket, err := db.GetTicket(h.DB, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "Ticket not found", http.StatusNotFound)
		} else {
			sendJSONError(w, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Check if user has permission to update status
	isAdmin := dbUserInfo.Role == "admin"
	isAssignee := ticket.AssigneeID != nil && *ticket.AssigneeID == dbUserInfo.ID
	if !isAdmin && !isAssignee {
		sendJSONError(w, "Access denied", http.StatusForbidden)
		return
	}

	// Parse status update
	var statusUpdate struct {
		Status string `json:"status"`
	}

	err = json.NewDecoder(r.Body).Decode(&statusUpdate)
	if err != nil || statusUpdate.Status == "" {
		sendJSONError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Check if status is valid
	isValidStatus := false
	for _, status := range config.TicketStatusValues {
		if status == statusUpdate.Status {
			isValidStatus = true
			break
		}
	}

	if !isValidStatus {
		sendJSONError(w, "Invalid status value", http.StatusBadRequest)
		return
	}

	// Store current status for response
	currentStatus := ticket.Status

	// Update ticket status
	ticket.Status = statusUpdate.Status

	// Save changes
	err = db.UpdateTicket(h.DB, ticket)
	if err != nil {
		sendJSONError(w, "Error updating ticket status", http.StatusInternalServerError)
		return
	}

	// Log status change
	db.UpdateTicketStatus(h.DB, ticketID, statusUpdate.Status, dbUserInfo.ID)

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Return success message
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":    "Status updated successfully",
		"old_status": currentStatus,
		"new_status": statusUpdate.Status,
	})
}

// APITicketCommentsHandler handles GET and POST operations for ticket comments
func (h *APIHandler) APITicketCommentsHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get ticket ID from URL
	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["id"])
	if err != nil {
		sendJSONError(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Check if ticket exists and if user has access
	ticket, err := db.GetTicket(h.DB, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "Ticket not found", http.StatusNotFound)
		} else {
			sendJSONError(w, "Error retrieving ticket", http.StatusInternalServerError)
		}
		return
	}

	// Check if user has access to this ticket
	isAdmin := dbUserInfo.Role == "admin"
	if !isAdmin && ticket.CreatorID != dbUserInfo.ID &&
		(ticket.AssigneeID == nil || *ticket.AssigneeID != dbUserInfo.ID) {
		sendJSONError(w, "Access denied", http.StatusForbidden)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Handle request based on method
	switch r.Method {
	case "GET":
		// Get comments for the ticket
		comments, err := db.GetTicketComments(h.DB, ticketID)
		if err != nil {
			sendJSONError(w, "Error retrieving comments", http.StatusInternalServerError)
			return
		}

		// Return comments as JSON
		json.NewEncoder(w).Encode(comments)

	case "POST":
		// Parse comment data
		var comment models.TicketComment
		err := json.NewDecoder(r.Body).Decode(&comment)
		if err != nil || comment.Comment == "" {
			sendJSONError(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Set ticket ID and user ID
		comment.TicketID = ticketID
		comment.UserID = dbUserInfo.ID

		// Save comment
		err = db.AddTicketComment(h.DB, &comment)
		if err != nil {
			sendJSONError(w, "Error adding comment", http.StatusInternalServerError)
			return
		}

		// Retrieve comment with user info
		fullComment, err := db.GetTicketComment(h.DB, comment.ID)
		if err != nil {
			// If we can't retrieve the full comment, just return the basic one
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(comment)
			return
		}

		// Return created comment
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(fullComment)

	default:
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// APITicketCommentHandler handles GET and DELETE operations for a single comment
func (h *APIHandler) APITicketCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get ticket and comment IDs from URL
	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["ticket_id"])
	if err != nil {
		sendJSONError(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	commentID, err := strconv.Atoi(vars["comment_id"])
	if err != nil {
		sendJSONError(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Handle request based on method
	switch r.Method {
	case "GET":
		// Get the comment
		comment, err := db.GetTicketComment(h.DB, commentID)
		if err != nil {
			if err == sql.ErrNoRows {
				sendJSONError(w, "Comment not found", http.StatusNotFound)
			} else {
				sendJSONError(w, "Error retrieving comment", http.StatusInternalServerError)
			}
			return
		}

		// Check if comment belongs to the specified ticket
		if comment.TicketID != ticketID {
			sendJSONError(w, "Comment does not belong to this ticket", http.StatusBadRequest)
			return
		}

		// Check if user has access to this ticket
		ticket, err := db.GetTicket(h.DB, ticketID)
		if err != nil {
			sendJSONError(w, "Error retrieving ticket", http.StatusInternalServerError)
			return
		}

		isAdmin := dbUserInfo.Role == "admin"
		if !isAdmin && ticket.CreatorID != dbUserInfo.ID &&
			(ticket.AssigneeID == nil || *ticket.AssigneeID != dbUserInfo.ID) {
			sendJSONError(w, "Access denied", http.StatusForbidden)
			return
		}

		// Return comment as JSON
		json.NewEncoder(w).Encode(comment)

	case "DELETE":
		// Get the comment
		comment, err := db.GetTicketComment(h.DB, commentID)
		if err != nil {
			if err == sql.ErrNoRows {
				sendJSONError(w, "Comment not found", http.StatusNotFound)
			} else {
				sendJSONError(w, "Error retrieving comment", http.StatusInternalServerError)
			}
			return
		}

		// Check if comment belongs to the specified ticket
		if comment.TicketID != ticketID {
			sendJSONError(w, "Comment does not belong to this ticket", http.StatusBadRequest)
			return
		}

		// Check if user has permission to delete this comment
		isAdmin := dbUserInfo.Role == "admin"
		if !isAdmin && comment.UserID != dbUserInfo.ID {
			sendJSONError(w, "Access denied", http.StatusForbidden)
			return
		}

		// Delete the comment
		err = db.DeleteTicketComment(h.DB, commentID)
		if err != nil {
			sendJSONError(w, "Error deleting comment", http.StatusInternalServerError)
			return
		}

		// Return success message
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Comment deleted successfully"})

	default:
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// APINotificationSettingsHandler handles GET and PUT operations for notification settings
func (h *APIHandler) APINotificationSettingsHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info
	dbUserInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Handle request based on method
	switch r.Method {
	case "GET":
		// Get notification settings for the user
		settings, err := db.GetNotificationSettings(h.DB, dbUserInfo.ID)
		if err != nil {
			if err == sql.ErrNoRows {
				// If no settings exist, return default settings
				settings = &models.NotificationSettings{
					UserID:              dbUserInfo.ID,
					NotifyNewTicket:     true,
					NotifyTicketUpdate:  true,
					NotifyTicketComment: true,
					NotifyTicketStatus:  true,
				}
				// Save default settings
				db.SaveNotificationSettings(h.DB, settings)
			} else {
				sendJSONError(w, "Error retrieving notification settings", http.StatusInternalServerError)
				return
			}
		}

		// Return settings as JSON
		json.NewEncoder(w).Encode(settings)

	case "PUT":
		// Parse settings data
		var settings models.NotificationSettings
		err := json.NewDecoder(r.Body).Decode(&settings)
		if err != nil {
			sendJSONError(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Set user ID
		settings.UserID = dbUserInfo.ID

		// Save settings
		err = db.SaveNotificationSettings(h.DB, &settings)
		if err != nil {
			sendJSONError(w, "Error saving notification settings", http.StatusInternalServerError)
			return
		}

		// Return updated settings
		json.NewEncoder(w).Encode(settings)

	default:
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Helper function to send JSON error responses
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

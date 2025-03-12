package handlers

import (
	"TeacherJournal/app/calendar/db"
	"TeacherJournal/app/calendar/models"
	dashboardDB "TeacherJournal/app/dashboard/db"
	"TeacherJournal/config"
	"encoding/json"
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
	"gorm.io/gorm"
)

// CalendarHandler handles calendar-related routes
type CalendarHandler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

// NewCalendarHandler creates a new CalendarHandler
func NewCalendarHandler(database *gorm.DB, tmpl *template.Template) *CalendarHandler {
	return &CalendarHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// CalendarViewData contains data for the calendar view
type CalendarViewData struct {
	User      dashboardDB.UserInfo
	Events    []models.Event
	StartDate time.Time
	EndDate   time.Time
	View      string // "day", "week", "month"
}

// EventData contains data for an event
type EventData struct {
	Event        models.Event
	Participants []models.EventParticipant
	Attachments  []models.EventAttachment
	AllUsers     []map[string]interface{}
	User         dashboardDB.UserInfo
}

// IndexHandler shows the calendar main view
func (h *CalendarHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Parse date and view parameters
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "month" // Default view
	}

	dateStr := r.URL.Query().Get("date")
	var currentDate time.Time
	if dateStr == "" {
		currentDate = time.Now()
	} else {
		currentDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			currentDate = time.Now()
		}
	}

	var startDate, endDate time.Time
	switch view {
	case "day":
		startDate = time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 0, 0, 0, 0, time.UTC)
		endDate = startDate.AddDate(0, 0, 1)
	case "week":
		// Get the beginning of the week (Monday)
		daysSinceMonday := int(currentDate.Weekday())
		if daysSinceMonday == 0 {
			daysSinceMonday = 7 // Sunday is 0, make it 7 for our calculation
		}
		startDate = time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day()-daysSinceMonday+1, 0, 0, 0, 0, time.UTC)
		endDate = startDate.AddDate(0, 0, 7)
	default: // month view
		startDate = time.Date(currentDate.Year(), currentDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		endDate = startDate.AddDate(0, 1, 0)
	}

	// Get events for the date range
	events, err := db.GetEventsByDateRange(h.DB, userInfo.ID, startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to get events", http.StatusInternalServerError)
		return
	}

	data := CalendarViewData{
		User:      userInfo,
		Events:    events,
		StartDate: startDate,
		EndDate:   endDate,
		View:      view,
	}

	h.renderTemplate(w, "calendar.html", data)
}

// GetEventsJSON returns events as JSON for AJAX requests
func (h *CalendarHandler) GetEventsJSON(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var startDate, endDate time.Time
	if startStr != "" {
		startDate, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			startDate = time.Now().AddDate(0, -1, 0) // Default to 1 month ago
		}
	} else {
		startDate = time.Now().AddDate(0, -1, 0) // Default to 1 month ago
	}

	if endStr != "" {
		endDate, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			endDate = time.Now().AddDate(0, 1, 0) // Default to 1 month ahead
		}
	} else {
		endDate = time.Now().AddDate(0, 1, 0) // Default to 1 month ahead
	}

	events, err := db.GetEventsByDateRange(h.DB, userInfo.ID, startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to get events", http.StatusInternalServerError)
		return
	}

	// Transform events to a format suitable for fullcalendar
	type EventJSON struct {
		ID          int       `json:"id"`
		Title       string    `json:"title"`
		Start       time.Time `json:"start"`
		End         time.Time `json:"end"`
		Description string    `json:"description"`
		Location    string    `json:"location"`
		CreatorID   int       `json:"creatorId"`
	}

	var eventsJSON []EventJSON
	for _, event := range events {
		eventsJSON = append(eventsJSON, EventJSON{
			ID:          event.ID,
			Title:       event.Title,
			Start:       event.StartTime,
			End:         event.EndTime,
			Description: event.Description,
			Location:    event.Location,
			CreatorID:   event.CreatorID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eventsJSON)
}

// CreateEventHandler handles event creation
func (h *CalendarHandler) CreateEventHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if r.Method == "GET" {
		users, err := db.GetAllActiveUsers(h.DB)
		if err != nil {
			http.Error(w, "Failed to get users", http.StatusInternalServerError)
			return
		}

		// Convert users to map for template
		var allUsers []map[string]interface{}
		for _, user := range users {
			allUsers = append(allUsers, map[string]interface{}{
				"ID":   user.ID,
				"FIO":  user.FIO,
				"Role": user.Role,
			})
		}

		data := EventData{
			User:     userInfo,
			AllUsers: allUsers,
		}

		h.renderTemplate(w, "create_event.html", data)
		return
	}

	// Process form submission
	err = r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	location := r.FormValue("location")
	startTimeStr := r.FormValue("start_time")
	endTimeStr := r.FormValue("end_time")

	if title == "" || startTimeStr == "" || endTimeStr == "" {
		http.Error(w, "Title, start time, and end time are required", http.StatusBadRequest)
		return
	}

	startTime, err := time.Parse("2006-01-02T15:04", startTimeStr)
	if err != nil {
		http.Error(w, "Invalid start time format", http.StatusBadRequest)
		return
	}

	endTime, err := time.Parse("2006-01-02T15:04", endTimeStr)
	if err != nil {
		http.Error(w, "Invalid end time format", http.StatusBadRequest)
		return
	}

	// Create the event
	event := models.Event{
		CreatorID:   userInfo.ID,
		Title:       title,
		Description: description,
		Location:    location,
		StartTime:   startTime,
		EndTime:     endTime,
	}

	err = db.CreateEvent(h.DB, &event)
	if err != nil {
		http.Error(w, "Failed to create event", http.StatusInternalServerError)
		return
	}

	// Add participants
	participants := r.Form["participants"]
	for _, participantID := range participants {
		userID, err := strconv.Atoi(participantID)
		if err != nil {
			continue
		}
		db.AddEventParticipant(h.DB, event.ID, userID)
	}

	// Handle file uploads
	files := r.MultipartForm.File["attachments"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		// Create directory if it doesn't exist
		uploadDir := filepath.Join("uploads", "calendar", fmt.Sprintf("event_%d", event.ID))
		os.MkdirAll(uploadDir, 0755)

		// Create file on disk
		dst, err := os.Create(filepath.Join(uploadDir, fileHeader.Filename))
		if err != nil {
			continue
		}
		defer dst.Close()

		// Copy uploaded file to destination file
		if _, err := io.Copy(dst, file); err != nil {
			continue
		}

		// Save attachment in database
		attachment := models.EventAttachment{
			EventID:    event.ID,
			FileName:   fileHeader.Filename,
			FilePath:   filepath.Join(uploadDir, fileHeader.Filename),
			FileSize:   fileHeader.Size,
			UploadedBy: userInfo.ID,
		}
		db.AddEventAttachment(h.DB, &attachment)
	}

	http.Redirect(w, r, "/calendar", http.StatusSeeOther)
}

// ViewEventHandler shows event details
func (h *CalendarHandler) ViewEventHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	eventID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}

	event, err := db.GetEventByID(h.DB, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	participants, err := db.GetEventParticipants(h.DB, eventID)
	if err != nil {
		http.Error(w, "Failed to get participants", http.StatusInternalServerError)
		return
	}

	attachments, err := db.GetEventAttachments(h.DB, eventID)
	if err != nil {
		http.Error(w, "Failed to get attachments", http.StatusInternalServerError)
		return
	}

	data := EventData{
		Event:        *event,
		Participants: participants,
		Attachments:  attachments,
		User:         userInfo,
	}

	h.renderTemplate(w, "view_event.html", data)
}

// EditEventHandler handles event editing
func (h *CalendarHandler) EditEventHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	eventID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}

	event, err := db.GetEventByID(h.DB, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Check if user is the creator
	if event.CreatorID != userInfo.ID {
		http.Error(w, "You don't have permission to edit this event", http.StatusForbidden)
		return
	}

	if r.Method == "GET" {
		participants, err := db.GetEventParticipants(h.DB, eventID)
		if err != nil {
			http.Error(w, "Failed to get participants", http.StatusInternalServerError)
			return
		}

		attachments, err := db.GetEventAttachments(h.DB, eventID)
		if err != nil {
			http.Error(w, "Failed to get attachments", http.StatusInternalServerError)
			return
		}

		users, err := db.GetAllActiveUsers(h.DB)
		if err != nil {
			http.Error(w, "Failed to get users", http.StatusInternalServerError)
			return
		}

		// Convert users to map for template
		var allUsers []map[string]interface{}
		for _, user := range users {
			allUsers = append(allUsers, map[string]interface{}{
				"ID":   user.ID,
				"FIO":  user.FIO,
				"Role": user.Role,
			})
		}

		data := EventData{
			Event:        *event,
			Participants: participants,
			Attachments:  attachments,
			AllUsers:     allUsers,
			User:         userInfo,
		}

		h.renderTemplate(w, "edit_event.html", data)
		return
	}

	// Process form submission
	err = r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	location := r.FormValue("location")
	startTimeStr := r.FormValue("start_time")
	endTimeStr := r.FormValue("end_time")

	if title == "" || startTimeStr == "" || endTimeStr == "" {
		http.Error(w, "Title, start time, and end time are required", http.StatusBadRequest)
		return
	}

	startTime, err := time.Parse("2006-01-02T15:04", startTimeStr)
	if err != nil {
		http.Error(w, "Invalid start time format", http.StatusBadRequest)
		return
	}

	endTime, err := time.Parse("2006-01-02T15:04", endTimeStr)
	if err != nil {
		http.Error(w, "Invalid end time format", http.StatusBadRequest)
		return
	}

	// Update the event
	event.Title = title
	event.Description = description
	event.Location = location
	event.StartTime = startTime
	event.EndTime = endTime

	err = db.UpdateEvent(h.DB, event)
	if err != nil {
		http.Error(w, "Failed to update event", http.StatusInternalServerError)
		return
	}

	// Handle file uploads
	files := r.MultipartForm.File["attachments"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		// Create directory if it doesn't exist
		uploadDir := filepath.Join("uploads", "calendar", fmt.Sprintf("event_%d", event.ID))
		os.MkdirAll(uploadDir, 0755)

		// Create file on disk
		dst, err := os.Create(filepath.Join(uploadDir, fileHeader.Filename))
		if err != nil {
			continue
		}
		defer dst.Close()

		// Copy uploaded file to destination file
		if _, err := io.Copy(dst, file); err != nil {
			continue
		}

		// Save attachment in database
		attachment := models.EventAttachment{
			EventID:    event.ID,
			FileName:   fileHeader.Filename,
			FilePath:   filepath.Join(uploadDir, fileHeader.Filename),
			FileSize:   fileHeader.Size,
			UploadedBy: userInfo.ID,
		}
		db.AddEventAttachment(h.DB, &attachment)
	}

	http.Redirect(w, r, fmt.Sprintf("/calendar/event/%d", event.ID), http.StatusSeeOther)
}

// DeleteEventHandler handles event deletion
func (h *CalendarHandler) DeleteEventHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	eventID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}

	err = db.DeleteEvent(h.DB, eventID, userInfo.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/calendar", http.StatusSeeOther)
}

// DownloadAttachmentHandler handles attachment downloads
func (h *CalendarHandler) DownloadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid attachment ID", http.StatusBadRequest)
		return
	}

	var attachment models.EventAttachment
	err = h.DB.First(&attachment, attachmentID).Error
	if err != nil {
		http.Error(w, "Attachment not found", http.StatusNotFound)
		return
	}

	// Open the file
	file, err := os.Open(attachment.FilePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Set the appropriate headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", attachment.FileName))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Copy the file to the response
	io.Copy(w, file)
}

// DeleteAttachmentHandler handles attachment deletion
func (h *CalendarHandler) DeleteAttachmentHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

	var attachment models.EventAttachment
	err = h.DB.First(&attachment, attachmentID).Error
	if err != nil {
		http.Error(w, "Attachment not found", http.StatusNotFound)
		return
	}

	eventID := attachment.EventID

	err = db.DeleteEventAttachment(h.DB, attachmentID, userInfo.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Try to delete the file
	os.Remove(attachment.FilePath)

	http.Redirect(w, r, fmt.Sprintf("/calendar/event/%d", eventID), http.StatusSeeOther)
}

// renderTemplate renders a template with standard data structure
func (h *CalendarHandler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	err := h.Tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Template rendering error: %v", err)
		http.Error(w, fmt.Sprintf("Template rendering error: %v", err), http.StatusInternalServerError)
	}
}

package handlers

import (
	"TeacherJournal/app/calendar/db"
	"TeacherJournal/app/calendar/models"
	dashboardDB "TeacherJournal/app/dashboard/db"
	dashboardModels "TeacherJournal/app/dashboard/models"
	"TeacherJournal/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// AdminIndexHandler показывает главную страницу админ-панели календаря
func (h *CalendarHandler) AdminIndexHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Получаем всех пользователей
	var users []dashboardModels.User
	h.DB.Model(&dashboardModels.User{}).Select("id, fio, role").Find(&users)

	// Получаем статистику по событиям
	var eventCount int64
	h.DB.Model(&models.Event{}).Count(&eventCount)

	data := struct {
		User       dashboardDB.UserInfo
		Users      []dashboardModels.User
		EventCount int64
	}{
		User:       userInfo,
		Users:      users,
		EventCount: eventCount,
	}

	h.renderTemplate(w, "admin_index.html", data)
}

// AdminUsersHandler показывает список пользователей для выбора
func (h *CalendarHandler) AdminUsersHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Получаем всех пользователей
	var users []dashboardModels.User
	h.DB.Model(&dashboardModels.User{}).Select("id, fio, role").Find(&users)

	// Получаем количество событий для каждого пользователя
	userEvents := make(map[int]int64)
	for _, user := range users {
		var count int64
		h.DB.Model(&models.Event{}).Where("creator_id = ?", user.ID).Count(&count)
		userEvents[user.ID] = count
	}

	data := struct {
		User       dashboardDB.UserInfo
		Users      []dashboardModels.User
		UserEvents map[int]int64
	}{
		User:       userInfo,
		Users:      users,
		UserEvents: userEvents,
	}

	h.renderTemplate(w, "admin_users.html", data)
}

// AdminUserCalendarHandler показывает календарь выбранного пользователя
func (h *CalendarHandler) AdminUserCalendarHandler(w http.ResponseWriter, r *http.Request) {
	adminInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Получаем информацию о выбранном пользователе
	var selectedUser dashboardModels.User
	if err := h.DB.First(&selectedUser, userID).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
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
	events, err := db.GetEventsByDateRange(h.DB, userID, startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to get events", http.StatusInternalServerError)
		return
	}

	data := struct {
		User         dashboardDB.UserInfo
		SelectedUser dashboardModels.User
		Events       []models.Event
		StartDate    time.Time
		EndDate      time.Time
		View         string
	}{
		User:         adminInfo,
		SelectedUser: selectedUser,
		Events:       events,
		StartDate:    startDate,
		EndDate:      endDate,
		View:         view,
	}

	h.renderTemplate(w, "admin_user_calendar.html", data)
}

// AdminViewEventHandler показывает детали события для администратора
func (h *CalendarHandler) AdminViewEventHandler(w http.ResponseWriter, r *http.Request) {
	adminInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

	// Получаем информацию о создателе
	var creator dashboardModels.User
	if err := h.DB.First(&creator, event.CreatorID).Error; err != nil {
		http.Error(w, "Creator not found", http.StatusInternalServerError)
		return
	}

	data := struct {
		User         dashboardDB.UserInfo
		Event        models.Event
		Creator      dashboardModels.User
		Participants []models.EventParticipant
		Attachments  []models.EventAttachment
	}{
		User:         adminInfo,
		Event:        *event,
		Creator:      creator,
		Participants: participants,
		Attachments:  attachments,
	}

	h.renderTemplate(w, "admin_view_event.html", data)
}

// AdminEditEventHandler обрабатывает редактирование события администратором
func (h *CalendarHandler) AdminEditEventHandler(w http.ResponseWriter, r *http.Request) {
	adminInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

		data := struct {
			User         dashboardDB.UserInfo
			Event        models.Event
			Participants []models.EventParticipant
			Attachments  []models.EventAttachment
			AllUsers     []map[string]interface{}
		}{
			User:         adminInfo,
			Event:        *event,
			Participants: participants,
			Attachments:  attachments,
			AllUsers:     allUsers,
		}

		h.renderTemplate(w, "admin_edit_event.html", data)
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
			UploadedBy: adminInfo.ID,
		}
		db.AddEventAttachment(h.DB, &attachment)
	}

	// Log admin action
	dashboardDB.LogAction(h.DB, adminInfo.ID, "Admin Event Edit", fmt.Sprintf("Admin edited event: %s (ID: %d)", event.Title, event.ID))

	http.Redirect(w, r, fmt.Sprintf("/admin/calendar/event/%d", event.ID), http.StatusSeeOther)
}

// AdminDeleteEventHandler обрабатывает удаление события администратором
func (h *CalendarHandler) AdminDeleteEventHandler(w http.ResponseWriter, r *http.Request) {
	adminInfo, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

	// Получаем информацию о событии перед удалением
	event, err := db.GetEventByID(h.DB, eventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	tx := h.DB.Begin()
	if tx.Error != nil {
		http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
		return
	}

	// Удаляем вложения
	if err := tx.Delete(&models.EventAttachment{}, "event_id = ?", eventID).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete attachments", http.StatusInternalServerError)
		return
	}

	// Удаляем участников
	if err := tx.Delete(&models.EventParticipant{}, "event_id = ?", eventID).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete participants", http.StatusInternalServerError)
		return
	}

	// Удаляем событие
	if err := tx.Delete(&models.Event{}, "id = ?", eventID).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete event", http.StatusInternalServerError)
		return
	}

	// Логируем действие администратора
	dashboardDB.LogAction(tx, adminInfo.ID, "Admin Event Deletion", fmt.Sprintf("Admin deleted event: %s (ID: %d, Creator: %d)", event.Title, event.ID, event.CreatorID))

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// Перенаправляем на страницу календаря пользователя
	http.Redirect(w, r, fmt.Sprintf("/admin/calendar/user/%d", event.CreatorID), http.StatusSeeOther)
}

// GetAdminEventsJSON возвращает события пользователя в формате JSON
func (h *CalendarHandler) GetAdminEventsJSON(w http.ResponseWriter, r *http.Request) {
	_, err := dashboardDB.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
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

	events, err := db.GetEventsByDateRange(h.DB, userID, startDate, endDate)
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

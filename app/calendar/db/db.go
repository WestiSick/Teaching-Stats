package db

import (
	"TeacherJournal/app/calendar/models"
	"TeacherJournal/app/dashboard/db"
	dashboardModels "TeacherJournal/app/dashboard/models"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// InitCalendarDB migrates the calendar-related tables
func InitCalendarDB(database *gorm.DB) error {
	err := database.AutoMigrate(
		&models.Event{},
		&models.EventParticipant{},
		&models.EventAttachment{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate calendar tables: %w", err)
	}

	return nil
}

// GetEventsByUserID retrieves all events where the user is a creator or participant
func GetEventsByUserID(database *gorm.DB, userID int) ([]models.Event, error) {
	var events []models.Event

	// Get events created by the user
	creatorEvents := database.Where("creator_id = ?", userID)

	// Get events where the user is a participant
	participantEvents := database.Where("id IN (?)",
		database.Table("event_participants").
			Select("event_id").
			Where("user_id = ?", userID))

	// Combine the queries
	err := database.Where(creatorEvents).Or(participantEvents).Find(&events).Error

	if err != nil {
		return nil, err
	}

	return events, nil
}

// GetEventsByDateRange retrieves events within a specific date range for a user
func GetEventsByDateRange(database *gorm.DB, userID int, startDate, endDate time.Time) ([]models.Event, error) {
	var events []models.Event

	// Create a subquery to find all event IDs where the user is either the creator or a participant
	subQuery := database.Table("events").
		Select("events.id").
		Where("events.creator_id = ?", userID).
		Or("events.id IN (?)",
			database.Table("event_participants").
				Select("event_id").
				Where("user_id = ?", userID))

	// Get all events within the date range
	query := database.Where("id IN (?)", subQuery).
		Where("(start_time <= ? AND end_time >= ?) OR "+
			"(start_time BETWEEN ? AND ?) OR "+
			"(end_time BETWEEN ? AND ?)",
			endDate, startDate,
			startDate, endDate,
			startDate, endDate)

	// Execute the query
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}

	// Add preloading to ensure related data is available
	if len(events) > 0 {
		if err := database.Preload("Creator").Find(&events, "id IN ?", getEventIDs(events)).Error; err != nil {
			return nil, err
		}
	}

	return events, nil
}

// Helper function to extract IDs from events
func getEventIDs(events []models.Event) []int {
	ids := make([]int, len(events))
	for i, event := range events {
		ids[i] = event.ID
	}
	return ids
}

// CreateEvent creates a new calendar event
func CreateEvent(database *gorm.DB, event *models.Event) error {
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Create(event).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Log the action
	db.LogAction(tx, event.CreatorID, "Event Creation", fmt.Sprintf("Created event: %s", event.Title))

	return tx.Commit().Error
}

// UpdateEvent updates an existing event
func UpdateEvent(database *gorm.DB, event *models.Event) error {
	return database.Save(event).Error
}

// DeleteEvent deletes an event and its related data
func DeleteEvent(database *gorm.DB, eventID int, userID int) error {
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Check if the user is the creator of the event
	var event models.Event
	if err := tx.First(&event, "id = ? AND creator_id = ?", eventID, userID).Error; err != nil {
		tx.Rollback()
		return errors.New("event not found or you don't have permission to delete it")
	}

	// Delete event attachments
	if err := tx.Delete(&models.EventAttachment{}, "event_id = ?", eventID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete event participants
	if err := tx.Delete(&models.EventParticipant{}, "event_id = ?", eventID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete the event
	if err := tx.Delete(&models.Event{}, "id = ?", eventID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Log the action
	db.LogAction(tx, userID, "Event Deletion", fmt.Sprintf("Deleted event: %s", event.Title))

	return tx.Commit().Error
}

// GetEventByID retrieves an event by its ID
func GetEventByID(database *gorm.DB, eventID int) (*models.Event, error) {
	var event models.Event
	err := database.Preload("Creator").First(&event, eventID).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetEventParticipants retrieves all participants for an event
func GetEventParticipants(database *gorm.DB, eventID int) ([]models.EventParticipant, error) {
	var participants []models.EventParticipant
	err := database.Where("event_id = ?", eventID).
		Preload("User").
		Find(&participants).Error
	if err != nil {
		return nil, err
	}
	return participants, nil
}

// AddEventParticipant adds a participant to an event
func AddEventParticipant(database *gorm.DB, eventID, userID int) error {
	participant := models.EventParticipant{
		EventID: eventID,
		UserID:  userID,
		Status:  "pending",
	}
	return database.Create(&participant).Error
}

// UpdateParticipantStatus updates a participant's status for an event
func UpdateParticipantStatus(database *gorm.DB, eventID, userID int, status string) error {
	return database.Model(&models.EventParticipant{}).
		Where("event_id = ? AND user_id = ?", eventID, userID).
		Update("status", status).Error
}

// GetEventAttachments retrieves all attachments for an event
func GetEventAttachments(database *gorm.DB, eventID int) ([]models.EventAttachment, error) {
	var attachments []models.EventAttachment
	err := database.Where("event_id = ?", eventID).
		Preload("User").
		Find(&attachments).Error
	if err != nil {
		return nil, err
	}
	return attachments, nil
}

// AddEventAttachment adds an attachment to an event
func AddEventAttachment(database *gorm.DB, attachment *models.EventAttachment) error {
	return database.Create(attachment).Error
}

// DeleteEventAttachment deletes an attachment
func DeleteEventAttachment(database *gorm.DB, attachmentID, userID int) error {
	var attachment models.EventAttachment
	if err := database.First(&attachment, attachmentID).Error; err != nil {
		return err
	}

	// Check if the user is the creator of the event or the uploader of the attachment
	var event models.Event
	if err := database.First(&event, attachment.EventID).Error; err != nil {
		return err
	}

	if event.CreatorID != userID && attachment.UploadedBy != userID {
		return errors.New("you don't have permission to delete this attachment")
	}

	return database.Delete(&models.EventAttachment{}, attachmentID).Error
}

// GetAllActiveUsers gets all active users for participant selection
func GetAllActiveUsers(database *gorm.DB) ([]dashboardModels.User, error) {
	var users []dashboardModels.User
	err := database.Model(&dashboardModels.User{}).
		Select("id, fio, role").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

package tickets

import (
	"TeacherJournal/app/tickets/db"
	"TeacherJournal/app/tickets/models"
	"fmt"
	"time"
)

// CreateSystemTicket creates a ticket from the main application
func CreateSystemTicket(userID int, title string, description string, category string) (int, error) {
	// Initialize database connection
	database := db.InitTicketDB()

	// Create ticket
	ticket := models.Ticket{
		Title:        title,
		Description:  description,
		Status:       "New",
		Priority:     "Medium", // Default priority
		Category:     category,
		CreatedBy:    userID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// Save ticket
	if err := db.CreateTicket(database, &ticket); err != nil {
		return 0, err
	}

	return ticket.ID, nil
}

// AddSystemComment adds a comment to a ticket from the main application
func AddSystemComment(ticketID int, userID int, content string, isInternal bool) error {
	// Initialize database connection
	database := db.InitTicketDB()

	// Create comment
	comment := models.TicketComment{
		TicketID:   ticketID,
		UserID:     userID,
		Content:    content,
		CreatedAt:  time.Now(),
		IsInternal: isInternal,
	}

	// Save comment
	return db.AddTicketComment(database, &comment)
}

// GetUserTickets gets tickets for a user
func GetUserTickets(userID int, status string) ([]models.Ticket, error) {
	// Initialize database connection
	database := db.InitTicketDB()

	// Get tickets
	return db.GetUserTickets(database, userID, status, "")
}

// GetTicketURL generates a URL to view a ticket
func GetTicketURL(ticketID int) string {
	return fmt.Sprintf("/tickets/view/%d", ticketID)
}

// MarkTicketResolved marks a ticket as resolved
func MarkTicketResolved(ticketID int, adminID int) error {
	// Initialize database connection
	database := db.InitTicketDB()

	// Update ticket status
	updates := map[string]interface{}{
		"status": "Resolved",
	}

	return db.UpdateTicket(database, ticketID, adminID, updates)
}

// AssignTicket assigns a ticket to an admin
func AssignTicket(ticketID int, adminID int, userID int) error {
	// Initialize database connection
	database := db.InitTicketDB()

	// Update ticket assignee
	updates := map[string]interface{}{
		"assigned_to": adminID,
	}

	return db.UpdateTicket(database, ticketID, userID, updates)
}

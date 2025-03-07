package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"TeacherJournal/app/TeacherTickets/models"
)

// CreateTicketTables creates the tables needed for the ticket system
func CreateTicketTables(db *sql.DB) error {
	// Create tickets table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tickets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			creator_id INTEGER NOT NULL,
			assignee_id INTEGER,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			status TEXT NOT NULL,
			priority TEXT NOT NULL,
			category TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (creator_id) REFERENCES users(id),
			FOREIGN KEY (assignee_id) REFERENCES users(id)
		);
	`)
	if err != nil {
		return err
	}

	// Create ticket_comments table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ticket_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			comment TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (ticket_id) REFERENCES tickets(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`)
	if err != nil {
		return err
	}

	// Create ticket_status_history table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ticket_status_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			changed_by INTEGER NOT NULL,
			changed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (ticket_id) REFERENCES tickets(id),
			FOREIGN KEY (changed_by) REFERENCES users(id)
		);
	`)
	if err != nil {
		return err
	}

	// Create ticket_attachments table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ticket_attachments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			file_name TEXT NOT NULL,
			file_path TEXT NOT NULL,
			uploaded_by INTEGER NOT NULL,
			uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (ticket_id) REFERENCES tickets(id),
			FOREIGN KEY (uploaded_by) REFERENCES users(id)
		);
	`)
	if err != nil {
		return err
	}

	// Create notification_settings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notification_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			notify_new_ticket BOOLEAN DEFAULT 1,
			notify_ticket_update BOOLEAN DEFAULT 1,
			notify_ticket_comment BOOLEAN DEFAULT 1,
			notify_ticket_status BOOLEAN DEFAULT 1,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(user_id)
		);
	`)
	if err != nil {
		return err
	}

	// Create indexes
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_tickets_creator ON tickets(creator_id);
		CREATE INDEX IF NOT EXISTS idx_tickets_assignee ON tickets(assignee_id);
		CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
		CREATE INDEX IF NOT EXISTS idx_tickets_category ON tickets(category); 
		CREATE INDEX IF NOT EXISTS idx_ticket_comments_ticket ON ticket_comments(ticket_id);
		CREATE INDEX IF NOT EXISTS idx_ticket_status_history_ticket ON ticket_status_history(ticket_id);
		CREATE INDEX IF NOT EXISTS idx_ticket_attachments_ticket ON ticket_attachments(ticket_id);
	`)

	return err
}

// CreateTicket creates a new ticket in the database
func CreateTicket(db *sql.DB, ticket *models.Ticket) error {
	result, err := db.Exec(`
		INSERT INTO tickets (creator_id, title, description, status, priority, category, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, ticket.CreatorID, ticket.Title, ticket.Description, ticket.Status, ticket.Priority, ticket.Category)

	if err != nil {
		return err
	}

	ticketID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	ticket.ID = int(ticketID)

	// Add initial status to history
	_, err = db.Exec(`
		INSERT INTO ticket_status_history (ticket_id, status, changed_by, changed_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, ticket.ID, ticket.Status, ticket.CreatorID)

	return err
}

// GetTicket retrieves a ticket by ID
func GetTicket(db *sql.DB, ticketID int) (*models.Ticket, error) {
	ticket := &models.Ticket{}

	err := db.QueryRow(`
		SELECT id, creator_id, assignee_id, title, description, status, priority, category, created_at, updated_at
		FROM tickets
		WHERE id = ?
	`, ticketID).Scan(
		&ticket.ID, &ticket.CreatorID, &ticket.AssigneeID, &ticket.Title, &ticket.Description,
		&ticket.Status, &ticket.Priority, &ticket.Category, &ticket.CreatedAt, &ticket.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Get creator info
	err = db.QueryRow("SELECT id, fio FROM users WHERE id = ?", ticket.CreatorID).Scan(
		&ticket.Creator.ID, &ticket.Creator.FIO,
	)
	if err != nil {
		log.Printf("Error retrieving creator info: %v", err)
	}

	// Get assignee info if assigned
	if ticket.AssigneeID != nil {
		ticket.Assignee = &models.UserInfo{}
		err = db.QueryRow("SELECT id, fio FROM users WHERE id = ?", *ticket.AssigneeID).Scan(
			&ticket.Assignee.ID, &ticket.Assignee.FIO,
		)
		if err != nil {
			log.Printf("Error retrieving assignee info: %v", err)
		}
	}

	return ticket, nil
}

// UpdateTicket updates an existing ticket
func UpdateTicket(db *sql.DB, ticket *models.Ticket) error {
	_, err := db.Exec(`
		UPDATE tickets
		SET title = ?, description = ?, status = ?, priority = ?, category = ?, assignee_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, ticket.Title, ticket.Description, ticket.Status, ticket.Priority, ticket.Category, ticket.AssigneeID, ticket.ID)

	return err
}

// UpdateTicketStatus updates the status of a ticket and adds to status history
func UpdateTicketStatus(db *sql.DB, ticketID int, status string, changedBy int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Update ticket status
	_, err = tx.Exec(`
		UPDATE tickets
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, ticketID)

	if err != nil {
		tx.Rollback()
		return err
	}

	// Add to status history
	_, err = tx.Exec(`
		INSERT INTO ticket_status_history (ticket_id, status, changed_by, changed_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, ticketID, status, changedBy)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// AddTicketComment adds a comment to a ticket
func AddTicketComment(db *sql.DB, comment *models.TicketComment) error {
	result, err := db.Exec(`
		INSERT INTO ticket_comments (ticket_id, user_id, comment, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, comment.TicketID, comment.UserID, comment.Comment)

	if err != nil {
		return err
	}

	commentID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	comment.ID = int(commentID)

	// Update the ticket's updated_at timestamp
	_, err = db.Exec(`
		UPDATE tickets
		SET updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, comment.TicketID)

	return err
}

// GetTicketComments retrieves all comments for a ticket
func GetTicketComments(db *sql.DB, ticketID int) ([]models.TicketComment, error) {
	rows, err := db.Query(`
		SELECT tc.id, tc.ticket_id, tc.user_id, tc.comment, tc.created_at, u.fio
		FROM ticket_comments tc
		JOIN users u ON tc.user_id = u.id
		WHERE tc.ticket_id = ?
		ORDER BY tc.created_at
	`, ticketID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.TicketComment
	for rows.Next() {
		var comment models.TicketComment
		var userFIO string

		err := rows.Scan(
			&comment.ID, &comment.TicketID, &comment.UserID, &comment.Comment, &comment.CreatedAt, &userFIO,
		)
		if err != nil {
			return nil, err
		}

		comment.User = &models.UserInfo{
			ID:  comment.UserID,
			FIO: userFIO,
		}

		comments = append(comments, comment)
	}

	return comments, nil
}

// GetTickets retrieves tickets based on filters
func GetTickets(db *sql.DB, userID int, isAdmin bool, filters map[string]string, page, limit int) (*models.PaginatedTickets, error) {
	// Build query with filters
	query := `
		SELECT t.id, t.creator_id, t.assignee_id, t.title, t.description, 
		       t.status, t.priority, t.category, t.created_at, t.updated_at,
		       c.fio as creator_fio, a.fio as assignee_fio
		FROM tickets t
		JOIN users c ON t.creator_id = c.id
		LEFT JOIN users a ON t.assignee_id = a.id
		WHERE 1=1
	`
	countQuery := `
		SELECT COUNT(*)
		FROM tickets t
		WHERE 1=1
	`

	var queryParams []interface{}

	// If not admin, show only tickets created by the user
	if !isAdmin {
		query += " AND t.creator_id = ?"
		countQuery += " AND t.creator_id = ?"
		queryParams = append(queryParams, userID)
	}

	// Apply filters
	if status, ok := filters["status"]; ok && status != "" {
		query += " AND t.status = ?"
		countQuery += " AND t.status = ?"
		queryParams = append(queryParams, status)
	}

	if priority, ok := filters["priority"]; ok && priority != "" {
		query += " AND t.priority = ?"
		countQuery += " AND t.priority = ?"
		queryParams = append(queryParams, priority)
	}

	if category, ok := filters["category"]; ok && category != "" {
		query += " AND t.category = ?"
		countQuery += " AND t.category = ?"
		queryParams = append(queryParams, category)
	}

	if creatorID, ok := filters["creator_id"]; ok && creatorID != "" {
		query += " AND t.creator_id = ?"
		countQuery += " AND t.creator_id = ?"
		queryParams = append(queryParams, creatorID)
	}

	if assigneeID, ok := filters["assignee_id"]; ok && assigneeID != "" {
		query += " AND t.assignee_id = ?"
		countQuery += " AND t.assignee_id = ?"
		queryParams = append(queryParams, assigneeID)
	}

	// Add order and pagination
	query += " ORDER BY t.updated_at DESC LIMIT ? OFFSET ?"

	// Calculate offset
	offset := (page - 1) * limit
	queryParamsCopy := make([]interface{}, len(queryParams))
	copy(queryParamsCopy, queryParams)

	queryParams = append(queryParams, limit, offset)

	// Get total count
	var total int
	err := db.QueryRow(countQuery, queryParamsCopy...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Execute query
	rows, err := db.Query(query, queryParams...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Process results
	var tickets []models.Ticket
	for rows.Next() {
		var ticket models.Ticket
		var assigneeID sql.NullInt64
		var assigneeFIO sql.NullString
		var creatorFIO string

		err := rows.Scan(
			&ticket.ID, &ticket.CreatorID, &assigneeID, &ticket.Title, &ticket.Description,
			&ticket.Status, &ticket.Priority, &ticket.Category, &ticket.CreatedAt, &ticket.UpdatedAt,
			&creatorFIO, &assigneeFIO,
		)
		if err != nil {
			return nil, err
		}

		// Set creator info
		ticket.Creator = &models.UserInfo{
			ID:  ticket.CreatorID,
			FIO: creatorFIO,
		}

		// Set assignee info if assigned
		if assigneeID.Valid {
			intID := int(assigneeID.Int64)
			ticket.AssigneeID = &intID
			ticket.Assignee = &models.UserInfo{
				ID:  intID,
				FIO: assigneeFIO.String,
			}
		}

		tickets = append(tickets, ticket)
	}

	// Calculate pages
	pages := (total + limit - 1) / limit

	result := &models.PaginatedTickets{
		Tickets: tickets,
		Pagination: models.Pagination{
			Total: total,
			Page:  page,
			Limit: limit,
			Pages: pages,
		},
	}

	return result, nil
}

// GetTicketDetails retrieves a ticket with all its details (comments, attachments, history)
func GetTicketDetails(db *sql.DB, ticketID int) (*models.Ticket, error) {
	// Get basic ticket info
	ticket, err := GetTicket(db, ticketID)
	if err != nil {
		return nil, err
	}

	// Get comments
	comments, err := GetTicketComments(db, ticketID)
	if err != nil {
		log.Printf("Error retrieving comments: %v", err)
	} else {
		ticket.Comments = comments
	}

	// Get attachments
	attachments, err := GetTicketAttachments(db, ticketID)
	if err != nil {
		log.Printf("Error retrieving attachments: %v", err)
	} else {
		ticket.Attachments = attachments
	}

	// Get status history
	history, err := GetTicketStatusHistory(db, ticketID)
	if err != nil {
		log.Printf("Error retrieving status history: %v", err)
	} else {
		ticket.StatusHistory = history
	}

	return ticket, nil
}

// Additional functions to implement:
// - GetTicketAttachments
// - AddTicketAttachment
// - GetTicketStatusHistory
// - GetTicketStatistics
// etc.

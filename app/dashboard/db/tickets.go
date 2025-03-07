package db

import (
	"TeacherJournal/app/TeacherTickets/models"
	"database/sql"
	"fmt"
	"log"
	"time"
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

func GetTicket(db *sql.DB, ticketID int) (*models.Ticket, error) {
	// Query the ticket
	var ticket models.Ticket
	var assigneeID sql.NullInt64
	var createdAt, updatedAt string

	err := db.QueryRow(`
        SELECT id, creator_id, assignee_id, title, description, 
               status, priority, category, created_at, updated_at
        FROM tickets 
        WHERE id = ?
    `, ticketID).Scan(
		&ticket.ID, &ticket.CreatorID, &assigneeID, &ticket.Title,
		&ticket.Description, &ticket.Status, &ticket.Priority,
		&ticket.Category, &createdAt, &updatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert time strings to time.Time
	ticket.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	ticket.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	// Handle nullable assignee_id
	if assigneeID.Valid {
		id := int(assigneeID.Int64)
		ticket.AssigneeID = &id
	}

	return &ticket, nil
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

func GetTicketDetails(db *sql.DB, ticketID int) (*models.Ticket, error) {
	// First get the basic ticket information
	ticket, err := GetTicket(db, ticketID)
	if err != nil {
		return nil, err
	}

	// Get creator information
	var creatorFIO string
	err = db.QueryRow("SELECT fio FROM users WHERE id = ?", ticket.CreatorID).Scan(&creatorFIO)
	if err != nil {
		// Even if we can't get creator info, don't fail completely
		// Just log the error and continue
		log.Printf("Error getting creator info: %v", err)
	} else {
		// Create the UserInfo object for the creator
		ticket.Creator = &models.UserInfo{
			ID:  ticket.CreatorID,
			FIO: creatorFIO,
		}
	}

	// Similar code for assignee if ticket.AssigneeID is not nil
	if ticket.AssigneeID != nil {
		var assigneeFIO string
		err = db.QueryRow("SELECT fio FROM users WHERE id = ?", *ticket.AssigneeID).Scan(&assigneeFIO)
		if err != nil {
			log.Printf("Error getting assignee info: %v", err)
		} else {
			ticket.Assignee = &models.UserInfo{
				ID:  *ticket.AssigneeID,
				FIO: assigneeFIO,
			}
		}
	}

	// Get comments, attachments, status history, etc.
	// ...

	return ticket, nil
}

// GetTicketAttachments retrieves all attachments for a specified ticket
func GetTicketAttachments(db *sql.DB, ticketID int) ([]models.Attachment, error) {
	rows, err := db.Query(`
		SELECT a.id, a.ticket_id, a.file_name, a.file_path, a.uploaded_by, a.uploaded_at, u.fio
		FROM ticket_attachments a
		JOIN users u ON a.uploaded_by = u.id
		WHERE a.ticket_id = ?
		ORDER BY a.uploaded_at DESC
	`, ticketID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []models.Attachment
	for rows.Next() {
		var attachment models.Attachment
		var userFIO string

		err := rows.Scan(
			&attachment.ID, &attachment.TicketID, &attachment.FileName, &attachment.FilePath,
			&attachment.UploadedBy, &attachment.UploadedAt, &userFIO,
		)
		if err != nil {
			return nil, err
		}

		attachment.User = &models.UserInfo{
			ID:  attachment.UploadedBy,
			FIO: userFIO,
		}

		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

// AddTicketAttachment adds a new attachment to a ticket
func AddTicketAttachment(db *sql.DB, attachment *models.Attachment) error {
	result, err := db.Exec(`
		INSERT INTO ticket_attachments (ticket_id, file_name, file_path, uploaded_by, uploaded_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, attachment.TicketID, attachment.FileName, attachment.FilePath, attachment.UploadedBy)

	if err != nil {
		return err
	}

	attachmentID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	attachment.ID = int(attachmentID)

	// Update the ticket's updated_at timestamp
	_, err = db.Exec(`
		UPDATE tickets
		SET updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, attachment.TicketID)

	return err
}

// GetTicketStatusHistory retrieves the status history of a ticket
func GetTicketStatusHistory(db *sql.DB, ticketID int) ([]models.StatusChange, error) {
	rows, err := db.Query(`
		SELECT h.id, h.ticket_id, h.status, h.changed_by, h.changed_at, u.fio
		FROM ticket_status_history h
		JOIN users u ON h.changed_by = u.id
		WHERE h.ticket_id = ?
		ORDER BY h.changed_at DESC
	`, ticketID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.StatusChange
	for rows.Next() {
		var change models.StatusChange
		var userFIO string

		err := rows.Scan(
			&change.ID, &change.TicketID, &change.Status, &change.ChangedBy, &change.ChangedAt, &userFIO,
		)
		if err != nil {
			return nil, err
		}

		change.User = &models.UserInfo{
			ID:  change.ChangedBy,
			FIO: userFIO,
		}

		history = append(history, change)
	}

	return history, nil
}

// Helper function to calculate response time statistics
func calculateResponseTimes(db *sql.DB, timeStats *struct {
	Average string
	Min     string
	Max     string
}, whereClause string) error {
	// Query to calculate the time between ticket creation and first status change
	query := `
		SELECT 
			AVG(response_time) as avg_time,
			MIN(response_time) as min_time,
			MAX(response_time) as max_time
		FROM (
			SELECT 
				t.id,
				CAST((julianday(MIN(h.changed_at)) - julianday(t.created_at)) * 24 * 60 as INTEGER) as response_time
			FROM tickets t
			JOIN ticket_status_history h ON t.id = h.ticket_id
			` + whereClause + `
			GROUP BY t.id
			HAVING MIN(h.changed_at) > t.created_at
		) response_times
	`

	var avgMinutes, minMinutes, maxMinutes sql.NullInt64
	err := db.QueryRow(query).Scan(&avgMinutes, &minMinutes, &maxMinutes)
	if err != nil {
		return err
	}

	// Format the times as strings
	if avgMinutes.Valid {
		timeStats.Average = formatMinutes(avgMinutes.Int64)
	} else {
		timeStats.Average = "N/A"
	}

	if minMinutes.Valid {
		timeStats.Min = formatMinutes(minMinutes.Int64)
	} else {
		timeStats.Min = "N/A"
	}

	if maxMinutes.Valid {
		timeStats.Max = formatMinutes(maxMinutes.Int64)
	} else {
		timeStats.Max = "N/A"
	}

	return nil
}

// Helper function to calculate resolution time statistics
func calculateResolutionTimes(db *sql.DB, timeStats *struct {
	Average string
	Min     string
	Max     string
}, whereClause string) error {
	// Query to calculate the time between ticket creation and resolution
	query := `
		SELECT 
			AVG(resolution_time) as avg_time,
			MIN(resolution_time) as min_time,
			MAX(resolution_time) as max_time
		FROM (
			SELECT 
				t.id,
				CAST((julianday(h.changed_at) - julianday(t.created_at)) * 24 * 60 as INTEGER) as resolution_time
			FROM tickets t
			JOIN ticket_status_history h ON t.id = h.ticket_id
			` + whereClause + `
			AND h.status IN ('Resolved', 'Closed')
			AND t.status IN ('Resolved', 'Closed')
			GROUP BY t.id
			HAVING MIN(h.changed_at) > t.created_at
		) resolution_times
	`

	var avgMinutes, minMinutes, maxMinutes sql.NullInt64
	err := db.QueryRow(query).Scan(&avgMinutes, &minMinutes, &maxMinutes)
	if err != nil {
		return err
	}

	// Format the times as strings
	if avgMinutes.Valid {
		timeStats.Average = formatMinutes(avgMinutes.Int64)
	} else {
		timeStats.Average = "N/A"
	}

	if minMinutes.Valid {
		timeStats.Min = formatMinutes(minMinutes.Int64)
	} else {
		timeStats.Min = "N/A"
	}

	if maxMinutes.Valid {
		timeStats.Max = formatMinutes(maxMinutes.Int64)
	} else {
		timeStats.Max = "N/A"
	}

	return nil
}

// Helper function to format minutes into a readable duration
func formatMinutes(minutes int64) string {
	if minutes < 60 {
		return fmt.Sprintf("%d мин", minutes)
	}

	hours := minutes / 60
	mins := minutes % 60

	if hours < 24 {
		return fmt.Sprintf("%d ч %d мин", hours, mins)
	}

	days := hours / 24
	hrs := hours % 24

	return fmt.Sprintf("%d д %d ч %d мин", days, hrs, mins)
}

// GetAllTickets retrieves all tickets with optional filtering
func GetAllTickets(db *sql.DB, filters map[string]string, page, limit int) (*models.PaginatedTickets, error) {
	// Base query
	query := `
		SELECT 
			t.id, t.creator_id, t.assignee_id, t.title, t.description, 
			t.status, t.priority, t.category, t.created_at, t.updated_at,
			c.fio as creator_name, 
			a.fio as assignee_name
		FROM tickets t
		LEFT JOIN users c ON t.creator_id = c.id
		LEFT JOIN users a ON t.assignee_id = a.id
		WHERE 1=1
	`
	countQuery := "SELECT COUNT(*) FROM tickets WHERE 1=1"

	// Add filters
	args := []interface{}{}

	if status, ok := filters["status"]; ok && status != "" {
		query += " AND t.status = ?"
		countQuery += " AND status = ?"
		args = append(args, status)
	}

	if priority, ok := filters["priority"]; ok && priority != "" {
		query += " AND t.priority = ?"
		countQuery += " AND priority = ?"
		args = append(args, priority)
	}

	if category, ok := filters["category"]; ok && category != "" {
		query += " AND t.category = ?"
		countQuery += " AND category = ?"
		args = append(args, category)
	}

	if creatorID, ok := filters["creator_id"]; ok && creatorID != "" {
		query += " AND t.creator_id = ?"
		countQuery += " AND creator_id = ?"
		args = append(args, creatorID)
	}

	if assigneeID, ok := filters["assignee_id"]; ok && assigneeID != "" {
		if assigneeID == "unassigned" {
			query += " AND t.assignee_id IS NULL"
			countQuery += " AND assignee_id IS NULL"
		} else {
			query += " AND t.assignee_id = ?"
			countQuery += " AND assignee_id = ?"
			args = append(args, assigneeID)
		}
	}

	// Order and pagination
	query += " ORDER BY t.updated_at DESC LIMIT ? OFFSET ?"

	// Calculate pagination
	offset := (page - 1) * limit
	queryArgs := append(args, limit, offset)

	// Count total tickets with filters
	var total int
	err := db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Calculate total pages
	pages := (total + limit - 1) / limit
	if pages < 1 {
		pages = 1
	}

	// Get tickets
	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := []models.Ticket{}
	for rows.Next() {
		var ticket models.Ticket
		var assigneeID sql.NullInt64
		var assigneeName sql.NullString
		var creatorName string
		var createdAt, updatedAt string

		err := rows.Scan(
			&ticket.ID, &ticket.CreatorID, &assigneeID, &ticket.Title, &ticket.Description,
			&ticket.Status, &ticket.Priority, &ticket.Category, &createdAt, &updatedAt,
			&creatorName, &assigneeName,
		)
		if err != nil {
			return nil, err
		}

		// Convert time strings to time.Time
		ticket.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		ticket.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		// Set assignee information if available
		if assigneeID.Valid {
			id := int(assigneeID.Int64)
			ticket.AssigneeID = &id
			ticket.Assignee = &models.UserInfo{
				ID:  id,
				FIO: assigneeName.String,
			}
		}

		// Set creator information
		ticket.Creator = &models.UserInfo{
			ID:  ticket.CreatorID,
			FIO: creatorName,
		}

		tickets = append(tickets, ticket)
	}

	// Create paginated response
	paginatedTickets := &models.PaginatedTickets{
		Tickets: tickets,
		Pagination: models.Pagination{
			Total: total,
			Page:  page,
			Limit: limit,
			Pages: pages,
		},
	}

	return paginatedTickets, nil
}

// GetTicketStatistics returns basic statistics about tickets
func GetTicketStatistics(db *sql.DB) (*models.TicketStatistics, error) {
	statistics := &models.TicketStatistics{
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
		ByCategory: make(map[string]int),
	}

	// Get total count
	err := db.QueryRow("SELECT COUNT(*) FROM tickets").Scan(&statistics.Total)
	if err != nil {
		return nil, err
	}

	// Count by status
	rows, err := db.Query("SELECT status, COUNT(*) FROM tickets GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		statistics.ByStatus[status] = count
	}

	// Count by priority
	rows, err = db.Query("SELECT priority, COUNT(*) FROM tickets GROUP BY priority")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var priority string
		var count int
		if err := rows.Scan(&priority, &count); err != nil {
			return nil, err
		}
		statistics.ByPriority[priority] = count
	}

	// Count by category
	rows, err = db.Query("SELECT category, COUNT(*) FROM tickets GROUP BY category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		statistics.ByCategory[category] = count
	}

	// Average response time
	var avgResponseTime float64
	err = db.QueryRow(`
		SELECT 
			COALESCE(AVG(JULIANDAY(min_time) - JULIANDAY(created_at)), 0) 
		FROM (
			SELECT 
				t.id, 
				t.created_at, 
				MIN(CASE WHEN t.status != 'New' THEN sh.changed_at ELSE NULL END) as min_time
			FROM tickets t
			LEFT JOIN ticket_status_history sh ON t.id = sh.ticket_id
			GROUP BY t.id
		)
	`).Scan(&avgResponseTime)
	if err != nil {
		statistics.ResponseTime.Average = "N/A"
	} else {
		// Convert to hours (SQLite returns days)
		avgHours := avgResponseTime * 24
		statistics.ResponseTime.Average = formatDuration(time.Duration(avgHours * float64(time.Hour)))
	}

	// Average resolution time
	var avgResolutionTime float64
	err = db.QueryRow(`
		SELECT 
			COALESCE(AVG(JULIANDAY(min_resolved) - JULIANDAY(created_at)), 0) 
		FROM (
			SELECT 
				t.id, 
				t.created_at, 
				MIN(CASE WHEN sh.status = 'Resolved' OR sh.status = 'Closed' THEN sh.changed_at ELSE NULL END) as min_resolved
			FROM tickets t
			LEFT JOIN ticket_status_history sh ON t.id = sh.ticket_id
			GROUP BY t.id
			HAVING min_resolved IS NOT NULL
		)
	`).Scan(&avgResolutionTime)
	if err != nil {
		statistics.ResolutionTime.Average = "N/A"
	} else {
		// Convert to hours (SQLite returns days)
		avgHours := avgResolutionTime * 24
		statistics.ResolutionTime.Average = formatDuration(time.Duration(avgHours * float64(time.Hour)))
	}

	return statistics, nil
}

// GetDetailedTicketStatistics returns detailed statistics with time ranges
func GetDetailedTicketStatistics(db *sql.DB) (*models.TicketStatistics, error) {
	// Get basic statistics first
	statistics, err := GetTicketStatistics(db)
	if err != nil {
		return nil, err
	}

	// Add min/max response times
	var minResponseTime, maxResponseTime float64
	err = db.QueryRow(`
		SELECT 
			MIN(response_time), 
			MAX(response_time)
		FROM (
			SELECT 
				(JULIANDAY(min_time) - JULIANDAY(created_at)) * 24 as response_time
			FROM (
				SELECT 
					t.id, 
					t.created_at, 
					MIN(CASE WHEN t.status != 'New' THEN sh.changed_at ELSE NULL END) as min_time
				FROM tickets t
				LEFT JOIN ticket_status_history sh ON t.id = sh.ticket_id
				GROUP BY t.id
				HAVING min_time IS NOT NULL
			)
		)
	`).Scan(&minResponseTime, &maxResponseTime)
	if err != nil {
		statistics.ResponseTime.Min = "N/A"
		statistics.ResponseTime.Max = "N/A"
	} else {
		// Convert to hours
		statistics.ResponseTime.Min = formatDuration(time.Duration(minResponseTime * float64(time.Hour)))
		statistics.ResponseTime.Max = formatDuration(time.Duration(maxResponseTime * float64(time.Hour)))
	}

	// Add min/max resolution times
	var minResolutionTime, maxResolutionTime float64
	err = db.QueryRow(`
		SELECT 
			MIN(resolution_time), 
			MAX(resolution_time)
		FROM (
			SELECT 
				(JULIANDAY(min_resolved) - JULIANDAY(created_at)) * 24 as resolution_time
			FROM (
				SELECT 
					t.id, 
					t.created_at, 
					MIN(CASE WHEN sh.status = 'Resolved' OR sh.status = 'Closed' THEN sh.changed_at ELSE NULL END) as min_resolved
				FROM tickets t
				LEFT JOIN ticket_status_history sh ON t.id = sh.ticket_id
				GROUP BY t.id
				HAVING min_resolved IS NOT NULL
			)
		)
	`).Scan(&minResolutionTime, &maxResolutionTime)
	if err != nil {
		statistics.ResolutionTime.Min = "N/A"
		statistics.ResolutionTime.Max = "N/A"
	} else {
		// Convert to hours
		statistics.ResolutionTime.Min = formatDuration(time.Duration(minResolutionTime * float64(time.Hour)))
		statistics.ResolutionTime.Max = formatDuration(time.Duration(maxResolutionTime * float64(time.Hour)))
	}

	return statistics, nil
}

// Helper function to format duration in a user-friendly way
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d д. %d ч.", days, hours)
	}

	if hours > 0 {
		return fmt.Sprintf("%d ч. %d мин.", hours, minutes)
	}

	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d мин. %d сек.", minutes, seconds)
}

// DeleteTicket removes a ticket from the database
func DeleteTicket(db *sql.DB, ticketID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Delete from ticket_comments
	_, err = tx.Exec("DELETE FROM ticket_comments WHERE ticket_id = ?", ticketID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Delete from ticket_status_history
	_, err = tx.Exec("DELETE FROM ticket_status_history WHERE ticket_id = ?", ticketID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Delete from ticket_attachments
	_, err = tx.Exec("DELETE FROM ticket_attachments WHERE ticket_id = ?", ticketID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Delete from tickets
	_, err = tx.Exec("DELETE FROM tickets WHERE id = ?", ticketID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// GetTicketComment retrieves a specific comment with user information
func GetTicketComment(db *sql.DB, commentID int) (*models.TicketComment, error) {
	var comment models.TicketComment
	var createdAt string
	var userFIO string

	err := db.QueryRow(`
		SELECT 
			c.id, c.ticket_id, c.user_id, c.comment, c.created_at,
			u.fio
		FROM ticket_comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.id = ?
	`, commentID).Scan(
		&comment.ID, &comment.TicketID, &comment.UserID, &comment.Comment, &createdAt,
		&userFIO,
	)

	if err != nil {
		return nil, err
	}

	// Convert time string to time.Time
	comment.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

	// Set user information
	comment.User = &models.UserInfo{
		ID:  comment.UserID,
		FIO: userFIO,
	}

	return &comment, nil
}

// DeleteTicketComment removes a comment from the database
func DeleteTicketComment(db *sql.DB, commentID int) error {
	_, err := db.Exec("DELETE FROM ticket_comments WHERE id = ?", commentID)
	return err
}

// GetNotificationSettings retrieves notification preferences for a user
func GetNotificationSettings(db *sql.DB, userID int) (*models.NotificationSettings, error) {
	var settings models.NotificationSettings
	settings.UserID = userID

	err := db.QueryRow(`
		SELECT 
			notify_new_ticket, notify_ticket_update, 
			notify_ticket_comment, notify_ticket_status
		FROM user_notification_settings
		WHERE user_id = ?
	`, userID).Scan(
		&settings.NotifyNewTicket, &settings.NotifyTicketUpdate,
		&settings.NotifyTicketComment, &settings.NotifyTicketStatus,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Return default settings
			settings.NotifyNewTicket = true
			settings.NotifyTicketUpdate = true
			settings.NotifyTicketComment = true
			settings.NotifyTicketStatus = true
			return &settings, nil
		}
		return nil, err
	}

	return &settings, nil
}

// SaveNotificationSettings updates or inserts notification preferences for a user
func SaveNotificationSettings(db *sql.DB, settings *models.NotificationSettings) error {
	// Check if settings exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM user_notification_settings WHERE user_id = ?", settings.UserID).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing settings
		_, err = db.Exec(`
			UPDATE user_notification_settings
			SET notify_new_ticket = ?, notify_ticket_update = ?,
				notify_ticket_comment = ?, notify_ticket_status = ?
			WHERE user_id = ?
		`,
			settings.NotifyNewTicket, settings.NotifyTicketUpdate,
			settings.NotifyTicketComment, settings.NotifyTicketStatus,
			settings.UserID,
		)
	} else {
		// Insert new settings
		_, err = db.Exec(`
			INSERT INTO user_notification_settings (
				user_id, notify_new_ticket, notify_ticket_update,
				notify_ticket_comment, notify_ticket_status
			) VALUES (?, ?, ?, ?, ?)
		`,
			settings.UserID, settings.NotifyNewTicket, settings.NotifyTicketUpdate,
			settings.NotifyTicketComment, settings.NotifyTicketStatus,
		)
	}

	return err
}

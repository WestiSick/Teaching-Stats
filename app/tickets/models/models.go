package models

import (
	"time"
)

// Ticket represents a support ticket in the system
type Ticket struct {
	ID           int       `gorm:"primaryKey"`
	Title        string    `gorm:"not null"`
	Description  string    `gorm:"type:text;not null"`
	Status       string    `gorm:"not null;default:New"`    // New, Open, InProgress, Resolved, Closed
	Priority     string    `gorm:"not null;default:Medium"` // Low, Medium, High, Critical
	Category     string    `gorm:"not null"`                // Technical, Administrative, Account, Feature, Bug, Other
	CreatedBy    int       `gorm:"not null"`                // UserID from main app
	AssignedTo   *int      `gorm:"default:null"`            // UserID from main app, can be null
	CreatedAt    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	LastActivity time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TicketComment represents a comment on a ticket
type TicketComment struct {
	ID         int       `gorm:"primaryKey"`
	TicketID   int       `gorm:"not null"`
	Ticket     Ticket    `gorm:"foreignKey:TicketID"`
	UserID     int       `gorm:"not null"` // UserID from main app
	Content    string    `gorm:"type:text;not null"`
	CreatedAt  time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	IsInternal bool      `gorm:"not null;default:false"` // For internal notes visible only to staff
}

// TicketAttachment represents a file attached to a ticket
type TicketAttachment struct {
	ID          int           `gorm:"primaryKey"`
	TicketID    int           `gorm:"not null"`
	Ticket      Ticket        `gorm:"foreignKey:TicketID"`
	CommentID   *int          `gorm:"default:null"` // Can be attached to a comment or directly to a ticket
	Comment     TicketComment `gorm:"foreignKey:CommentID"`
	FileName    string        `gorm:"not null"`
	FilePath    string        `gorm:"not null"` // Server path where file is stored
	FileSize    int64         `gorm:"not null"`
	ContentType string        `gorm:"not null"`
	UploadedBy  int           `gorm:"not null"` // UserID from main app
	UploadedAt  time.Time     `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TicketHistory records all changes to a ticket
type TicketHistory struct {
	ID         int       `gorm:"primaryKey"`
	TicketID   int       `gorm:"not null"`
	Ticket     Ticket    `gorm:"foreignKey:TicketID"`
	UserID     int       `gorm:"not null"` // UserID who made the change
	FieldName  string    `gorm:"not null"` // Which field was changed
	OldValue   string    `gorm:"type:text"`
	NewValue   string    `gorm:"type:text"`
	ChangeTime time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TicketSubscription allows users to subscribe to ticket updates
type TicketSubscription struct {
	ID         int    `gorm:"primaryKey"`
	TicketID   int    `gorm:"not null"`
	Ticket     Ticket `gorm:"foreignKey:TicketID"`
	UserID     int    `gorm:"not null"` // UserID from main app
	Subscribed bool   `gorm:"not null;default:true"`
}

// APIResponse for ticket system API endpoints
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

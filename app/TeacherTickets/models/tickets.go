package models

import (
	"time"
)

// Ticket represents a support ticket
type Ticket struct {
	ID          int       `json:"id"`
	CreatorID   int       `json:"creator_id"`
	AssigneeID  *int      `json:"assignee_id,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Populated fields (not stored directly in the database)
	Creator       *UserInfo       `json:"creator,omitempty"`
	Assignee      *UserInfo       `json:"assignee,omitempty"`
	Comments      []TicketComment `json:"comments,omitempty"`
	Attachments   []Attachment    `json:"attachments,omitempty"`
	StatusHistory []StatusChange  `json:"status_history,omitempty"`
}

// TicketComment represents a comment on a ticket
type TicketComment struct {
	ID        int       `json:"id"`
	TicketID  int       `json:"ticket_id"`
	UserID    int       `json:"user_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`

	// Populated fields
	User *UserInfo `json:"user,omitempty"`
}

// Attachment represents a file attached to a ticket
type Attachment struct {
	ID         int       `json:"id"`
	TicketID   int       `json:"ticket_id"`
	FileName   string    `json:"file_name"`
	FilePath   string    `json:"file_path"`
	UploadedBy int       `json:"uploaded_by"`
	UploadedAt time.Time `json:"uploaded_at"`

	// Populated fields
	User *UserInfo `json:"user,omitempty"`
}

// StatusChange represents a change in ticket status
type StatusChange struct {
	ID        int       `json:"id"`
	TicketID  int       `json:"ticket_id"`
	Status    string    `json:"status"`
	ChangedBy int       `json:"changed_by"`
	ChangedAt time.Time `json:"changed_at"`

	// Populated fields
	User *UserInfo `json:"user,omitempty"`
}

// NotificationSettings represents a user's notification preferences
type NotificationSettings struct {
	UserID              int  `json:"user_id"`
	NotifyNewTicket     bool `json:"notify_new_ticket"`
	NotifyTicketUpdate  bool `json:"notify_ticket_update"`
	NotifyTicketComment bool `json:"notify_ticket_comment"`
	NotifyTicketStatus  bool `json:"notify_ticket_status"`
}

// UserInfo contains basic user information
type UserInfo struct {
	ID   int    `json:"id"`
	FIO  string `json:"fio"`
	Role string `json:"role"` // Add this line
}

// TicketStatistics contains statistics about tickets
type TicketStatistics struct {
	Total        int            `json:"total"`
	ByStatus     map[string]int `json:"by_status"`
	ByPriority   map[string]int `json:"by_priority"`
	ByCategory   map[string]int `json:"by_category"`
	ResponseTime struct {
		Average string `json:"average"`
		Min     string `json:"min"`
		Max     string `json:"max"`
	} `json:"response_time"`
	ResolutionTime struct {
		Average string `json:"average"`
		Min     string `json:"min"`
		Max     string `json:"max"`
	} `json:"resolution_time"`
}

// PaginatedTickets represents a paginated list of tickets
type PaginatedTickets struct {
	Tickets    []Ticket   `json:"tickets"`
	Pagination Pagination `json:"pagination"`
}

// Pagination contains pagination information
type Pagination struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Pages int `json:"pages"`
}

package models

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID       int    `gorm:"primaryKey"`
	FIO      string `gorm:"not null"`
	Login    string `gorm:"uniqueIndex;not null"`
	Password string `gorm:"not null"`
	Role     string `gorm:"not null"`
}

// Lesson represents a teaching lesson
type Lesson struct {
	ID        int    `gorm:"primaryKey"`
	TeacherID int    `gorm:"index"`
	Teacher   User   `gorm:"foreignKey:TeacherID"`
	GroupName string `gorm:"not null"`
	Subject   string `gorm:"not null"`
	Topic     string `gorm:"not null"`
	Hours     int    `gorm:"not null"`
	Date      string `gorm:"not null"`
	Type      string `gorm:"not null;default:Лекция"`
}

// Student represents a student in a group
type Student struct {
	ID         int    `gorm:"primaryKey"`
	TeacherID  int    `gorm:"index"`
	Teacher    User   `gorm:"foreignKey:TeacherID"`
	GroupName  string `gorm:"not null;index"`
	StudentFIO string `gorm:"not null"`
}

// Attendance represents attendance record for a student
type Attendance struct {
	ID        int     `gorm:"primaryKey"`
	LessonID  int     `gorm:"index"`
	Lesson    Lesson  `gorm:"foreignKey:LessonID"`
	StudentID int     `gorm:"index"`
	Student   Student `gorm:"foreignKey:StudentID"`
	Attended  int     `gorm:"not null;default:0"`
}

// Log represents a system log entry
type Log struct {
	ID        int       `gorm:"primaryKey"`
	UserID    int       `gorm:"index"`
	User      User      `gorm:"foreignKey:UserID"`
	Action    string    `gorm:"not null"`
	Details   string    `gorm:"not null"`
	Timestamp time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// LabSettings represents settings for laboratory works
type LabSettings struct {
	ID        int    `gorm:"primaryKey"`
	TeacherID int    `gorm:"index"`
	Teacher   User   `gorm:"foreignKey:TeacherID"`
	GroupName string `gorm:"not null"`
	Subject   string `gorm:"not null"`
	TotalLabs int    `gorm:"not null;default:5"`
}

// LabGrade represents a grade for a lab work
type LabGrade struct {
	ID        int     `gorm:"primaryKey"`
	StudentID int     `gorm:"index"`
	Student   Student `gorm:"foreignKey:StudentID"`
	TeacherID int     `gorm:"index"`
	Teacher   User    `gorm:"foreignKey:TeacherID"`
	Subject   string  `gorm:"not null"`
	LabNumber int     `gorm:"not null"`
	Grade     int     `gorm:"not null"`
}

// Ticket represents a support ticket
type Ticket struct {
	ID          int       `gorm:"primaryKey"`
	CreatorID   int       `gorm:"index;not null"`
	Creator     User      `gorm:"foreignKey:CreatorID"`
	AssigneeID  *int      `gorm:"index"`
	Assignee    User      `gorm:"foreignKey:AssigneeID"`
	Title       string    `gorm:"not null"`
	Description string    `gorm:"not null"`
	Status      string    `gorm:"not null"`
	Priority    string    `gorm:"not null"`
	Category    string    `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TicketComment represents a comment on a ticket
type TicketComment struct {
	ID        int       `gorm:"primaryKey"`
	TicketID  int       `gorm:"index;not null"`
	Ticket    Ticket    `gorm:"foreignKey:TicketID"`
	UserID    int       `gorm:"index;not null"`
	User      User      `gorm:"foreignKey:UserID"`
	Comment   string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TicketAttachment represents a file attached to a ticket
type TicketAttachment struct {
	ID         int       `gorm:"primaryKey"`
	TicketID   int       `gorm:"index;not null"`
	Ticket     Ticket    `gorm:"foreignKey:TicketID"`
	FileName   string    `gorm:"not null"`
	FilePath   string    `gorm:"not null"`
	UploadedBy int       `gorm:"index;not null"`
	User       User      `gorm:"foreignKey:UploadedBy"`
	UploadedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TicketStatusHistory represents a record of status changes for a ticket
type TicketStatusHistory struct {
	ID        int       `gorm:"primaryKey"`
	TicketID  int       `gorm:"index;not null"`
	Ticket    Ticket    `gorm:"foreignKey:TicketID"`
	Status    string    `gorm:"not null"`
	ChangedBy int       `gorm:"index;not null"`
	User      User      `gorm:"foreignKey:ChangedBy"`
	ChangedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// UserNotificationSettings represents notification preferences for a user
type UserNotificationSettings struct {
	UserID              int  `gorm:"primaryKey"`
	User                User `gorm:"foreignKey:UserID"`
	NotifyNewTicket     bool `gorm:"not null;default:true"`
	NotifyTicketUpdate  bool `gorm:"not null;default:true"`
	NotifyTicketComment bool `gorm:"not null;default:true"`
	NotifyTicketStatus  bool `gorm:"not null;default:true"`
}

// APIResponse is a generic API response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

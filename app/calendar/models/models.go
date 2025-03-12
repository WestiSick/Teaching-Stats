package models

import (
	"TeacherJournal/app/dashboard/models"
	"time"
)

// Event represents a calendar event
type Event struct {
	ID          int         `gorm:"primaryKey"`
	CreatorID   int         `gorm:"index;not null"`
	Creator     models.User `gorm:"foreignKey:CreatorID"`
	Title       string      `gorm:"not null"`
	Description string
	Location    string
	StartTime   time.Time `gorm:"not null"`
	EndTime     time.Time `gorm:"not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// EventParticipant represents a user participating in an event
type EventParticipant struct {
	ID        int         `gorm:"primaryKey"`
	EventID   int         `gorm:"index;not null"`
	Event     Event       `gorm:"foreignKey:EventID"`
	UserID    int         `gorm:"index;not null"`
	User      models.User `gorm:"foreignKey:UserID"`
	Status    string      `gorm:"not null;default:'pending'"` // pending, accepted, declined
	CreatedAt time.Time   `gorm:"autoCreateTime"`
	UpdatedAt time.Time   `gorm:"autoUpdateTime"`
}

// EventAttachment represents a file attached to an event
type EventAttachment struct {
	ID         int         `gorm:"primaryKey"`
	EventID    int         `gorm:"index;not null"`
	Event      Event       `gorm:"foreignKey:EventID"`
	FileName   string      `gorm:"not null"`
	FilePath   string      `gorm:"not null"`
	FileSize   int64       `gorm:"not null"`
	UploadedBy int         `gorm:"index;not null"`
	User       models.User `gorm:"foreignKey:UploadedBy"`
	CreatedAt  time.Time   `gorm:"autoCreateTime"`
}

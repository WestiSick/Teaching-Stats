package db

import (
	"TeacherJournal/app/dashboard/models"
	"crypto/rand"
	"encoding/base64"
	"time"

	"gorm.io/gorm"
)

// Generate a secure random token
func generateToken(length int) (string, error) {
	buffer := make([]byte, length)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}

	// Convert to base64 and remove URL-unsafe characters
	token := base64.URLEncoding.EncodeToString(buffer)
	return token[:length], nil
}

// CreateSharedLabLink creates a new shared link for lab grades
func CreateSharedLabLink(db *gorm.DB, teacherID int, groupName, subject string, expirationDays int) (string, error) {
	// Generate a unique token
	token, err := generateToken(16)
	if err != nil {
		return "", err
	}

	// Create a new shared link record
	sharedLink := models.SharedLabLink{
		Token:       token,
		TeacherID:   teacherID,
		GroupName:   groupName,
		Subject:     subject,
		CreatedAt:   time.Now(),
		AccessCount: 0,
	}

	// Set expiration date if specified
	if expirationDays > 0 {
		expiresAt := time.Now().AddDate(0, 0, expirationDays)
		sharedLink.ExpiresAt = &expiresAt
	}

	// Save to database
	if err := db.Create(&sharedLink).Error; err != nil {
		return "", err
	}

	return token, nil
}

// GetSharedLabLink retrieves a shared link by token
func GetSharedLabLink(db *gorm.DB, token string) (*models.SharedLabLink, error) {
	var link models.SharedLabLink
	result := db.Where("token = ?", token).First(&link)
	if result.Error != nil {
		return nil, result.Error
	}

	// Check if link has expired
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return nil, gorm.ErrRecordNotFound
	}

	// Increment access count
	db.Model(&link).Update("access_count", link.AccessCount+1)

	return &link, nil
}

// GetTeacherSharedLinks gets all shared links created by a teacher
func GetTeacherSharedLinks(db *gorm.DB, teacherID int) ([]models.SharedLabLink, error) {
	var links []models.SharedLabLink
	result := db.Where("teacher_id = ?", teacherID).Order("created_at DESC").Find(&links)
	return links, result.Error
}

// DeleteSharedLabLink deletes a shared link
func DeleteSharedLabLink(db *gorm.DB, teacherID int, token string) error {
	return db.Where("teacher_id = ? AND token = ?", teacherID, token).Delete(&models.SharedLabLink{}).Error
}

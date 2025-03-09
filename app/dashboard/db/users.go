package db

import (
	"TeacherJournal/app/dashboard/models"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"gorm.io/gorm"
)

// GetUserInfo retrieves user information from the session
func GetUserInfo(db *gorm.DB, r *http.Request, store *sessions.CookieStore, sessionName string) (UserInfo, error) {
	session, _ := store.Get(r, sessionName)
	userID, ok := session.Values["userID"].(int)
	if !ok {
		return UserInfo{}, fmt.Errorf("unauthorized access")
	}

	var user models.User
	err := db.Model(&models.User{}).
		Select("fio, role, id").
		Where("id = ?", userID).
		First(&user).Error

	if err != nil {
		return UserInfo{}, err
	}

	return UserInfo{
		FIO:  user.FIO,
		Role: user.Role,
		ID:   user.ID,
	}, nil
}

// UserInfo stores basic user information
type UserInfo struct {
	FIO  string
	Role string
	ID   int
}

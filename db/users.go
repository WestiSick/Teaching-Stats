package db

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
)

// GetUserInfo retrieves user information from the session
func GetUserInfo(db *sql.DB, r *http.Request, store *sessions.CookieStore, sessionName string) (UserInfo, error) {
	session, _ := store.Get(r, sessionName)
	userID, ok := session.Values["userID"].(int)
	if !ok {
		return UserInfo{}, fmt.Errorf("unauthorized access")
	}

	var info UserInfo
	err := db.QueryRow("SELECT fio, role, id FROM users WHERE id = ?", userID).
		Scan(&info.FIO, &info.Role, &info.ID)
	if err != nil {
		return UserInfo{}, err
	}
	return info, nil
}

// UserInfo stores basic user information
type UserInfo struct {
	FIO  string
	Role string
	ID   int
}

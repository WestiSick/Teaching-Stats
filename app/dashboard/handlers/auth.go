package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/app/dashboard/utils"
	"TeacherJournal/config"
	"fmt"
	"html/template"
	"net/http"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler handles authentication-related routes
type AuthHandler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(database *gorm.DB, tmpl *template.Template) *AuthHandler {
	return &AuthHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// IndexHandler handles the index page
func (h *AuthHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := config.Store.Get(r, config.SessionName)
	if userID, ok := session.Values["userID"].(int); ok && userID != 0 {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	data := struct {
		User db.UserInfo
	}{
		User: db.UserInfo{},
	}
	renderTemplate(w, h.Tmpl, "index.html", data)
}

// RegisterHandler handles user registration
func (h *AuthHandler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		data := struct {
			User db.UserInfo
		}{
			User: db.UserInfo{},
		}
		renderTemplate(w, h.Tmpl, "register.html", data)
		return
	}

	// Process registration form
	fio := r.FormValue("fio")
	login := r.FormValue("login")
	password := r.FormValue("password")
	role := "free" // Changed from "teacher" to "free" for the subscription model

	// Validate inputs
	if fio == "" || login == "" || password == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	// Hash password and create user
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		HandleError(w, err, "Error hashing password", http.StatusInternalServerError)
		return
	}

	// Create new user
	user := models.User{
		FIO:      fio,
		Login:    login,
		Password: string(hashedPassword),
		Role:     role,
	}

	result := h.DB.Create(&user)
	if result.Error != nil {
		http.Error(w, "Registration error", http.StatusBadRequest)
		return
	}

	// Add log entry
	db.LogAction(h.DB, user.ID, "Registration", fmt.Sprintf("New user registered: %s (%s)", fio, login))

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// SubscriptionHandler displays the subscription page for free users
func (h *AuthHandler) SubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	data := struct {
		User db.UserInfo
	}{
		User: userInfo,
	}
	renderTemplate(w, h.Tmpl, "subscription.html", data)
}

// LoginHandler handles user login
func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		data := struct {
			User db.UserInfo
		}{
			User: db.UserInfo{},
		}
		renderTemplate(w, h.Tmpl, "login.html", data)
		return
	}

	// Process login form
	login := r.FormValue("login")
	password := r.FormValue("password")

	// Validate login credentials
	var user models.User
	err := h.DB.Model(&models.User{}).
		Where("login = ?", login).
		First(&user).Error

	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		http.Error(w, "Invalid login or password", http.StatusUnauthorized)
		return
	}

	// Set user session
	session, _ := config.Store.Get(r, config.SessionName)
	session.Values["userID"] = user.ID
	session.Save(r, w)

	db.LogAction(h.DB, user.ID, "Authentication", fmt.Sprintf("User %s (%s) logged in", user.FIO, login))

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// LogoutHandler handles user logout
func (h *AuthHandler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := config.Store.Get(r, config.SessionName)
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// HandleError handles HTTP errors with logging
func HandleError(w http.ResponseWriter, err error, message string, statusCode int) {
	utils.HandleError(w, err, message, statusCode)
}

// renderTemplate renders a template with standard data structure
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) {
	err := tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		HandleError(w, err, "Template rendering error", http.StatusInternalServerError)
	}
}

package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/tealeg/xlsx"
	"golang.org/x/crypto/bcrypt"

	"TeacherJournal/db"    // Import our database package
	"TeacherJournal/utils" // Import our utils package
)

// Application constants
const (
	cookieStoreKey = "super-secret-key"
	sessionName    = "session-name"
)

// Global store for sessions
var store = sessions.NewCookieStore([]byte(cookieStoreKey))

// ============================================================================
// MIDDLEWARE
// ============================================================================

// Middleware to verify user authentication
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, sessionName)
		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// Middleware to verify admin role
func adminMiddleware(database *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, sessionName)
		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var role string
		err := database.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
		if err != nil || role != "admin" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// Renders a template with standard data structure
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) {
	err := tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		utils.HandleError(w, err, "Template rendering error", http.StatusInternalServerError)
	}
}

// Helper functions for templates
func createTemplateHelperFunctions() template.FuncMap {
	return template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
		"inc": func(i int) int {
			return i + 1
		},
		"divideAndMultiply": func(a, b int, multiplier float64) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b) * multiplier
		},
		"ge": func(a, b float64) bool {
			return a >= b
		},
	}
}

// ============================================================================
// PAGE HANDLERS - Basic Navigation
// ============================================================================

// Handler for the index page
func indexHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, sessionName)
		if userID, ok := session.Values["userID"].(int); ok && userID != 0 {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}

		data := struct {
			User db.UserInfo
		}{
			User: db.UserInfo{},
		}
		renderTemplate(w, tmpl, "index.html", data)
	}
}

// Handler for user registration
func registerHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			data := struct {
				User db.UserInfo
			}{
				User: db.UserInfo{},
			}
			renderTemplate(w, tmpl, "register.html", data)
			return
		}

		// Process registration form
		fio := r.FormValue("fio")
		login := r.FormValue("login")
		password := r.FormValue("password")
		role := "teacher"

		// Validate inputs
		if fio == "" || login == "" || password == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		// Hash password and create user
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			utils.HandleError(w, err, "Error hashing password", http.StatusInternalServerError)
			return
		}

		result, err := db.ExecuteQuery(database,
			"INSERT INTO users (fio, login, password, role) VALUES (?, ?, ?, ?)",
			fio, login, hashedPassword, role)
		if err != nil {
			http.Error(w, "Registration error", http.StatusBadRequest)
			return
		}

		userID, _ := result.LastInsertId()
		db.LogAction(database, int(userID), "Registration", fmt.Sprintf("New user registered: %s (%s)", fio, login))

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// Handler for user login
func loginHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			data := struct {
				User db.UserInfo
			}{
				User: db.UserInfo{},
			}
			renderTemplate(w, tmpl, "login.html", data)
			return
		}

		// Process login form
		login := r.FormValue("login")
		password := r.FormValue("password")

		// Validate login credentials
		var user struct {
			ID       int
			FIO      string
			Password string
		}

		err := database.QueryRow("SELECT id, fio, password FROM users WHERE login = ?", login).
			Scan(&user.ID, &user.FIO, &user.Password)
		if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
			http.Error(w, "Invalid login or password", http.StatusUnauthorized)
			return
		}

		// Set user session
		session, _ := store.Get(r, sessionName)
		session.Values["userID"] = user.ID
		session.Save(r, w)

		db.LogAction(database, user.ID, "Authentication", fmt.Sprintf("User %s (%s) logged in", user.FIO, login))

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

// Handler for user logout
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, sessionName)
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ============================================================================
// PAGE HANDLERS - Dashboard & Statistics
// ============================================================================

// Handler for the dashboard page
func dashboardHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Get lesson statistics
		var totalLessons, totalHours sql.NullInt64
		err = database.QueryRow(
			"SELECT COUNT(*) as total_lessons, SUM(hours) as total_hours FROM lessons WHERE teacher_id = ?",
			userInfo.ID).Scan(&totalLessons, &totalHours)

		lessonsExist := true
		if err != nil {
			if err == sql.ErrNoRows {
				totalLessons = sql.NullInt64{Int64: 0, Valid: true}
				totalHours = sql.NullInt64{Int64: 0, Valid: true}
				lessonsExist = false
			} else {
				utils.HandleError(w, err, "Error retrieving statistics", http.StatusInternalServerError)
				return
			}
		}

		// Get subject statistics
		subjects := make(map[string]int)
		rows, err := database.Query("SELECT subject, COUNT(*) as count FROM lessons WHERE teacher_id = ? GROUP BY subject", userInfo.ID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var subject string
				var count int
				rows.Scan(&subject, &count)
				subjects[subject] = count
			}
		} else if err != sql.ErrNoRows {
			utils.HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
			return
		}

		// Get groups
		groups, err := db.GetTeacherGroups(database, userInfo.ID)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		// Render dashboard template
		data := struct {
			User         db.UserInfo
			TotalLessons int
			TotalHours   int
			Subjects     map[string]int
			Groups       []string
			HasLessons   bool
		}{
			User:         userInfo,
			TotalLessons: int(totalLessons.Int64),
			TotalHours:   int(totalHours.Int64),
			Subjects:     subjects,
			Groups:       groups,
			HasLessons:   lessonsExist,
		}
		renderTemplate(w, tmpl, "dashboard.html", data)
	}
}

// Handler for the export functionality
func exportExcelHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		subject := r.URL.Query().Get("subject")
		group := r.URL.Query().Get("group")

		// Build query with filters
		query := "SELECT date, group_name, subject, topic, hours, type FROM lessons WHERE teacher_id = ? AND subject = ?"
		args := []interface{}{userInfo.ID, subject}

		if group != "" {
			query += " AND group_name = ?"
			args = append(args, group)
		}

		query += " ORDER BY date"

		// Execute query
		rows, err := database.Query(query, args...)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving data", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Create Excel file
		file := xlsx.NewFile()
		sheet, _ := file.AddSheet("Statistics")
		header := sheet.AddRow()
		header.WriteSlice(&[]string{"Date", "Group", "Subject", "Topic", "Hours", "Type"}, -1)

		// Populate data
		for rows.Next() {
			var dateStr, groupName, subj, topic, lessonType string
			var hours int
			rows.Scan(&dateStr, &groupName, &subj, &topic, &hours, &lessonType)

			formattedDate := utils.FormatDate(dateStr)

			row := sheet.AddRow()
			row.WriteSlice(&[]interface{}{formattedDate, groupName, subj, topic, hours, lessonType}, -1)
		}

		// Send file to user
		w.Header().Set("Content-Disposition", "attachment; filename=stats.xlsx")
		file.Write(w)
	}
}

// ============================================================================
// PAGE HANDLERS - Lesson Management
// ============================================================================

// Handler for adding a new lesson
func addLessonHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" {
			// Get groups for selection
			groups, err := db.GetTeacherGroups(database, userInfo.ID)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
				return
			}

			// Get subjects for selection
			subjects, err := db.GetTeacherSubjects(database, userInfo.ID)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
				return
			}

			data := struct {
				User     db.UserInfo
				Groups   []string
				Subjects []string
			}{
				User:     userInfo,
				Groups:   groups,
				Subjects: subjects,
			}
			renderTemplate(w, tmpl, "add_lesson.html", data)
			return
		}

		// Process form submission
		group := r.FormValue("group")
		subject := r.FormValue("subject")
		topic := r.FormValue("topic")
		hours, _ := strconv.Atoi(r.FormValue("hours"))
		date := r.FormValue("date")
		lessonType := r.FormValue("type")

		// Validate inputs
		if group == "" || subject == "" || topic == "" || hours <= 0 || date == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		// Normalize lesson type
		if lessonType != "Лекция" && lessonType != "Лабораторная работа" && lessonType != "Практика" {
			lessonType = "Лекция"
		}

		// Insert lesson
		_, err = db.ExecuteQuery(database,
			"INSERT INTO lessons (teacher_id, group_name, subject, topic, hours, date, type) VALUES (?, ?, ?, ?, ?, ?, ?)",
			userInfo.ID, group, subject, topic, hours, date, lessonType)
		if err != nil {
			utils.HandleError(w, err, "Error adding lesson", http.StatusInternalServerError)
			return
		}

		db.LogAction(database, userInfo.ID, "Add Lesson",
			fmt.Sprintf("Added %s: %s, %s, %s, %d hours, %s", lessonType, subject, group, topic, hours, date))

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

// Handler for viewing and managing lessons by subject
func subjectLessonsHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		subject := r.URL.Query().Get("subject")
		if subject == "" {
			http.Error(w, "Subject not specified", http.StatusBadRequest)
			return
		}

		// Handle lesson deletion
		if r.Method == "POST" {
			lessonID := r.FormValue("lesson_id")
			if lessonID == "" {
				http.Error(w, "Lesson ID not specified", http.StatusBadRequest)
				return
			}

			// Get lesson details for logging
			var group, topic, lessonType, dateStr string
			var hours int
			err := database.QueryRow(`
				SELECT group_name, topic, hours, date, type 
				FROM lessons 
				WHERE id = ? AND teacher_id = ?`, lessonID, userInfo.ID).
				Scan(&group, &topic, &hours, &dateStr, &lessonType)
			if err != nil {
				http.Error(w, "Lesson not found", http.StatusNotFound)
				return
			}

			// Delete lesson
			_, err = db.ExecuteQuery(database, "DELETE FROM lessons WHERE id = ? AND teacher_id = ?", lessonID, userInfo.ID)
			if err != nil {
				utils.HandleError(w, err, "Error deleting lesson", http.StatusInternalServerError)
				return
			}

			formattedDate := utils.FormatDate(dateStr)
			db.LogAction(database, userInfo.ID, "Delete Lesson",
				fmt.Sprintf("Deleted lesson ID %s: %s, %s, %s, %d hours, %s, type: %s",
					lessonID, subject, group, topic, hours, formattedDate, lessonType))

			http.Redirect(w, r, fmt.Sprintf("/lessons/subject?subject=%s", subject), http.StatusSeeOther)
			return
		}

		// Get lessons for this subject using our DB function
		lessons, err := db.GetLessonsBySubject(database, userInfo.ID, subject)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving lessons", http.StatusInternalServerError)
			return
		}

		// Process lessons - format dates
		for i := range lessons {
			lessons[i].Date = utils.FormatDate(lessons[i].Date)
		}

		data := struct {
			User    db.UserInfo
			Subject string
			Lessons []db.LessonData
		}{
			User:    userInfo,
			Subject: subject,
			Lessons: lessons,
		}
		renderTemplate(w, tmpl, "subject_lessons.html", data)
	}
}

// Handler for editing a lesson
func editLessonHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		lessonID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.Error(w, "Invalid lesson ID", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" {
			// Get lesson details
			lesson, err := db.GetLessonByID(database, lessonID, userInfo.ID)
			if err != nil {
				http.Error(w, "Lesson not found", http.StatusNotFound)
				return
			}

			// Get groups for selection
			groups, err := db.GetTeacherGroups(database, userInfo.ID)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
				return
			}

			// Get subjects for selection
			subjects, err := db.GetTeacherSubjects(database, userInfo.ID)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
				return
			}

			data := struct {
				User     db.UserInfo
				Lesson   db.LessonData
				Groups   []string
				Subjects []string
			}{
				User:     userInfo,
				Lesson:   lesson,
				Groups:   groups,
				Subjects: subjects,
			}
			renderTemplate(w, tmpl, "edit_lesson.html", data)
			return
		}

		// Process form submission
		group := r.FormValue("group")
		subject := r.FormValue("subject")
		topic := r.FormValue("topic")
		hours, _ := strconv.Atoi(r.FormValue("hours"))
		date := r.FormValue("date")
		lessonType := r.FormValue("type")

		// Validate inputs
		if group == "" || subject == "" || topic == "" || hours <= 0 || date == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		// Normalize lesson type
		if lessonType != "Лекция" && lessonType != "Лабораторная работа" && lessonType != "Практика" {
			lessonType = "Лекция"
		}

		// Update lesson
		_, err = db.ExecuteQuery(database, `
			UPDATE lessons 
			SET group_name = ?, subject = ?, topic = ?, hours = ?, date = ?, type = ? 
			WHERE id = ? AND teacher_id = ?`,
			group, subject, topic, hours, date, lessonType, lessonID, userInfo.ID)
		if err != nil {
			utils.HandleError(w, err, "Error updating lesson", http.StatusInternalServerError)
			return
		}

		db.LogAction(database, userInfo.ID, "Edit Lesson",
			fmt.Sprintf("Edited lesson ID %d: %s, %s, %s, %d hours, %s, type: %s",
				lessonID, subject, group, topic, hours, date, lessonType))

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

// ============================================================================
// PAGE HANDLERS - Group Management
// ============================================================================

// Handler for viewing and managing groups
func groupsHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Handle group deletion
		if r.Method == "POST" {
			groupName := r.FormValue("group_name")
			if groupName == "" {
				http.Error(w, "Group name not specified", http.StatusBadRequest)
				return
			}

			// Transaction for deleting group data
			tx, err := database.Begin()
			if err != nil {
				utils.HandleError(w, err, "Error starting transaction", http.StatusInternalServerError)
				return
			}

			// Delete lessons for this group
			_, err = tx.Exec("DELETE FROM lessons WHERE teacher_id = ? AND group_name = ?", userInfo.ID, groupName)
			if err != nil {
				tx.Rollback()
				utils.HandleError(w, err, "Error deleting group lessons", http.StatusInternalServerError)
				return
			}

			// Delete students for this group
			_, err = tx.Exec("DELETE FROM students WHERE teacher_id = ? AND group_name = ?", userInfo.ID, groupName)
			if err != nil {
				tx.Rollback()
				utils.HandleError(w, err, "Error deleting group students", http.StatusInternalServerError)
				return
			}

			// Commit transaction
			if err := tx.Commit(); err != nil {
				utils.HandleError(w, err, "Error committing transaction", http.StatusInternalServerError)
				return
			}

			db.LogAction(database, userInfo.ID, "Delete Group",
				fmt.Sprintf("Deleted group %s with all lessons and students", groupName))

			http.Redirect(w, r, "/groups", http.StatusSeeOther)
			return
		}

		// Get groups for this teacher
		groups, err := db.GetGroupsByTeacher(database, userInfo.ID)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		data := struct {
			User   db.UserInfo
			Groups []db.GroupData
		}{
			User:   userInfo,
			Groups: groups,
		}
		renderTemplate(w, tmpl, "groups.html", data)
	}
}

// Handler for editing a group
func editGroupHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		groupName := vars["groupName"]

		// Handle form submissions
		if r.Method == "POST" {
			action := r.FormValue("action")
			switch action {
			case "upload":
				// Process file upload
				file, _, err := r.FormFile("student_list")
				if err != nil {
					http.Error(w, "Error uploading file", http.StatusBadRequest)
					return
				}
				defer file.Close()

				// Read student list from file
				scanner := bufio.NewScanner(file)
				studentsAdded := 0
				for scanner.Scan() {
					studentFIO := strings.TrimSpace(scanner.Text())
					if studentFIO != "" {
						_, err := db.ExecuteQuery(database,
							"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
							userInfo.ID, groupName, studentFIO)
						if err == nil {
							studentsAdded++
						}
					}
				}
				if err := scanner.Err(); err != nil {
					utils.HandleError(w, err, "Error reading file", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Upload Student List",
					fmt.Sprintf("Uploaded list of %d students to group %s", studentsAdded, groupName))

			case "delete":
				// Delete student
				studentID := r.FormValue("student_id")
				_, err := db.ExecuteQuery(database, "DELETE FROM students WHERE id = ? AND teacher_id = ?", studentID, userInfo.ID)
				if err != nil {
					utils.HandleError(w, err, "Error deleting student", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Delete Student",
					fmt.Sprintf("Deleted student from group %s (ID: %s)", groupName, studentID))

			case "update":
				// Update student name
				studentID := r.FormValue("student_id")
				newFIO := r.FormValue("new_fio")
				_, err := db.ExecuteQuery(database,
					"UPDATE students SET student_fio = ? WHERE id = ? AND teacher_id = ?",
					newFIO, studentID, userInfo.ID)
				if err != nil {
					utils.HandleError(w, err, "Error updating name", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Update Student Name",
					fmt.Sprintf("Updated student ID %s in group %s to %s", studentID, groupName, newFIO))

			case "move":
				// Move student to another group
				studentID := r.FormValue("student_id")
				newGroup := r.FormValue("new_group")
				_, err := db.ExecuteQuery(database,
					"UPDATE students SET group_name = ? WHERE id = ? AND teacher_id = ?",
					newGroup, studentID, userInfo.ID)
				if err != nil {
					utils.HandleError(w, err, "Error moving student", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Move Student",
					fmt.Sprintf("Moved student ID %s from group %s to %s", studentID, groupName, newGroup))

			case "add_student":
				// Add new student
				studentFIO := r.FormValue("student_fio")
				if studentFIO == "" {
					http.Error(w, "Student name not specified", http.StatusBadRequest)
					return
				}

				_, err := db.ExecuteQuery(database,
					"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
					userInfo.ID, groupName, studentFIO)
				if err != nil {
					utils.HandleError(w, err, "Error adding student", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Add Student",
					fmt.Sprintf("Added student %s to group %s", studentFIO, groupName))
			}

			http.Redirect(w, r, fmt.Sprintf("/groups/edit/%s", groupName), http.StatusSeeOther)
			return
		}

		// Get students in this group
		students, err := db.GetStudentsInGroup(database, userInfo.ID, groupName)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
			return
		}

		// Get all groups for move operation
		groups, err := db.GetTeacherGroups(database, userInfo.ID)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		data := struct {
			User      db.UserInfo
			GroupName string
			Students  []db.StudentData
			Groups    []string
		}{
			User:      userInfo,
			GroupName: groupName,
			Students:  students,
			Groups:    groups,
		}
		renderTemplate(w, tmpl, "edit_group.html", data)
	}
}

// Handler for adding a new group
func addGroupHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" {
			data := struct {
				User db.UserInfo
			}{
				User: userInfo,
			}
			renderTemplate(w, tmpl, "add_group.html", data)
			return
		}

		// Process form submission
		groupName := r.FormValue("group_name")
		if groupName == "" {
			http.Error(w, "Group name not specified", http.StatusBadRequest)
			return
		}

		// Check if group already exists
		var exists int
		err = database.QueryRow(`
			SELECT COUNT(*) 
			FROM (
				SELECT group_name FROM lessons WHERE teacher_id = ? 
				UNION 
				SELECT group_name FROM students WHERE teacher_id = ?
			) WHERE group_name = ?`,
			userInfo.ID, userInfo.ID, groupName).Scan(&exists)
		if err != nil {
			utils.HandleError(w, err, "Error checking group", http.StatusInternalServerError)
			return
		}
		if exists > 0 {
			http.Error(w, "Group with this name already exists", http.StatusBadRequest)
			return
		}

		// Process student list file if provided
		studentsAdded := false
		file, _, err := r.FormFile("student_list")
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				studentFIO := strings.TrimSpace(scanner.Text())
				if studentFIO != "" {
					_, err := db.ExecuteQuery(database,
						"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
						userInfo.ID, groupName, studentFIO)
					if err == nil {
						studentsAdded = true
					}
				}
			}
			if err := scanner.Err(); err != nil {
				utils.HandleError(w, err, "Error reading file", http.StatusInternalServerError)
				return
			}
		}

		// Process manually entered students
		r.ParseForm()
		studentFIOs := r.Form["student_fio"]
		studentCount := 0
		for _, studentFIO := range studentFIOs {
			studentFIO = strings.TrimSpace(studentFIO)
			if studentFIO != "" {
				_, err := db.ExecuteQuery(database,
					"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
					userInfo.ID, groupName, studentFIO)
				if err == nil {
					studentCount++
					studentsAdded = true
				}
			}
		}

		// Log action
		logMessage := fmt.Sprintf("Created group %s", groupName)
		if studentsAdded {
			logMessage += fmt.Sprintf(" with added students (from file: %v, manually: %d)", file != nil, studentCount)
		}
		db.LogAction(database, userInfo.ID, "Create Group", logMessage)

		http.Redirect(w, r, "/groups", http.StatusSeeOther)
	}
}

// ============================================================================
// PAGE HANDLERS - Attendance Management
// ============================================================================

// Handler for editing attendance records
func editAttendanceHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		lessonID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.Error(w, "Invalid lesson ID", http.StatusBadRequest)
			return
		}

		// Verify the lesson belongs to this teacher
		var lesson db.LessonData
		err = database.QueryRow(`
			SELECT l.id, l.group_name, l.subject, l.topic, l.date, l.type
			FROM lessons l
			WHERE l.id = ? AND l.teacher_id = ?`,
			lessonID, userInfo.ID).Scan(&lesson.ID, &lesson.Group, &lesson.Subject, &lesson.Topic, &lesson.Date, &lesson.Type)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Lesson not found or doesn't belong to you", http.StatusNotFound)
			} else {
				utils.HandleError(w, err, "Error retrieving lesson", http.StatusInternalServerError)
			}
			return
		}

		// Format the date for display
		lesson.Date = utils.FormatDate(lesson.Date)

		// Handle form submission
		if r.Method == "POST" {
			// Parse the form data
			if err := r.ParseForm(); err != nil {
				utils.HandleError(w, err, "Error parsing form data", http.StatusBadRequest)
				return
			}

			// Get attended student IDs
			attendedStudents := r.Form["attended"]
			attendedIDs := make([]int, 0)
			for _, idStr := range attendedStudents {
				id, err := strconv.Atoi(idStr)
				if err == nil {
					attendedIDs = append(attendedIDs, id)
				}
			}

			// Save attendance using our DB function
			err = db.SaveAttendance(database, lessonID, userInfo.ID, attendedIDs)
			if err != nil {
				utils.HandleError(w, err, "Error updating attendance", http.StatusInternalServerError)
				return
			}

			db.LogAction(database, userInfo.ID, "Edit Attendance",
				fmt.Sprintf("Updated attendance for lesson ID %d, group %s", lessonID, lesson.Group))

			http.Redirect(w, r, "/attendance", http.StatusSeeOther)
			return
		}

		// For GET requests, display the form
		// Get student attendance records using our DB function
		students, err := db.GetAttendanceForLesson(database, lessonID, userInfo.ID)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
			return
		}

		data := struct {
			User     db.UserInfo
			Lesson   db.LessonData
			Students []db.StudentAttendance
		}{
			User:     userInfo,
			Lesson:   lesson,
			Students: students,
		}
		renderTemplate(w, tmpl, "edit_attendance.html", data)
	}
}

// Handler for viewing and managing attendance records
func attendanceHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Handle attendance deletion
		if r.Method == "POST" {
			attendanceID := r.FormValue("attendance_id")
			if attendanceID == "" {
				http.Error(w, "Attendance ID not specified", http.StatusBadRequest)
				return
			}

			// Delete attendance records
			_, err := db.ExecuteQuery(database,
				"DELETE FROM attendance WHERE lesson_id = ? AND EXISTS (SELECT 1 FROM lessons WHERE id = ? AND teacher_id = ?)",
				attendanceID, attendanceID, userInfo.ID)
			if err != nil {
				utils.HandleError(w, err, "Error deleting attendance", http.StatusInternalServerError)
				return
			}

			db.LogAction(database, userInfo.ID, "Delete Attendance",
				fmt.Sprintf("Deleted attendance for lesson ID %s", attendanceID))

			http.Redirect(w, r, "/attendance", http.StatusSeeOther)
			return
		}

		// Get attendance records
		attendances, err := db.GetTeacherAttendanceRecords(database, userInfo.ID)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving attendance list", http.StatusInternalServerError)
			return
		}

		// Format dates
		for i := range attendances {
			attendances[i].Date = utils.FormatDate(attendances[i].Date)
		}

		data := struct {
			User        db.UserInfo
			Attendances []db.AttendanceData
		}{
			User:        userInfo,
			Attendances: attendances,
		}
		renderTemplate(w, tmpl, "attendance.html", data)
	}
}

// Handler for creating/recording attendance
func addAttendanceHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" {
			// Get subjects for selection
			subjects, err := db.GetTeacherSubjects(database, userInfo.ID)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
				return
			}

			data := struct {
				User     db.UserInfo
				Subjects []string
			}{
				User:     userInfo,
				Subjects: subjects,
			}
			renderTemplate(w, tmpl, "add_attendance.html", data)
			return
		}

		// Process form submission
		lessonID, _ := strconv.Atoi(r.FormValue("lesson_id"))
		if lessonID == 0 {
			http.Error(w, "Lesson not selected", http.StatusBadRequest)
			return
		}

		// Parse the form data
		if err := r.ParseForm(); err != nil {
			utils.HandleError(w, err, "Error parsing form data", http.StatusBadRequest)
			return
		}

		// Get attended student IDs
		attendedStudents := r.Form["attended"]
		attendedIDs := make([]int, 0)
		for _, idStr := range attendedStudents {
			id, err := strconv.Atoi(idStr)
			if err == nil {
				attendedIDs = append(attendedIDs, id)
			}
		}

		// Save attendance using our DB function
		err = db.SaveAttendance(database, lessonID, userInfo.ID, attendedIDs)
		if err != nil {
			utils.HandleError(w, err, "Error saving attendance", http.StatusInternalServerError)
			return
		}

		// Get group name for logging
		var groupName string
		database.QueryRow("SELECT group_name FROM lessons WHERE id = ?", lessonID).Scan(&groupName)

		db.LogAction(database, userInfo.ID, "Create Attendance",
			fmt.Sprintf("Added attendance for lesson ID %d, group %s", lessonID, groupName))

		http.Redirect(w, r, "/attendance", http.StatusSeeOther)
	}
}

// Handler for exporting attendance data to Excel
func exportAttendanceExcelHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		exportMode := r.URL.Query().Get("mode")
		if exportMode != "group" && exportMode != "lesson" {
			http.Error(w, "Invalid export mode. Use 'group' or 'lesson'", http.StatusBadRequest)
			return
		}

		// Create Excel file
		file := xlsx.NewFile()

		var err2 error
		if exportMode == "group" {
			err2 = db.ExportAttendanceByGroup(database, userInfo.ID, file)
		} else {
			err2 = db.ExportAttendanceByLesson(database, userInfo.ID, file)
		}

		if err2 != nil {
			utils.HandleError(w, err2, fmt.Sprintf("Error exporting attendance by %s", exportMode), http.StatusInternalServerError)
			return
		}

		db.LogAction(database, userInfo.ID, "Export Attendance",
			fmt.Sprintf("Exported attendance data in %s mode", exportMode))

		// Send file to user
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename=attendance.xlsx")
		file.Write(w)
	}
}

// ============================================================================
// API HANDLERS
// ============================================================================

// API endpoint for getting lessons by subject
func apiLessonsHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teacherID, err := strconv.Atoi(r.Header.Get("X-Teacher-ID"))
		if err != nil {
			http.Error(w, "Unauthorized access", http.StatusUnauthorized)
			return
		}

		subject := r.URL.Query().Get("subject")
		if subject == "" {
			http.Error(w, "Subject not specified", http.StatusBadRequest)
			return
		}

		// Get lessons for this subject
		rows, err := database.Query(
			"SELECT id, date, group_name FROM lessons WHERE teacher_id = ? AND subject = ? ORDER BY date",
			teacherID, subject)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving lessons", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Process lessons
		type LessonAPIData struct {
			ID        int    `json:"id"`
			Date      string `json:"date"`
			GroupName string `json:"group_name"`
		}
		var lessons []LessonAPIData
		for rows.Next() {
			var l LessonAPIData
			rows.Scan(&l.ID, &l.Date, &l.GroupName)
			lessons = append(lessons, l)
		}

		utils.RespondJSON(w, lessons)
	}
}

// API endpoint for getting students by lesson
func apiStudentsHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teacherID, err := strconv.Atoi(r.Header.Get("X-Teacher-ID"))
		if err != nil {
			http.Error(w, "Unauthorized access", http.StatusUnauthorized)
			return
		}

		lessonID, err := strconv.Atoi(r.URL.Query().Get("lesson_id"))
		if err != nil || lessonID == 0 {
			http.Error(w, "Lesson not specified", http.StatusBadRequest)
			return
		}

		// Get group for this lesson
		var groupName string
		err = database.QueryRow("SELECT group_name FROM lessons WHERE id = ? AND teacher_id = ?",
			lessonID, teacherID).Scan(&groupName)
		if err != nil {
			http.Error(w, "Lesson not found", http.StatusBadRequest)
			return
		}

		// Get students in this group
		students, err := db.GetStudentsInGroup(database, teacherID, groupName)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
			return
		}

		type StudentAPIData struct {
			ID  int    `json:"id"`
			FIO string `json:"fio"`
		}
		var apiStudents []StudentAPIData
		for _, s := range students {
			apiStudents = append(apiStudents, StudentAPIData{ID: s.ID, FIO: s.FIO})
		}

		utils.RespondJSON(w, apiStudents)
	}
}

// ============================================================================
// PAGE HANDLERS - Admin Functionality
// ============================================================================

// Handler for admin dashboard
func adminDashboardHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Get filter parameters
		teacherIDFilter := r.URL.Query().Get("teacher_id")
		subjectFilter := r.URL.Query().Get("subject")
		startDate := r.URL.Query().Get("start_date")
		endDate := r.URL.Query().Get("end_date")
		sortBy := r.URL.Query().Get("sort_by")

		// Build query for teacher statistics
		query := `
			SELECT u.id, u.fio, COUNT(l.id) as lessons, SUM(l.hours) as hours
			FROM users u
			LEFT JOIN lessons l ON u.id = l.teacher_id
		`
		var args []interface{}
		if teacherIDFilter != "" {
			query += " WHERE u.id = ?"
			args = append(args, teacherIDFilter)
		}
		query += " GROUP BY u.id, u.fio"

		// Apply sorting
		if sortBy == "fio" || sortBy == "lessons" || sortBy == "hours" {
			query += " ORDER BY " + sortBy
		} else {
			query += " ORDER BY u.fio"
		}

		// Execute query
		rows, err := database.Query(query, args...)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving statistics", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Process teacher statistics
		var teachers []db.TeacherStats
		for rows.Next() {
			var t db.TeacherStats
			rows.Scan(&t.ID, &t.FIO, &t.Lessons, &t.Hours)
			t.Subjects = make(map[string]int)

			// Get subject details for each teacher
			subjQuery := "SELECT subject, COUNT(*) FROM lessons WHERE teacher_id = ?"
			subjArgs := []interface{}{t.ID}

			if subjectFilter != "" {
				subjQuery += " AND subject = ?"
				subjArgs = append(subjArgs, subjectFilter)
			}
			if startDate != "" && endDate != "" {
				subjQuery += " AND date BETWEEN ? AND ?"
				subjArgs = append(subjArgs, startDate, endDate)
			}
			subjQuery += " GROUP BY subject"

			subjRows, err := database.Query(subjQuery, subjArgs...)
			if err != nil {
				continue
			}
			for subjRows.Next() {
				var subject string
				var count int
				subjRows.Scan(&subject, &count)
				t.Subjects[subject] = count
			}
			subjRows.Close()
			teachers = append(teachers, t)
		}

		// Handle Excel export request
		if r.URL.Query().Get("export") == "true" {
			file := xlsx.NewFile()
			sheet, _ := file.AddSheet("Teacher Statistics")
			header := sheet.AddRow()
			header.WriteSlice(&[]string{"Name", "Subject", "Group", "Topic", "Hours", "Type", "Date"}, -1)

			// Build export query
			exportQuery := `
				SELECT u.fio, l.subject, l.group_name, l.topic, l.hours, l.type, l.date
				FROM users u
				JOIN lessons l ON u.id = l.teacher_id
			`
			exportArgs := []interface{}{}
			whereAdded := false

			if teacherIDFilter != "" {
				exportQuery += " WHERE u.id = ?"
				exportArgs = append(exportArgs, teacherIDFilter)
				whereAdded = true
			}
			if subjectFilter != "" {
				if !whereAdded {
					exportQuery += " WHERE"
					whereAdded = true
				} else {
					exportQuery += " AND"
				}
				exportQuery += " l.subject = ?"
				exportArgs = append(exportArgs, subjectFilter)
			}
			if startDate != "" && endDate != "" {
				if !whereAdded {
					exportQuery += " WHERE"
				} else {
					exportQuery += " AND"
				}
				exportQuery += " l.date BETWEEN ? AND ?"
				exportArgs = append(exportArgs, startDate, endDate)
			}
			exportQuery += " ORDER BY u.fio, l.date"

			// Execute export query
			rows, err := database.Query(exportQuery, exportArgs...)
			if err != nil {
				utils.HandleError(w, err, "Error exporting data", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			// Populate Excel file
			for rows.Next() {
				var fio, subject, group, topic, lessonType, dateStr string
				var hours int
				rows.Scan(&fio, &subject, &group, &topic, &hours, &lessonType, &dateStr)

				formattedDate := utils.FormatDate(dateStr)

				row := sheet.AddRow()
				row.WriteSlice(&[]interface{}{fio, subject, group, topic, hours, lessonType, formattedDate}, -1)
			}

			// Send file to user
			w.Header().Set("Content-Disposition", "attachment; filename=teacher_stats.xlsx")
			file.Write(w)
			return
		}

		// Get teachers for filter dropdown
		teacherRows, err := database.Query("SELECT id, fio FROM users ORDER BY fio")
		if err != nil {
			utils.HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
			return
		}
		defer teacherRows.Close()

		var teacherList []struct {
			ID  int
			FIO string
		}
		for teacherRows.Next() {
			var t struct {
				ID  int
				FIO string
			}
			teacherRows.Scan(&t.ID, &t.FIO)
			teacherList = append(teacherList, t)
		}

		// Get subjects for filter dropdown
		subjectRows, err := database.Query("SELECT DISTINCT subject FROM lessons ORDER BY subject")
		if err != nil {
			utils.HandleError(w, err, "Error retrieving subject list", http.StatusInternalServerError)
			return
		}
		defer subjectRows.Close()

		var subjectList []string
		for subjectRows.Next() {
			var subject string
			subjectRows.Scan(&subject)
			subjectList = append(subjectList, subject)
		}

		data := struct {
			User        db.UserInfo
			Teachers    []db.TeacherStats
			TeacherList []struct {
				ID  int
				FIO string
			}
			SubjectList []string
			Filter      struct {
				TeacherID string
				Subject   string
				StartDate string
				EndDate   string
				SortBy    string
			}
		}{
			User:        userInfo,
			Teachers:    teachers,
			TeacherList: teacherList,
			SubjectList: subjectList,
			Filter: struct {
				TeacherID string
				Subject   string
				StartDate string
				EndDate   string
				SortBy    string
			}{
				TeacherID: teacherIDFilter,
				Subject:   subjectFilter,
				StartDate: startDate,
				EndDate:   endDate,
				SortBy:    sortBy,
			},
		}
		renderTemplate(w, tmpl, "admin.html", data)
	}
}

// Handler for user management (admin)
func adminUsersHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Handle user management actions
		if r.Method == "POST" {
			action := r.FormValue("action")
			userID := r.FormValue("user_id")

			switch action {
			case "delete":
				// Get user name for logging
				var fio string
				database.QueryRow("SELECT fio FROM users WHERE id = ?", userID).Scan(&fio)

				// Delete user data
				tx, err := database.Begin()
				if err != nil {
					utils.HandleError(w, err, "Error starting transaction", http.StatusInternalServerError)
					return
				}

				// Delete lessons first (foreign key constraint)
				_, err = tx.Exec("DELETE FROM lessons WHERE teacher_id = ?", userID)
				if err != nil {
					tx.Rollback()
					utils.HandleError(w, err, "Error deleting user lessons", http.StatusInternalServerError)
					return
				}

				// Delete user
				_, err = tx.Exec("DELETE FROM users WHERE id = ?", userID)
				if err != nil {
					tx.Rollback()
					utils.HandleError(w, err, "Error deleting user", http.StatusInternalServerError)
					return
				}

				err = tx.Commit()
				if err != nil {
					utils.HandleError(w, err, "Error committing transaction", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Delete User", fmt.Sprintf("Deleted user: %s (ID: %s)", fio, userID))

			case "update_role":
				// Update user role
				newRole := r.FormValue("role")
				if newRole != "teacher" && newRole != "admin" {
					http.Error(w, "Invalid role", http.StatusBadRequest)
					return
				}

				var fio string
				database.QueryRow("SELECT fio FROM users WHERE id = ?", userID).Scan(&fio)

				_, err := db.ExecuteQuery(database, "UPDATE users SET role = ? WHERE id = ?", newRole, userID)
				if err != nil {
					utils.HandleError(w, err, "Error updating role", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Change Role",
					fmt.Sprintf("Changed role of user %s (ID: %s) to %s", fio, userID, newRole))
			}

			http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
			return
		}

		// Get user list
		rows, err := database.Query("SELECT id, fio, login, role FROM users ORDER BY fio")
		if err != nil {
			utils.HandleError(w, err, "Error retrieving user list", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Process users
		type UserData struct {
			ID    int
			FIO   string
			Login string
			Role  string
		}
		var users []UserData
		for rows.Next() {
			var u UserData
			rows.Scan(&u.ID, &u.FIO, &u.Login, &u.Role)
			users = append(users, u)
		}

		data := struct {
			User  db.UserInfo
			Users []UserData
		}{
			User:  userInfo,
			Users: users,
		}
		renderTemplate(w, tmpl, "admin_users.html", data)
	}
}

// Handler for viewing system logs (admin)
func adminLogsHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Get pagination parameters
		pageStr := r.URL.Query().Get("page")
		page := 1 // Default to first page
		if pageStr != "" {
			pageNum, err := strconv.Atoi(pageStr)
			if err == nil && pageNum > 0 {
				page = pageNum
			}
		}

		const entriesPerPage = 10
		offset := (page - 1) * entriesPerPage

		// Get total count for pagination
		var totalEntries int
		err = database.QueryRow("SELECT COUNT(*) FROM logs").Scan(&totalEntries)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving log count", http.StatusInternalServerError)
			return
		}

		totalPages := (totalEntries + entriesPerPage - 1) / entriesPerPage // Ceiling division

		// Get system logs with pagination
		rows, err := database.Query(`
			SELECT l.id, u.fio, l.action, l.details, l.timestamp
			FROM logs l
			JOIN users u ON l.user_id = u.id
			ORDER BY l.timestamp DESC
			LIMIT ? OFFSET ?
		`, entriesPerPage, offset)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving logs", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Process logs
		type LogEntry struct {
			ID        int
			UserFIO   string
			Action    string
			Details   string
			Timestamp string
		}
		var logs []LogEntry
		for rows.Next() {
			var l LogEntry
			rows.Scan(&l.ID, &l.UserFIO, &l.Action, &l.Details, &l.Timestamp)
			logs = append(logs, l)
		}

		data := struct {
			User       db.UserInfo
			Logs       []LogEntry
			Pagination struct {
				CurrentPage int
				TotalPages  int
				HasPrev     bool
				HasNext     bool
				PrevPage    int
				NextPage    int
			}
		}{
			User: userInfo,
			Logs: logs,
			Pagination: struct {
				CurrentPage int
				TotalPages  int
				HasPrev     bool
				HasNext     bool
				PrevPage    int
				NextPage    int
			}{
				CurrentPage: page,
				TotalPages:  totalPages,
				HasPrev:     page > 1,
				HasNext:     page < totalPages,
				PrevPage:    page - 1,
				NextPage:    page + 1,
			},
		}
		renderTemplate(w, tmpl, "admin_logs.html", data)
	}
}

// Handler for admin management of teacher groups
func adminTeacherGroupsHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Get all teachers for dropdown
		teacherRows, err := database.Query("SELECT id, fio FROM users WHERE role = 'teacher' ORDER BY fio")
		if err != nil {
			utils.HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
			return
		}
		defer teacherRows.Close()

		var teacherList []struct {
			ID  int
			FIO string
		}
		for teacherRows.Next() {
			var t struct {
				ID  int
				FIO string
			}
			teacherRows.Scan(&t.ID, &t.FIO)
			teacherList = append(teacherList, t)
		}

		// Check if a teacher is selected
		selectedTeacherID := r.URL.Query().Get("teacher_id")
		var selectedTeacher struct {
			ID  int
			FIO string
		}

		// If a teacher is selected, get their groups
		var groups []struct {
			Name         string
			StudentCount int
			Students     []struct {
				ID  int
				FIO string
			}
		}

		if selectedTeacherID != "" {
			teacherIDInt, err := strconv.Atoi(selectedTeacherID)
			if err != nil {
				http.Error(w, "Invalid teacher ID", http.StatusBadRequest)
				return
			}

			// Get teacher info
			err = database.QueryRow("SELECT id, fio FROM users WHERE id = ?", teacherIDInt).
				Scan(&selectedTeacher.ID, &selectedTeacher.FIO)
			if err != nil {
				http.Error(w, "Teacher not found", http.StatusNotFound)
				return
			}

			// Get groups for this teacher
			groupRows, err := database.Query(`
				SELECT DISTINCT group_name 
				FROM (
					SELECT group_name FROM lessons WHERE teacher_id = ? 
					UNION 
					SELECT group_name FROM students WHERE teacher_id = ?
				) ORDER BY group_name`,
				teacherIDInt, teacherIDInt)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
				return
			}
			defer groupRows.Close()

			for groupRows.Next() {
				var groupName string
				groupRows.Scan(&groupName)

				var group struct {
					Name         string
					StudentCount int
					Students     []struct {
						ID  int
						FIO string
					}
				}
				group.Name = groupName

				// Count students in this group
				err := database.QueryRow("SELECT COUNT(*) FROM students WHERE teacher_id = ? AND group_name = ?",
					teacherIDInt, groupName).Scan(&group.StudentCount)
				if err != nil {
					utils.HandleError(w, err, "Error counting students", http.StatusInternalServerError)
					return
				}

				// Get students in this group
				studentRows, err := database.Query(
					"SELECT id, student_fio FROM students WHERE teacher_id = ? AND group_name = ? ORDER BY student_fio",
					teacherIDInt, groupName)
				if err != nil {
					utils.HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
					return
				}

				for studentRows.Next() {
					var student struct {
						ID  int
						FIO string
					}
					studentRows.Scan(&student.ID, &student.FIO)
					group.Students = append(group.Students, student)
				}
				studentRows.Close()

				groups = append(groups, group)
			}
		}

		data := struct {
			User        db.UserInfo
			TeacherList []struct {
				ID  int
				FIO string
			}
			SelectedTeacherID string
			SelectedTeacher   struct {
				ID  int
				FIO string
			}
			Groups []struct {
				Name         string
				StudentCount int
				Students     []struct {
					ID  int
					FIO string
				}
			}
		}{
			User:              userInfo,
			TeacherList:       teacherList,
			SelectedTeacherID: selectedTeacherID,
			SelectedTeacher:   selectedTeacher,
			Groups:            groups,
		}
		renderTemplate(w, tmpl, "admin_teacher_groups.html", data)
	}
}

// Handler for admin to add a group to a teacher
func adminAddGroupHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		teacherID, err := strconv.Atoi(vars["teacherID"])
		if err != nil {
			http.Error(w, "Invalid teacher ID", http.StatusBadRequest)
			return
		}

		// Get teacher info
		var teacherFIO string
		err = database.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
		if err != nil {
			http.Error(w, "Teacher not found", http.StatusNotFound)
			return
		}

		if r.Method == "GET" {
			data := struct {
				User       db.UserInfo
				TeacherID  int
				TeacherFIO string
			}{
				User:       userInfo,
				TeacherID:  teacherID,
				TeacherFIO: teacherFIO,
			}
			renderTemplate(w, tmpl, "admin_add_group.html", data)
			return
		}

		// Process form submission
		groupName := r.FormValue("group_name")
		if groupName == "" {
			http.Error(w, "Group name not specified", http.StatusBadRequest)
			return
		}

		// Check if group already exists
		var exists int
		err = database.QueryRow(`
			SELECT COUNT(*) 
			FROM (
				SELECT group_name FROM lessons WHERE teacher_id = ? 
				UNION 
				SELECT group_name FROM students WHERE teacher_id = ?
			) WHERE group_name = ?`,
			teacherID, teacherID, groupName).Scan(&exists)
		if err != nil {
			utils.HandleError(w, err, "Error checking group", http.StatusInternalServerError)
			return
		}
		if exists > 0 {
			http.Error(w, "Group with this name already exists for this teacher", http.StatusBadRequest)
			return
		}

		// Process student list file if provided
		studentsAdded := false
		file, _, err := r.FormFile("student_list")
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				studentFIO := strings.TrimSpace(scanner.Text())
				if studentFIO != "" {
					_, err := db.ExecuteQuery(database,
						"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
						teacherID, groupName, studentFIO)
					if err == nil {
						studentsAdded = true
					}
				}
			}
			if err := scanner.Err(); err != nil {
				utils.HandleError(w, err, "Error reading file", http.StatusInternalServerError)
				return
			}
		}

		// Process manually entered students
		r.ParseForm()
		studentFIOs := r.Form["student_fio"]
		studentCount := 0
		for _, studentFIO := range studentFIOs {
			studentFIO = strings.TrimSpace(studentFIO)
			if studentFIO != "" {
				_, err := db.ExecuteQuery(database,
					"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
					teacherID, groupName, studentFIO)
				if err == nil {
					studentCount++
					studentsAdded = true
				}
			}
		}

		// Log action
		logMessage := fmt.Sprintf("Created group %s for teacher ID %d", groupName, teacherID)
		if studentsAdded {
			logMessage += fmt.Sprintf(" with added students (from file: %v, manually: %d)", file != nil, studentCount)
		}
		db.LogAction(database, userInfo.ID, "Admin Create Group", logMessage)

		http.Redirect(w, r, fmt.Sprintf("/admin/groups?teacher_id=%d", teacherID), http.StatusSeeOther)
	}
}

// Handler for admin to edit a teacher's group
func adminEditGroupHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		teacherID, err := strconv.Atoi(vars["teacherID"])
		if err != nil {
			http.Error(w, "Invalid teacher ID", http.StatusBadRequest)
			return
		}
		groupName := vars["groupName"]

		// Get teacher info
		var teacherFIO string
		err = database.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
		if err != nil {
			http.Error(w, "Teacher not found", http.StatusNotFound)
			return
		}

		// Handle form submissions
		if r.Method == "POST" {
			// Parse form to ensure we have access to all form values
			if err := r.ParseForm(); err != nil {
				utils.HandleError(w, err, "Error parsing form", http.StatusBadRequest)
				return
			}

			action := r.FormValue("action")
			switch action {
			case "upload":
				// Process file upload
				file, _, err := r.FormFile("student_list")
				if err != nil {
					http.Error(w, "Error uploading file", http.StatusBadRequest)
					return
				}
				defer file.Close()

				// Read student list from file
				scanner := bufio.NewScanner(file)
				studentsAdded := 0
				for scanner.Scan() {
					studentFIO := strings.TrimSpace(scanner.Text())
					if studentFIO != "" {
						_, err := db.ExecuteQuery(database,
							"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
							teacherID, groupName, studentFIO)
						if err == nil {
							studentsAdded++
						}
					}
				}
				if err := scanner.Err(); err != nil {
					utils.HandleError(w, err, "Error reading file", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Admin Upload Student List",
					fmt.Sprintf("Uploaded list of %d students to group %s for teacher ID %d", studentsAdded, groupName, teacherID))

			case "delete":
				// Delete student
				studentID := r.FormValue("student_id")
				_, err := db.ExecuteQuery(database, "DELETE FROM students WHERE id = ? AND teacher_id = ?", studentID, teacherID)
				if err != nil {
					utils.HandleError(w, err, "Error deleting student", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Admin Delete Student",
					fmt.Sprintf("Deleted student from group %s for teacher ID %d (Student ID: %s)", groupName, teacherID, studentID))

			case "update":
				// Update student name
				studentID := r.FormValue("student_id")
				newFIO := r.FormValue("new_fio")
				_, err := db.ExecuteQuery(database,
					"UPDATE students SET student_fio = ? WHERE id = ? AND teacher_id = ?",
					newFIO, studentID, teacherID)
				if err != nil {
					utils.HandleError(w, err, "Error updating name", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Admin Update Student Name",
					fmt.Sprintf("Updated student ID %s in group %s for teacher ID %d to %s", studentID, groupName, teacherID, newFIO))

			case "move":
				// Move student to another group
				studentID := r.FormValue("student_id")
				newGroup := r.FormValue("new_group")
				_, err := db.ExecuteQuery(database,
					"UPDATE students SET group_name = ? WHERE id = ? AND teacher_id = ?",
					newGroup, studentID, teacherID)
				if err != nil {
					utils.HandleError(w, err, "Error moving student", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Admin Move Student",
					fmt.Sprintf("Moved student ID %s from group %s to %s for teacher ID %d", studentID, groupName, newGroup, teacherID))

			case "add_student":
				// Add new student
				studentFIO := r.FormValue("student_fio")
				if studentFIO == "" {
					http.Error(w, "Student name not specified", http.StatusBadRequest)
					return
				}

				_, err := db.ExecuteQuery(database,
					"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
					teacherID, groupName, studentFIO)
				if err != nil {
					utils.HandleError(w, err, "Error adding student", http.StatusInternalServerError)
					return
				}

				db.LogAction(database, userInfo.ID, "Admin Add Student",
					fmt.Sprintf("Added student %s to group %s for teacher ID %d", studentFIO, groupName, teacherID))
			}

			http.Redirect(w, r, fmt.Sprintf("/admin/groups/edit/%d/%s", teacherID, groupName), http.StatusSeeOther)
			return
		}

		// Get students in this group
		students, err := db.GetStudentsInGroup(database, teacherID, groupName)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
			return
		}

		// Get all groups for move operation
		groupRows, err := database.Query(`
			SELECT DISTINCT group_name 
			FROM (
				SELECT group_name FROM lessons WHERE teacher_id = ? 
				UNION 
				SELECT group_name FROM students WHERE teacher_id = ?
			) ORDER BY group_name`, teacherID, teacherID)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}
		defer groupRows.Close()

		var groups []string
		for groupRows.Next() {
			var g string
			groupRows.Scan(&g)
			groups = append(groups, g)
		}

		data := struct {
			User       db.UserInfo
			TeacherID  int
			TeacherFIO string
			GroupName  string
			Students   []db.StudentData
			Groups     []string
		}{
			User:       userInfo,
			TeacherID:  teacherID,
			TeacherFIO: teacherFIO,
			GroupName:  groupName,
			Students:   students,
			Groups:     groups,
		}
		renderTemplate(w, tmpl, "admin_edit_group.html", data)
	}
}

// ============================================================================
// PAGE HANDLERS - Admin Attendance Management
// ============================================================================

// Handler for viewing and managing attendance from admin panel
func adminAttendanceHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Handle attendance deletion
		if r.Method == "POST" {
			attendanceID := r.FormValue("attendance_id")
			if attendanceID == "" {
				http.Error(w, "Attendance ID not specified", http.StatusBadRequest)
				return
			}

			// Delete attendance records
			_, err := db.ExecuteQuery(database,
				"DELETE FROM attendance WHERE lesson_id = ?", attendanceID)
			if err != nil {
				utils.HandleError(w, err, "Error deleting attendance", http.StatusInternalServerError)
				return
			}

			db.LogAction(database, userInfo.ID, "Admin Delete Attendance",
				fmt.Sprintf("Deleted attendance records for lesson ID %s", attendanceID))

			http.Redirect(w, r, "/admin/attendance", http.StatusSeeOther)
			return
		}

		// Get filter parameters
		teacherIDParam := r.URL.Query().Get("teacher_id")
		groupParam := r.URL.Query().Get("group")
		subjectParam := r.URL.Query().Get("subject")

		// Get all teachers for dropdown
		teacherRows, err := database.Query("SELECT id, fio FROM users ORDER BY fio")
		if err != nil {
			utils.HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
			return
		}
		defer teacherRows.Close()

		var teacherList []struct {
			ID  int
			FIO string
		}
		for teacherRows.Next() {
			var t struct {
				ID  int
				FIO string
			}
			teacherRows.Scan(&t.ID, &t.FIO)
			teacherList = append(teacherList, t)
		}

		// Initialize template data
		data := struct {
			User        db.UserInfo
			TeacherList []struct {
				ID  int
				FIO string
			}
			SelectedTeacherID string
			SelectedGroup     string
			SelectedSubject   string
			Groups            []string
			Subjects          []string
			AttendanceData    []db.AttendanceData
		}{
			User:              userInfo,
			TeacherList:       teacherList,
			SelectedTeacherID: teacherIDParam,
			SelectedGroup:     groupParam,
			SelectedSubject:   subjectParam,
		}

		// If a teacher is selected, get their groups and subjects
		if teacherIDParam != "" {
			teacherID, err := strconv.Atoi(teacherIDParam)
			if err != nil {
				http.Error(w, "Invalid teacher ID", http.StatusBadRequest)
				return
			}

			// Get groups for this teacher
			groups, err := db.GetTeacherGroups(database, teacherID)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
				return
			}
			data.Groups = groups

			// Get subjects for this teacher
			subjects, err := db.GetTeacherSubjects(database, teacherID)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
				return
			}
			data.Subjects = subjects

			// Build query for attendance data
			query := `
				SELECT l.id, l.date, l.subject, l.group_name, 
					(SELECT COUNT(*) FROM students s WHERE s.group_name = l.group_name AND s.teacher_id = l.teacher_id) as total_students,
					(SELECT COUNT(*) FROM attendance a WHERE a.lesson_id = l.id AND a.attended = 1) as attended_students
				FROM lessons l
				WHERE l.teacher_id = ? AND EXISTS (SELECT 1 FROM attendance a WHERE a.lesson_id = l.id)
			`
			args := []interface{}{teacherID}

			// Apply group filter if provided
			if groupParam != "" {
				query += " AND l.group_name = ?"
				args = append(args, groupParam)
			}

			// Apply subject filter if provided
			if subjectParam != "" {
				query += " AND l.subject = ?"
				args = append(args, subjectParam)
			}

			query += " ORDER BY l.date DESC"

			// Execute query
			rows, err := database.Query(query, args...)
			if err != nil {
				utils.HandleError(w, err, "Error retrieving attendance list", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			// Process attendance records
			var attendances []db.AttendanceData
			for rows.Next() {
				var a db.AttendanceData
				var dateStr string
				err := rows.Scan(&a.LessonID, &dateStr, &a.Subject, &a.GroupName, &a.TotalStudents, &a.AttendedStudents)
				if err != nil {
					utils.HandleError(w, err, "Error processing attendance data", http.StatusInternalServerError)
					return
				}
				a.Date = utils.FormatDate(dateStr)
				attendances = append(attendances, a)
			}
			data.AttendanceData = attendances
		}

		// Render template
		renderTemplate(w, tmpl, "admin_attendance.html", data)
	}
}

// Handler for admin to edit attendance
func adminEditAttendanceHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		lessonID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.Error(w, "Invalid lesson ID", http.StatusBadRequest)
			return
		}

		// Get lesson details
		var lesson struct {
			ID         int
			TeacherID  int
			Group      string
			Subject    string
			Topic      string
			Date       string
			Type       string
			TeacherFIO string
		}

		err = database.QueryRow(`
			SELECT l.id, l.teacher_id, l.group_name, l.subject, l.topic, l.date, l.type, u.fio
			FROM lessons l
			JOIN users u ON l.teacher_id = u.id
			WHERE l.id = ?`,
			lessonID).Scan(&lesson.ID, &lesson.TeacherID, &lesson.Group, &lesson.Subject, &lesson.Topic, &lesson.Date, &lesson.Type, &lesson.TeacherFIO)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Lesson not found", http.StatusNotFound)
			} else {
				utils.HandleError(w, err, "Error retrieving lesson", http.StatusInternalServerError)
			}
			return
		}

		// Format date for display
		lesson.Date = utils.FormatDate(lesson.Date)

		// Handle form submission
		if r.Method == "POST" {
			// Parse form data
			if err := r.ParseForm(); err != nil {
				utils.HandleError(w, err, "Error parsing form data", http.StatusBadRequest)
				return
			}

			// Get attended student IDs
			attendedStudents := r.Form["attended"]
			attendedIDs := make([]int, 0)
			for _, idStr := range attendedStudents {
				id, err := strconv.Atoi(idStr)
				if err == nil {
					attendedIDs = append(attendedIDs, id)
				}
			}

			// Save attendance
			err = db.SaveAttendance(database, lessonID, lesson.TeacherID, attendedIDs)
			if err != nil {
				utils.HandleError(w, err, "Error updating attendance", http.StatusInternalServerError)
				return
			}

			db.LogAction(database, userInfo.ID, "Admin Edit Attendance",
				fmt.Sprintf("Updated attendance for lesson ID %d, group %s", lessonID, lesson.Group))

			http.Redirect(w, r, "/admin/attendance?teacher_id="+strconv.Itoa(lesson.TeacherID), http.StatusSeeOther)
			return
		}

		// For GET requests, display the form
		// Get student attendance records
		students, err := db.GetAttendanceForLesson(database, lessonID, lesson.TeacherID)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
			return
		}

		data := struct {
			User   db.UserInfo
			Lesson struct {
				ID      int
				Group   string
				Subject string
				Topic   string
				Date    string
				Type    string
			}
			TeacherID  int
			TeacherFIO string
			Students   []db.StudentAttendance
		}{
			User: userInfo,
			Lesson: struct {
				ID      int
				Group   string
				Subject string
				Topic   string
				Date    string
				Type    string
			}{
				ID:      lesson.ID,
				Group:   lesson.Group,
				Subject: lesson.Subject,
				Topic:   lesson.Topic,
				Date:    lesson.Date,
				Type:    lesson.Type,
			},
			TeacherID:  lesson.TeacherID,
			TeacherFIO: lesson.TeacherFIO,
			Students:   students,
		}
		renderTemplate(w, tmpl, "admin_edit_attendance.html", data)
	}
}

// Handler for viewing attendance details
func adminViewAttendanceHandler(database *sql.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		lessonID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.Error(w, "Invalid lesson ID", http.StatusBadRequest)
			return
		}

		// Get lesson details
		var lesson struct {
			ID         int
			TeacherID  int
			Group      string
			Subject    string
			Topic      string
			Date       string
			Type       string
			TeacherFIO string
		}

		err = database.QueryRow(`
			SELECT l.id, l.teacher_id, l.group_name, l.subject, l.topic, l.date, l.type, u.fio
			FROM lessons l
			JOIN users u ON l.teacher_id = u.id
			WHERE l.id = ?`,
			lessonID).Scan(&lesson.ID, &lesson.TeacherID, &lesson.Group, &lesson.Subject, &lesson.Topic, &lesson.Date, &lesson.Type, &lesson.TeacherFIO)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Lesson not found", http.StatusNotFound)
			} else {
				utils.HandleError(w, err, "Error retrieving lesson", http.StatusInternalServerError)
			}
			return
		}

		// Format date for display
		lesson.Date = utils.FormatDate(lesson.Date)

		// Get student attendance records
		students, err := db.GetAttendanceForLesson(database, lessonID, lesson.TeacherID)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
			return
		}

		// Count present students
		totalStudents := len(students)
		presentStudents := 0
		for _, s := range students {
			if s.Attended {
				presentStudents++
			}
		}

		// Calculate attendance percentage
		attendancePercent := 0.0
		if totalStudents > 0 {
			attendancePercent = float64(presentStudents) / float64(totalStudents) * 100
		}

		data := struct {
			User   db.UserInfo
			Lesson struct {
				ID      int
				Group   string
				Subject string
				Topic   string
				Date    string
				Type    string
			}
			TeacherID         int
			TeacherFIO        string
			Students          []db.StudentAttendance
			TotalStudents     int
			PresentStudents   int
			AttendancePercent float64
		}{
			User: userInfo,
			Lesson: struct {
				ID      int
				Group   string
				Subject string
				Topic   string
				Date    string
				Type    string
			}{
				ID:      lesson.ID,
				Group:   lesson.Group,
				Subject: lesson.Subject,
				Topic:   lesson.Topic,
				Date:    lesson.Date,
				Type:    lesson.Type,
			},
			TeacherID:         lesson.TeacherID,
			TeacherFIO:        lesson.TeacherFIO,
			Students:          students,
			TotalStudents:     totalStudents,
			PresentStudents:   presentStudents,
			AttendancePercent: attendancePercent,
		}

		renderTemplate(w, tmpl, "admin_view_attendance.html", data)
	}
}

// Handler for exporting attendance data
func adminExportAttendanceHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := db.GetUserInfo(database, r, store, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Get filter parameters
		teacherIDParam := r.URL.Query().Get("teacher_id")
		groupParam := r.URL.Query().Get("group")
		subjectParam := r.URL.Query().Get("subject")

		if teacherIDParam == "" {
			http.Error(w, "Teacher ID is required", http.StatusBadRequest)
			return
		}

		teacherID, err := strconv.Atoi(teacherIDParam)
		if err != nil {
			http.Error(w, "Invalid teacher ID", http.StatusBadRequest)
			return
		}

		// Get teacher name
		var teacherFIO string
		err = database.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
		if err != nil {
			http.Error(w, "Teacher not found", http.StatusNotFound)
			return
		}

		// Create Excel file
		file := xlsx.NewFile()
		sheet, _ := file.AddSheet("Attendance Data")

		// Add headers
		header := sheet.AddRow()
		header.WriteSlice(&[]string{"Дата", "Предмет", "Группа", "Тема", "Студент", "Присутствие"}, -1)

		// Build query for attendance data
		query := `
			SELECT l.date, l.subject, l.group_name, l.topic, s.student_fio, a.attended
			FROM lessons l
			JOIN attendance a ON l.id = a.lesson_id
			JOIN students s ON a.student_id = s.id
			WHERE l.teacher_id = ?
		`
		args := []interface{}{teacherID}

		// Apply filters
		if groupParam != "" {
			query += " AND l.group_name = ?"
			args = append(args, groupParam)
		}
		if subjectParam != "" {
			query += " AND l.subject = ?"
			args = append(args, subjectParam)
		}

		query += " ORDER BY l.date DESC, l.group_name, s.student_fio"

		// Execute query
		rows, err := database.Query(query, args...)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving attendance data", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Add data rows
		for rows.Next() {
			var dateStr, subject, group, topic, studentFIO string
			var attended int

			err := rows.Scan(&dateStr, &subject, &group, &topic, &studentFIO, &attended)
			if err != nil {
				utils.HandleError(w, err, "Error processing attendance data", http.StatusInternalServerError)
				return
			}

			formattedDate := utils.FormatDate(dateStr)
			attendanceStatus := "Отсутствовал"
			if attended == 1 {
				attendanceStatus = "Присутствовал"
			}

			dataRow := sheet.AddRow()
			dataRow.WriteSlice(&[]interface{}{formattedDate, subject, group, topic, studentFIO, attendanceStatus}, -1)
		}

		// Create summary sheet
		summarySheet, _ := file.AddSheet("Summary")
		summaryHeader := summarySheet.AddRow()
		summaryHeader.WriteSlice(&[]string{"Дата", "Предмет", "Группа", "Тема", "Всего студентов", "Присутствовало", "Процент посещаемости"}, -1)

		// Get summary data
		summaryQuery := `
			SELECT l.date, l.subject, l.group_name, l.topic, 
				(SELECT COUNT(*) FROM students s WHERE s.group_name = l.group_name AND s.teacher_id = l.teacher_id) as total_students,
				(SELECT COUNT(*) FROM attendance a WHERE a.lesson_id = l.id AND a.attended = 1) as attended_students
			FROM lessons l
			WHERE l.teacher_id = ? AND EXISTS (SELECT 1 FROM attendance a WHERE a.lesson_id = l.id)
		`
		summaryArgs := []interface{}{teacherID}

		// Apply filters
		if groupParam != "" {
			summaryQuery += " AND l.group_name = ?"
			summaryArgs = append(summaryArgs, groupParam)
		}
		if subjectParam != "" {
			summaryQuery += " AND l.subject = ?"
			summaryArgs = append(summaryArgs, subjectParam)
		}

		summaryQuery += " ORDER BY l.date DESC"

		// Execute summary query
		summaryRows, err := database.Query(summaryQuery, summaryArgs...)
		if err != nil {
			utils.HandleError(w, err, "Error retrieving summary data", http.StatusInternalServerError)
			return
		}
		defer summaryRows.Close()

		// Add summary rows
		for summaryRows.Next() {
			var dateStr, subject, group, topic string
			var totalStudents, attendedStudents int

			err := summaryRows.Scan(&dateStr, &subject, &group, &topic, &totalStudents, &attendedStudents)
			if err != nil {
				utils.HandleError(w, err, "Error processing summary data", http.StatusInternalServerError)
				return
			}

			formattedDate := utils.FormatDate(dateStr)
			attendancePercent := 0.0
			if totalStudents > 0 {
				attendancePercent = float64(attendedStudents) / float64(totalStudents) * 100
			}

			summaryRow := summarySheet.AddRow()
			summaryRow.WriteSlice(&[]interface{}{
				formattedDate,
				subject,
				group,
				topic,
				totalStudents,
				attendedStudents,
				fmt.Sprintf("%.1f%%", attendancePercent),
			}, -1)
		}

		db.LogAction(database, userInfo.ID, "Admin Export Attendance",
			fmt.Sprintf("Exported attendance data for teacher %s (ID: %d)", teacherFIO, teacherID))

		// Send file to user
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=attendance_%s.xlsx", strings.ReplaceAll(teacherFIO, " ", "_")))
		file.Write(w)
	}
}

// ============================================================================
// MAIN FUNCTION
// ============================================================================

func main() {
	database := db.InitDB()
	defer database.Close()

	// Create a new template with the function map
	tmpl := template.New("")

	// Register the functions
	tmpl = tmpl.Funcs(createTemplateHelperFunctions())

	// Parse the templates
	tmpl, err := tmpl.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	// Basic routes
	router.HandleFunc("/", indexHandler(database, tmpl)).Methods("GET")
	router.HandleFunc("/register", registerHandler(database, tmpl)).Methods("GET", "POST")
	router.HandleFunc("/login", loginHandler(database, tmpl)).Methods("GET", "POST")
	router.HandleFunc("/logout", logoutHandler).Methods("GET")
	router.HandleFunc("/dashboard", authMiddleware(dashboardHandler(database, tmpl))).Methods("GET")

	// Lesson management
	router.HandleFunc("/lesson/add", authMiddleware(addLessonHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/lessons/subject", authMiddleware(subjectLessonsHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/lesson/edit/{id}", authMiddleware(editLessonHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/export", authMiddleware(exportExcelHandler(database))).Methods("GET")

	// Group management
	router.HandleFunc("/groups", authMiddleware(groupsHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/groups/edit/{groupName}", authMiddleware(editGroupHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/groups/add", authMiddleware(addGroupHandler(database, tmpl))).Methods("GET", "POST")

	// Attendance management
	router.HandleFunc("/attendance/add", authMiddleware(addAttendanceHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/attendance", authMiddleware(attendanceHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/attendance/edit/{id}", authMiddleware(editAttendanceHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/export/attendance", authMiddleware(exportAttendanceExcelHandler(database))).Methods("GET")

	// Admin routes
	router.HandleFunc("/admin", adminMiddleware(database, adminDashboardHandler(database, tmpl))).Methods("GET")
	router.HandleFunc("/admin/", adminMiddleware(database, adminDashboardHandler(database, tmpl))).Methods("GET")
	router.HandleFunc("/admin/users", adminMiddleware(database, adminUsersHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/admin/logs", adminMiddleware(database, adminLogsHandler(database, tmpl))).Methods("GET")
	router.HandleFunc("/admin/groups", adminMiddleware(database, adminTeacherGroupsHandler(database, tmpl))).Methods("GET")
	router.HandleFunc("/admin/groups/add/{teacherID}", adminMiddleware(database, adminAddGroupHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/admin/groups/edit/{teacherID}/{groupName}", adminMiddleware(database, adminEditGroupHandler(database, tmpl))).Methods("GET", "POST")

	// Admin attendance management routes
	router.HandleFunc("/admin/attendance", adminMiddleware(database, adminAttendanceHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/admin/attendance/view/{id}", adminMiddleware(database, adminViewAttendanceHandler(database, tmpl))).Methods("GET")
	router.HandleFunc("/admin/attendance/edit/{id}", adminMiddleware(database, adminEditAttendanceHandler(database, tmpl))).Methods("GET", "POST")
	router.HandleFunc("/admin/attendance/export", adminMiddleware(database, adminExportAttendanceHandler(database))).Methods("GET")
	router.HandleFunc("/admin/attendance/{teacherID}/{groupName}", adminMiddleware(database, adminAttendanceHandler(database, tmpl))).Methods("GET")

	// API endpoints
	router.HandleFunc("/api/lessons", apiLessonsHandler(database)).Methods("GET")
	router.HandleFunc("/api/students", apiStudentsHandler(database)).Methods("GET")

	log.Println("Server started on :8080")
	http.ListenAndServe(":8080", router)
}

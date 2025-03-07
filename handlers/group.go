package handlers

import (
	"bufio"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"TeacherJournal/config"
	"TeacherJournal/db"
)

// GroupHandler handles group-related routes
type GroupHandler struct {
	DB   *sql.DB
	Tmpl *template.Template
}

// NewGroupHandler creates a new GroupHandler
func NewGroupHandler(database *sql.DB, tmpl *template.Template) *GroupHandler {
	return &GroupHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// GroupsHandler handles viewing and managing groups
func (h *GroupHandler) GroupsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
		tx, err := h.DB.Begin()
		if err != nil {
			HandleError(w, err, "Error starting transaction", http.StatusInternalServerError)
			return
		}

		// Delete lessons for this group
		_, err = tx.Exec("DELETE FROM lessons WHERE teacher_id = ? AND group_name = ?", userInfo.ID, groupName)
		if err != nil {
			tx.Rollback()
			HandleError(w, err, "Error deleting group lessons", http.StatusInternalServerError)
			return
		}

		// Delete students for this group
		_, err = tx.Exec("DELETE FROM students WHERE teacher_id = ? AND group_name = ?", userInfo.ID, groupName)
		if err != nil {
			tx.Rollback()
			HandleError(w, err, "Error deleting group students", http.StatusInternalServerError)
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			HandleError(w, err, "Error committing transaction", http.StatusInternalServerError)
			return
		}

		db.LogAction(h.DB, userInfo.ID, "Delete Group",
			fmt.Sprintf("Deleted group %s with all lessons and students", groupName))

		http.Redirect(w, r, "/groups", http.StatusSeeOther)
		return
	}

	// Get groups for this teacher
	groups, err := db.GetGroupsByTeacher(h.DB, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	data := struct {
		User   db.UserInfo
		Groups []db.GroupData
	}{
		User:   userInfo,
		Groups: groups,
	}
	renderTemplate(w, h.Tmpl, "groups.html", data)
}

// EditGroupHandler handles editing a group
func (h *GroupHandler) EditGroupHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
					_, err := db.ExecuteQuery(h.DB,
						"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
						userInfo.ID, groupName, studentFIO)
					if err == nil {
						studentsAdded++
					}
				}
			}
			if err := scanner.Err(); err != nil {
				HandleError(w, err, "Error reading file", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Upload Student List",
				fmt.Sprintf("Uploaded list of %d students to group %s", studentsAdded, groupName))

		case "delete":
			// Delete student
			studentID := r.FormValue("student_id")
			_, err := db.ExecuteQuery(h.DB, "DELETE FROM students WHERE id = ? AND teacher_id = ?", studentID, userInfo.ID)
			if err != nil {
				HandleError(w, err, "Error deleting student", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Delete Student",
				fmt.Sprintf("Deleted student from group %s (ID: %s)", groupName, studentID))

		case "update":
			// Update student name
			studentID := r.FormValue("student_id")
			newFIO := r.FormValue("new_fio")
			_, err := db.ExecuteQuery(h.DB,
				"UPDATE students SET student_fio = ? WHERE id = ? AND teacher_id = ?",
				newFIO, studentID, userInfo.ID)
			if err != nil {
				HandleError(w, err, "Error updating name", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Update Student Name",
				fmt.Sprintf("Updated student ID %s in group %s to %s", studentID, groupName, newFIO))

		case "move":
			// Move student to another group
			studentID := r.FormValue("student_id")
			newGroup := r.FormValue("new_group")
			_, err := db.ExecuteQuery(h.DB,
				"UPDATE students SET group_name = ? WHERE id = ? AND teacher_id = ?",
				newGroup, studentID, userInfo.ID)
			if err != nil {
				HandleError(w, err, "Error moving student", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Move Student",
				fmt.Sprintf("Moved student ID %s from group %s to %s", studentID, groupName, newGroup))

		case "add_student":
			// Add new student
			studentFIO := r.FormValue("student_fio")
			if studentFIO == "" {
				http.Error(w, "Student name not specified", http.StatusBadRequest)
				return
			}

			_, err := db.ExecuteQuery(h.DB,
				"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
				userInfo.ID, groupName, studentFIO)
			if err != nil {
				HandleError(w, err, "Error adding student", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Add Student",
				fmt.Sprintf("Added student %s to group %s", studentFIO, groupName))
		}

		http.Redirect(w, r, fmt.Sprintf("/groups/edit/%s", groupName), http.StatusSeeOther)
		return
	}

	// Get students in this group
	students, err := db.GetStudentsInGroup(h.DB, userInfo.ID, groupName)
	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
		return
	}

	// Get all groups for move operation
	groups, err := db.GetTeacherGroups(h.DB, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
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
	renderTemplate(w, h.Tmpl, "edit_group.html", data)
}

// AddGroupHandler handles adding a new group
func (h *GroupHandler) AddGroupHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
		renderTemplate(w, h.Tmpl, "add_group.html", data)
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
	err = h.DB.QueryRow(`
		SELECT COUNT(*) 
		FROM (
			SELECT group_name FROM lessons WHERE teacher_id = ? 
			UNION 
			SELECT group_name FROM students WHERE teacher_id = ?
		) WHERE group_name = ?`,
		userInfo.ID, userInfo.ID, groupName).Scan(&exists)
	if err != nil {
		HandleError(w, err, "Error checking group", http.StatusInternalServerError)
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
				_, err := db.ExecuteQuery(h.DB,
					"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
					userInfo.ID, groupName, studentFIO)
				if err == nil {
					studentsAdded = true
				}
			}
		}
		if err := scanner.Err(); err != nil {
			HandleError(w, err, "Error reading file", http.StatusInternalServerError)
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
			_, err := db.ExecuteQuery(h.DB,
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
	db.LogAction(h.DB, userInfo.ID, "Create Group", logMessage)

	http.Redirect(w, r, "/groups", http.StatusSeeOther)
}

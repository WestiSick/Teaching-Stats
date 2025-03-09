package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/config"
	"bufio"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// GroupHandler handles group-related routes
type GroupHandler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

// NewGroupHandler creates a new GroupHandler
func NewGroupHandler(database *gorm.DB, tmpl *template.Template) *GroupHandler {
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
		err := h.DB.Transaction(func(tx *gorm.DB) error {
			// Delete lessons for this group
			if err := tx.Where("teacher_id = ? AND group_name = ?", userInfo.ID, groupName).Delete(&models.Lesson{}).Error; err != nil {
				return err
			}

			// Delete students for this group
			if err := tx.Where("teacher_id = ? AND group_name = ?", userInfo.ID, groupName).Delete(&models.Student{}).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			HandleError(w, err, "Error deleting group data", http.StatusInternalServerError)
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
					// Insert new student
					student := models.Student{
						TeacherID:  userInfo.ID,
						GroupName:  groupName,
						StudentFIO: studentFIO,
					}

					if err := h.DB.Create(&student).Error; err == nil {
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
			studentID, _ := strconv.Atoi(r.FormValue("student_id"))

			if err := h.DB.Where("id = ? AND teacher_id = ?", studentID, userInfo.ID).Delete(&models.Student{}).Error; err != nil {
				HandleError(w, err, "Error deleting student", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Delete Student",
				fmt.Sprintf("Deleted student from group %s (ID: %d)", groupName, studentID))

		case "update":
			// Update student name
			studentID, _ := strconv.Atoi(r.FormValue("student_id"))
			newFIO := r.FormValue("new_fio")

			if err := h.DB.Model(&models.Student{}).
				Where("id = ? AND teacher_id = ?", studentID, userInfo.ID).
				Update("student_fio", newFIO).Error; err != nil {
				HandleError(w, err, "Error updating name", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Update Student Name",
				fmt.Sprintf("Updated student ID %d in group %s to %s", studentID, groupName, newFIO))

		case "move":
			// Move student to another group
			studentID, _ := strconv.Atoi(r.FormValue("student_id"))
			newGroup := r.FormValue("new_group")

			if err := h.DB.Model(&models.Student{}).
				Where("id = ? AND teacher_id = ?", studentID, userInfo.ID).
				Update("group_name", newGroup).Error; err != nil {
				HandleError(w, err, "Error moving student", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Move Student",
				fmt.Sprintf("Moved student ID %d from group %s to %s", studentID, groupName, newGroup))

		case "add_student":
			// Add new student
			studentFIO := r.FormValue("student_fio")
			if studentFIO == "" {
				http.Error(w, "Student name not specified", http.StatusBadRequest)
				return
			}

			// Create new student
			student := models.Student{
				TeacherID:  userInfo.ID,
				GroupName:  groupName,
				StudentFIO: studentFIO,
			}

			if err := h.DB.Create(&student).Error; err != nil {
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
	var count int64
	h.DB.Raw(`
		SELECT COUNT(*) 
		FROM (
			SELECT group_name FROM lessons WHERE teacher_id = ? 
			UNION 
			SELECT group_name FROM students WHERE teacher_id = ?
		) AS combined_groups
		WHERE group_name = ?`,
		userInfo.ID, userInfo.ID, groupName).Count(&count)

	if count > 0 {
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
				student := models.Student{
					TeacherID:  userInfo.ID,
					GroupName:  groupName,
					StudentFIO: studentFIO,
				}

				if err := h.DB.Create(&student).Error; err == nil {
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
			student := models.Student{
				TeacherID:  userInfo.ID,
				GroupName:  groupName,
				StudentFIO: studentFIO,
			}

			if err := h.DB.Create(&student).Error; err == nil {
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

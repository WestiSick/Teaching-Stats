package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/app/dashboard/utils"
	"TeacherJournal/config"
	"bufio"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/tealeg/xlsx"
	"gorm.io/gorm"
)

// AdminHandler handles admin-related routes
type AdminHandler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

// NewAdminHandler creates a new AdminHandler
func NewAdminHandler(database *gorm.DB, tmpl *template.Template) *AdminHandler {
	return &AdminHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// AdminDashboardHandler handles the admin dashboard
func (h *AdminHandler) AdminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	teacherQuery := h.DB.Model(&models.User{}).
		Select("users.id, users.fio, COUNT(lessons.id) as lessons, COALESCE(SUM(lessons.hours), 0) as hours").
		Joins("LEFT JOIN lessons ON users.id = lessons.teacher_id")

	if teacherIDFilter != "" {
		teacherQuery = teacherQuery.Where("users.id = ?", teacherIDFilter)
	}

	teacherQuery = teacherQuery.Group("users.id, users.fio")

	// Apply sorting
	if sortBy == "fio" {
		teacherQuery = teacherQuery.Order("users.fio")
	} else if sortBy == "lessons" {
		teacherQuery = teacherQuery.Order("lessons DESC")
	} else if sortBy == "hours" {
		teacherQuery = teacherQuery.Order("hours DESC")
	} else {
		teacherQuery = teacherQuery.Order("users.fio")
	}

	// Execute query
	var teacherStats []struct {
		ID      int
		FIO     string
		Lessons int
		Hours   int
	}
	if err := teacherQuery.Find(&teacherStats).Error; err != nil {
		HandleError(w, err, "Error retrieving statistics", http.StatusInternalServerError)
		return
	}

	// Process teacher statistics
	var teachers []db.TeacherStats
	for _, t := range teacherStats {
		teacher := db.TeacherStats{
			ID:       t.ID,
			FIO:      t.FIO,
			Lessons:  t.Lessons,
			Hours:    t.Hours,
			Subjects: make(map[string]int),
		}

		// Get subject details for each teacher
		var subjectCounts []struct {
			Subject string
			Count   int
		}

		subjectQuery := h.DB.Model(&models.Lesson{}).
			Select("subject, COUNT(*) as count").
			Where("teacher_id = ?", t.ID)

		if subjectFilter != "" {
			subjectQuery = subjectQuery.Where("subject = ?", subjectFilter)
		}

		if startDate != "" && endDate != "" {
			subjectQuery = subjectQuery.Where("date BETWEEN ? AND ?", startDate, endDate)
		}

		subjectQuery = subjectQuery.Group("subject")

		if err := subjectQuery.Find(&subjectCounts).Error; err != nil {
			// Just continue if we can't get subject data
			continue
		}

		for _, sc := range subjectCounts {
			teacher.Subjects[sc.Subject] = sc.Count
		}

		teachers = append(teachers, teacher)
	}

	// Handle Excel export request
	if r.URL.Query().Get("export") == "true" {
		file := xlsx.NewFile()
		sheet, _ := file.AddSheet("Teacher Statistics")
		header := sheet.AddRow()
		header.WriteSlice(&[]string{"Name", "Subject", "Group", "Topic", "Hours", "Type", "Date"}, -1)

		// Build export query
		exportQuery := h.DB.Model(&models.Lesson{}).
			Select("users.fio, lessons.subject, lessons.group_name, lessons.topic, lessons.hours, lessons.type, lessons.date").
			Joins("JOIN users ON users.id = lessons.teacher_id")

		if teacherIDFilter != "" {
			exportQuery = exportQuery.Where("users.id = ?", teacherIDFilter)
		}

		if subjectFilter != "" {
			exportQuery = exportQuery.Where("lessons.subject = ?", subjectFilter)
		}

		if startDate != "" && endDate != "" {
			exportQuery = exportQuery.Where("lessons.date BETWEEN ? AND ?", startDate, endDate)
		}

		exportQuery = exportQuery.Order("users.fio, lessons.date")

		// Execute export query
		type ExportData struct {
			FIO       string
			Subject   string
			GroupName string
			Topic     string
			Hours     int
			Type      string
			Date      string
		}

		var exportData []ExportData
		if err := exportQuery.Find(&exportData).Error; err != nil {
			HandleError(w, err, "Error exporting data", http.StatusInternalServerError)
			return
		}

		// Populate Excel file
		for _, data := range exportData {
			formattedDate := utils.FormatDate(data.Date)

			row := sheet.AddRow()
			row.WriteSlice(&[]interface{}{
				data.FIO,
				data.Subject,
				data.GroupName,
				data.Topic,
				data.Hours,
				data.Type,
				formattedDate,
			}, -1)
		}

		// Send file to user
		w.Header().Set("Content-Disposition", "attachment; filename=teacher_stats.xlsx")
		file.Write(w)
		return
	}

	// Get teachers for filter dropdown
	var teacherList []struct {
		ID  int
		FIO string
	}
	if err := h.DB.Model(&models.User{}).
		Select("id, fio").
		Order("fio").
		Find(&teacherList).Error; err != nil {
		HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
		return
	}

	// Get subjects for filter dropdown
	var subjectList []string
	if err := h.DB.Model(&models.Lesson{}).
		Distinct("subject").
		Order("subject").
		Pluck("subject", &subjectList).Error; err != nil {
		HandleError(w, err, "Error retrieving subject list", http.StatusInternalServerError)
		return
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
	renderTemplate(w, h.Tmpl, "admin.html", data)
}

// AdminUsersHandler handles user management (admin)
func (h *AdminHandler) AdminUsersHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
			if err := h.DB.Model(&models.User{}).
				Where("id = ?", userID).
				Pluck("fio", &fio).Error; err != nil {
				HandleError(w, err, "Error getting user info", http.StatusInternalServerError)
				return
			}

			// Delete user data using transaction
			err := h.DB.Transaction(func(tx *gorm.DB) error {
				// Delete lessons first (foreign key constraint)
				if err := tx.Where("teacher_id = ?", userID).Delete(&models.Lesson{}).Error; err != nil {
					return err
				}

				// Delete students
				if err := tx.Where("teacher_id = ?", userID).Delete(&models.Student{}).Error; err != nil {
					return err
				}

				// Delete user
				if err := tx.Where("id = ?", userID).Delete(&models.User{}).Error; err != nil {
					return err
				}

				return nil
			})

			if err != nil {
				HandleError(w, err, "Error deleting user", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Delete User", fmt.Sprintf("Deleted user: %s (ID: %s)", fio, userID))

		case "update_role":
			// Update user role
			newRole := r.FormValue("role")
			if newRole != "teacher" && newRole != "admin" && newRole != "free" {
				http.Error(w, "Invalid role", http.StatusBadRequest)
				return
			}

			var fio string
			if err := h.DB.Model(&models.User{}).
				Where("id = ?", userID).
				Pluck("fio", &fio).Error; err != nil {
				HandleError(w, err, "Error getting user info", http.StatusInternalServerError)
				return
			}

			if err := h.DB.Model(&models.User{}).
				Where("id = ?", userID).
				Update("role", newRole).Error; err != nil {
				HandleError(w, err, "Error updating role", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Change Role",
				fmt.Sprintf("Changed role of user %s (ID: %s) to %s", fio, userID, newRole))
		}

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	// Get user list
	var users []struct {
		ID    int
		FIO   string
		Login string
		Role  string
	}

	if err := h.DB.Model(&models.User{}).
		Select("id, fio, login, role").
		Order("fio").
		Find(&users).Error; err != nil {
		HandleError(w, err, "Error retrieving user list", http.StatusInternalServerError)
		return
	}

	data := struct {
		User  db.UserInfo
		Users []struct {
			ID    int
			FIO   string
			Login string
			Role  string
		}
	}{
		User:  userInfo,
		Users: users,
	}
	renderTemplate(w, h.Tmpl, "admin_users.html", data)
}

// AdminLogsHandler handles viewing system logs (admin)
func (h *AdminHandler) AdminLogsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get user filter parameters
	userIDFilter := r.URL.Query().Get("user_id")

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

	// Build the base query
	logsQuery := h.DB.Model(&models.Log{}).
		Select("logs.id, users.fio, logs.action, logs.details, logs.timestamp").
		Joins("JOIN users ON logs.user_id = users.id")

	// Add user filter if specified
	if userIDFilter != "" {
		logsQuery = logsQuery.Where("logs.user_id = ?", userIDFilter)
	}

	// Get total count for pagination
	var totalEntries int64
	countQuery := h.DB.Model(&models.Log{})
	if userIDFilter != "" {
		countQuery = countQuery.Where("user_id = ?", userIDFilter)
	}

	if err := countQuery.Count(&totalEntries).Error; err != nil {
		HandleError(w, err, "Error retrieving log count", http.StatusInternalServerError)
		return
	}

	totalPages := (int(totalEntries) + entriesPerPage - 1) / entriesPerPage // Ceiling division

	// Execute query with pagination
	var logs []struct {
		ID        int
		FIO       string
		Action    string
		Details   string
		Timestamp time.Time
	}

	if err := logsQuery.Order("logs.timestamp DESC").
		Limit(entriesPerPage).
		Offset(offset).
		Find(&logs).Error; err != nil {
		HandleError(w, err, "Error retrieving logs", http.StatusInternalServerError)
		return
	}

	// Format timestamps to strings
	type LogEntry struct {
		ID        int
		UserFIO   string
		Action    string
		Details   string
		Timestamp string
	}

	var formattedLogs []LogEntry
	for _, l := range logs {
		formattedLogs = append(formattedLogs, LogEntry{
			ID:        l.ID,
			UserFIO:   l.FIO,
			Action:    l.Action,
			Details:   l.Details,
			Timestamp: l.Timestamp.Format("2006-01-02 15:04:05"),
		})
	}

	// Get all users for the dropdown
	var userList []struct {
		ID  int
		FIO string
	}

	if err := h.DB.Model(&models.User{}).
		Select("id, fio").
		Order("fio").
		Find(&userList).Error; err != nil {
		HandleError(w, err, "Error retrieving user list", http.StatusInternalServerError)
		return
	}

	data := struct {
		User     db.UserInfo
		Logs     []LogEntry
		UserList []struct {
			ID  int
			FIO string
		}
		SelectedUserID string
		Pagination     struct {
			CurrentPage int
			TotalPages  int
			HasPrev     bool
			HasNext     bool
			PrevPage    int
			NextPage    int
		}
	}{
		User:           userInfo,
		Logs:           formattedLogs,
		UserList:       userList,
		SelectedUserID: userIDFilter,
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
	renderTemplate(w, h.Tmpl, "admin_logs.html", data)
}

// AdminTeacherGroupsHandler handles admin management of teacher groups
func (h *AdminHandler) AdminTeacherGroupsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get all teachers for dropdown
	var teacherList []struct {
		ID  int
		FIO string
	}

	if err := h.DB.Model(&models.User{}).
		Select("id, fio").
		Where("role = ?", "teacher").
		Order("fio").
		Find(&teacherList).Error; err != nil {
		HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
		return
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
		if err := h.DB.Model(&models.User{}).
			Select("id, fio").
			Where("id = ?", teacherIDInt).
			First(&selectedTeacher).Error; err != nil {
			http.Error(w, "Teacher not found", http.StatusNotFound)
			return
		}

		// Get groups for this teacher
		groupNames, err := db.GetTeacherGroups(h.DB, teacherIDInt)
		if err != nil {
			HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		for _, groupName := range groupNames {
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
			var count int64
			if err := h.DB.Model(&models.Student{}).
				Where("teacher_id = ? AND group_name = ?", teacherIDInt, groupName).
				Count(&count).Error; err != nil {
				HandleError(w, err, "Error counting students", http.StatusInternalServerError)
				return
			}
			group.StudentCount = int(count)

			// Get students in this group
			var students []struct {
				ID  int
				FIO string
			}

			if err := h.DB.Model(&models.Student{}).
				Select("id, student_fio as fio").
				Where("teacher_id = ? AND group_name = ?", teacherIDInt, groupName).
				Order("student_fio").
				Find(&students).Error; err != nil {
				HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
				return
			}

			group.Students = students
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
	renderTemplate(w, h.Tmpl, "admin_teacher_groups.html", data)
}

// AdminAddGroupHandler handles admin to add a group to a teacher
func (h *AdminHandler) AdminAddGroupHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	if err := h.DB.Model(&models.User{}).
		Where("id = ?", teacherID).
		Pluck("fio", &teacherFIO).Error; err != nil {
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
		renderTemplate(w, h.Tmpl, "admin_add_group.html", data)
		return
	}

	// Process form submission
	groupName := r.FormValue("group_name")
	if groupName == "" {
		http.Error(w, "Group name not specified", http.StatusBadRequest)
		return
	}

	// Check if group already exists
	var groupExists bool
	// Check in lessons table
	var lessonCount int64
	if err := h.DB.Model(&models.Lesson{}).
		Where("teacher_id = ? AND group_name = ?", teacherID, groupName).
		Count(&lessonCount).Error; err != nil {
		HandleError(w, err, "Error checking group in lessons", http.StatusInternalServerError)
		return
	}

	// Check in students table
	var studentCount int64
	if err := h.DB.Model(&models.Student{}).
		Where("teacher_id = ? AND group_name = ?", teacherID, groupName).
		Count(&studentCount).Error; err != nil {
		HandleError(w, err, "Error checking group in students", http.StatusInternalServerError)
		return
	}

	groupExists = lessonCount > 0 || studentCount > 0

	if groupExists {
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
				student := models.Student{
					TeacherID:  teacherID,
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
	studentCount = 0
	for _, studentFIO := range studentFIOs {
		studentFIO = strings.TrimSpace(studentFIO)
		if studentFIO != "" {
			student := models.Student{
				TeacherID:  teacherID,
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
	logMessage := fmt.Sprintf("Created group %s for teacher ID %d", groupName, teacherID)
	if studentsAdded {
		logMessage += fmt.Sprintf(" with added students (from file: %v, manually: %d)", file != nil, studentCount)
	}
	db.LogAction(h.DB, userInfo.ID, "Admin Create Group", logMessage)

	http.Redirect(w, r, fmt.Sprintf("/admin/groups?teacher_id=%d", teacherID), http.StatusSeeOther)
}

// AdminEditGroupHandler handles admin to edit a teacher's group
func (h *AdminHandler) AdminEditGroupHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	if err := h.DB.Model(&models.User{}).
		Where("id = ?", teacherID).
		Pluck("fio", &teacherFIO).Error; err != nil {
		http.Error(w, "Teacher not found", http.StatusNotFound)
		return
	}

	// Handle form submissions
	if r.Method == "POST" {
		// Parse form to ensure we have access to all form values
		if err := r.ParseForm(); err != nil {
			HandleError(w, err, "Error parsing form", http.StatusBadRequest)
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
					student := models.Student{
						TeacherID:  teacherID,
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

			db.LogAction(h.DB, userInfo.ID, "Admin Upload Student List",
				fmt.Sprintf("Uploaded list of %d students to group %s for teacher ID %d", studentsAdded, groupName, teacherID))

		case "delete":
			// Delete student
			studentID, _ := strconv.Atoi(r.FormValue("student_id"))
			if err := h.DB.Where("id = ? AND teacher_id = ?", studentID, teacherID).Delete(&models.Student{}).Error; err != nil {
				HandleError(w, err, "Error deleting student", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Admin Delete Student",
				fmt.Sprintf("Deleted student from group %s for teacher ID %d (Student ID: %s)", groupName, teacherID, r.FormValue("student_id")))

		case "update":
			// Update student name
			studentID, _ := strconv.Atoi(r.FormValue("student_id"))
			newFIO := r.FormValue("new_fio")
			if err := h.DB.Model(&models.Student{}).
				Where("id = ? AND teacher_id = ?", studentID, teacherID).
				Update("student_fio", newFIO).Error; err != nil {
				HandleError(w, err, "Error updating name", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Admin Update Student Name",
				fmt.Sprintf("Updated student ID %s in group %s for teacher ID %d to %s", r.FormValue("student_id"), groupName, teacherID, newFIO))

		case "move":
			// Move student to another group
			studentID, _ := strconv.Atoi(r.FormValue("student_id"))
			newGroup := r.FormValue("new_group")
			if err := h.DB.Model(&models.Student{}).
				Where("id = ? AND teacher_id = ?", studentID, teacherID).
				Update("group_name", newGroup).Error; err != nil {
				HandleError(w, err, "Error moving student", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Admin Move Student",
				fmt.Sprintf("Moved student ID %s from group %s to %s for teacher ID %d", r.FormValue("student_id"), groupName, newGroup, teacherID))

		case "add_student":
			// Add new student
			studentFIO := r.FormValue("student_fio")
			if studentFIO == "" {
				http.Error(w, "Student name not specified", http.StatusBadRequest)
				return
			}

			student := models.Student{
				TeacherID:  teacherID,
				GroupName:  groupName,
				StudentFIO: studentFIO,
			}

			if err := h.DB.Create(&student).Error; err != nil {
				HandleError(w, err, "Error adding student", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Admin Add Student",
				fmt.Sprintf("Added student %s to group %s for teacher ID %d", studentFIO, groupName, teacherID))
		}

		http.Redirect(w, r, fmt.Sprintf("/admin/groups/edit/%d/%s", teacherID, groupName), http.StatusSeeOther)
		return
	}

	// Get students in this group
	students, err := db.GetStudentsInGroup(h.DB, teacherID, groupName)
	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
		return
	}

	// Get all groups for move operation
	groups, err := db.GetTeacherGroups(h.DB, teacherID)
	if err != nil {
		HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
		return
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
	renderTemplate(w, h.Tmpl, "admin_edit_group.html", data)
}

// AdminAttendanceHandler handles viewing and managing attendance from admin panel
func (h *AdminHandler) AdminAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

		// Delete attendance records - make sure to use "attendances" (plural) table name
		if err := h.DB.Where("lesson_id = ?", attendanceID).Delete(&models.Attendance{}).Error; err != nil {
			HandleError(w, err, "Error deleting attendance", http.StatusInternalServerError)
			return
		}

		db.LogAction(h.DB, userInfo.ID, "Admin Delete Attendance",
			fmt.Sprintf("Deleted attendance records for lesson ID %s", attendanceID))

		http.Redirect(w, r, "/admin/attendance", http.StatusSeeOther)
		return
	}

	// Get filter parameters
	teacherIDParam := r.URL.Query().Get("teacher_id")
	groupParam := r.URL.Query().Get("group")
	subjectParam := r.URL.Query().Get("subject")

	// Get all teachers for dropdown
	var teacherList []struct {
		ID  int
		FIO string
	}
	if err := h.DB.Model(&models.User{}).
		Select("id, fio").
		Order("fio").
		Find(&teacherList).Error; err != nil {
		HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
		return
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
		groups, err := db.GetTeacherGroups(h.DB, teacherID)
		if err != nil {
			HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}
		data.Groups = groups

		// Get subjects for this teacher
		subjects, err := db.GetTeacherSubjects(h.DB, teacherID)
		if err != nil {
			HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
			return
		}
		data.Subjects = subjects

		// Build query for attendance data - use "attendances" (plural) table name
		query := h.DB.Table("lessons l").
			Select(`l.id as lesson_id, 
                    l.date, 
                    l.subject, 
                    l.group_name, 
                    (SELECT COUNT(*) FROM students s WHERE s.group_name = l.group_name AND s.teacher_id = l.teacher_id) as total_students,
                    (SELECT COUNT(*) FROM attendances a WHERE a.lesson_id = l.id AND a.attended = 1) as attended_students`).
			Where("l.teacher_id = ? AND EXISTS (SELECT 1 FROM attendances a WHERE a.lesson_id = l.id)", teacherID)

		// Apply group filter if provided
		if groupParam != "" {
			query = query.Where("l.group_name = ?", groupParam)
		}

		// Apply subject filter if provided
		if subjectParam != "" {
			query = query.Where("l.subject = ?", subjectParam)
		}

		query = query.Order("l.date DESC")

		// Execute query
		var attendances []db.AttendanceData
		if err := query.Find(&attendances).Error; err != nil {
			HandleError(w, err, "Error retrieving attendance list", http.StatusInternalServerError)
			return
		}

		// Format dates
		for i := range attendances {
			attendances[i].Date = utils.FormatDate(attendances[i].Date)
		}

		data.AttendanceData = attendances
	}

	// Render template
	renderTemplate(w, h.Tmpl, "admin_attendance.html", data)
}

// AdminEditAttendanceHandler handles admin to edit attendance
func (h *AdminHandler) AdminEditAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
		GroupName  string // Changed from Group to GroupName
		Subject    string
		Topic      string
		Date       string
		Type       string
		TeacherFIO string
	}

	err = h.DB.Table("lessons l").
		Select("l.id, l.teacher_id, l.group_name, l.subject, l.topic, l.date, l.type, u.fio as teacher_fio").
		Joins("JOIN users u ON l.teacher_id = u.id").
		Where("l.id = ?", lessonID).
		First(&lesson).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Lesson not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving lesson", http.StatusInternalServerError)
		}
		return
	}

	// Format date for display
	lesson.Date = utils.FormatDate(lesson.Date)

	// Handle form submission
	if r.Method == "POST" {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			HandleError(w, err, "Error parsing form data", http.StatusBadRequest)
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
		err = db.SaveAttendance(h.DB, lessonID, lesson.TeacherID, attendedIDs)
		if err != nil {
			HandleError(w, err, "Error updating attendance", http.StatusInternalServerError)
			return
		}

		db.LogAction(h.DB, userInfo.ID, "Admin Edit Attendance",
			fmt.Sprintf("Updated attendance for lesson ID %d, group %s", lessonID, lesson.GroupName)) // Changed from lesson.Group

		http.Redirect(w, r, "/admin/attendance?teacher_id="+strconv.Itoa(lesson.TeacherID), http.StatusSeeOther)
		return
	}

	// For GET requests, display the form
	// Get student attendance records
	students, err := db.GetAttendanceForLesson(h.DB, lessonID, lesson.TeacherID)
	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
		return
	}

	data := struct {
		User   db.UserInfo
		Lesson struct {
			ID        int
			GroupName string // Changed from Group to GroupName
			Subject   string
			Topic     string
			Date      string
			Type      string
		}
		TeacherID  int
		TeacherFIO string
		Students   []db.StudentAttendance
	}{
		User: userInfo,
		Lesson: struct {
			ID        int
			GroupName string // Changed from Group to GroupName
			Subject   string
			Topic     string
			Date      string
			Type      string
		}{
			ID:        lesson.ID,
			GroupName: lesson.GroupName, // Changed from lesson.Group
			Subject:   lesson.Subject,
			Topic:     lesson.Topic,
			Date:      lesson.Date,
			Type:      lesson.Type,
		},
		TeacherID:  lesson.TeacherID,
		TeacherFIO: lesson.TeacherFIO,
		Students:   students,
	}
	renderTemplate(w, h.Tmpl, "admin_edit_attendance.html", data)
}

// AdminViewAttendanceHandler handles viewing attendance details
func (h *AdminHandler) AdminViewAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
		GroupName  string // Changed from Group to GroupName
		Subject    string
		Topic      string
		Date       string
		Type       string
		TeacherFIO string
	}

	err = h.DB.Table("lessons l").
		Select("l.id, l.teacher_id, l.group_name, l.subject, l.topic, l.date, l.type, u.fio as teacher_fio").
		Joins("JOIN users u ON l.teacher_id = u.id").
		Where("l.id = ?", lessonID).
		First(&lesson).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Lesson not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving lesson", http.StatusInternalServerError)
		}
		return
	}

	// Format date for display
	lesson.Date = utils.FormatDate(lesson.Date)

	// Get student attendance records
	students, err := db.GetAttendanceForLesson(h.DB, lessonID, lesson.TeacherID)
	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
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
			ID        int
			GroupName string // Changed from Group to GroupName
			Subject   string
			Topic     string
			Date      string
			Type      string
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
			ID        int
			GroupName string // Changed from Group to GroupName
			Subject   string
			Topic     string
			Date      string
			Type      string
		}{
			ID:        lesson.ID,
			GroupName: lesson.GroupName, // Changed from lesson.Group
			Subject:   lesson.Subject,
			Topic:     lesson.Topic,
			Date:      lesson.Date,
			Type:      lesson.Type,
		},
		TeacherID:         lesson.TeacherID,
		TeacherFIO:        lesson.TeacherFIO,
		Students:          students,
		TotalStudents:     totalStudents,
		PresentStudents:   presentStudents,
		AttendancePercent: attendancePercent,
	}

	renderTemplate(w, h.Tmpl, "admin_view_attendance.html", data)
}

// AdminExportAttendanceHandler handles exporting attendance data
func (h *AdminHandler) AdminExportAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	if err := h.DB.Model(&models.User{}).
		Where("id = ?", teacherID).
		Pluck("fio", &teacherFIO).Error; err != nil {
		http.Error(w, "Teacher not found", http.StatusNotFound)
		return
	}

	// Create Excel file
	file := xlsx.NewFile()
	sheet, _ := file.AddSheet("Attendance Data")

	// Add headers
	header := sheet.AddRow()
	header.WriteSlice(&[]string{"Дата", "Предмет", "Группа", "Тема", "Студент", "Присутствие"}, -1)

	// Build query for attendance data - use "attendances" (plural) table name
	query := h.DB.Table("lessons l").
		Select("l.date, l.subject, l.group_name, l.topic, s.student_fio, a.attended").
		Joins("JOIN attendances a ON l.id = a.lesson_id").
		Joins("JOIN students s ON a.student_id = s.id").
		Where("l.teacher_id = ?", teacherID)

	// Apply filters
	if groupParam != "" {
		query = query.Where("l.group_name = ?", groupParam)
	}
	if subjectParam != "" {
		query = query.Where("l.subject = ?", subjectParam)
	}

	query = query.Order("l.date DESC, l.group_name, s.student_fio")

	// Execute query
	type AttendanceExportData struct {
		Date       string
		Subject    string
		GroupName  string
		Topic      string
		StudentFIO string
		Attended   int
	}

	var attendanceData []AttendanceExportData
	if err := query.Find(&attendanceData).Error; err != nil {
		HandleError(w, err, "Error retrieving attendance data", http.StatusInternalServerError)
		return
	}

	// Add data rows
	for _, data := range attendanceData {
		formattedDate := utils.FormatDate(data.Date)
		attendanceStatus := "Отсутствовал"
		if data.Attended == 1 {
			attendanceStatus = "Присутствовал"
		}

		dataRow := sheet.AddRow()
		dataRow.WriteSlice(&[]interface{}{
			formattedDate,
			data.Subject,
			data.GroupName,
			data.Topic,
			data.StudentFIO,
			attendanceStatus,
		}, -1)
	}

	// Create summary sheet
	summarySheet, _ := file.AddSheet("Summary")
	summaryHeader := summarySheet.AddRow()
	summaryHeader.WriteSlice(&[]string{"Дата", "Предмет", "Группа", "Тема", "Всего студентов", "Присутствовало", "Процент посещаемости"}, -1)

	// Get summary data - use "attendances" (plural) table name
	summaryQuery := h.DB.Table("lessons l").
		Select(`l.date, l.subject, l.group_name, l.topic, 
			(SELECT COUNT(*) FROM students s WHERE s.group_name = l.group_name AND s.teacher_id = l.teacher_id) as total_students,
			(SELECT COUNT(*) FROM attendances a WHERE a.lesson_id = l.id AND a.attended = 1) as attended_students`).
		Where("l.teacher_id = ? AND EXISTS (SELECT 1 FROM attendances a WHERE a.lesson_id = l.id)", teacherID)

	// Apply filters
	if groupParam != "" {
		summaryQuery = summaryQuery.Where("l.group_name = ?", groupParam)
	}
	if subjectParam != "" {
		summaryQuery = summaryQuery.Where("l.subject = ?", subjectParam)
	}

	summaryQuery = summaryQuery.Order("l.date DESC")

	// Execute summary query
	type SummaryData struct {
		Date             string
		Subject          string
		GroupName        string
		Topic            string
		TotalStudents    int
		AttendedStudents int
	}

	var summaryData []SummaryData
	if err := summaryQuery.Find(&summaryData).Error; err != nil {
		HandleError(w, err, "Error retrieving summary data", http.StatusInternalServerError)
		return
	}

	// Add summary rows
	for _, data := range summaryData {
		formattedDate := utils.FormatDate(data.Date)
		attendancePercent := 0.0
		if data.TotalStudents > 0 {
			attendancePercent = float64(data.AttendedStudents) / float64(data.TotalStudents) * 100
		}

		summaryRow := summarySheet.AddRow()
		summaryRow.WriteSlice(&[]interface{}{
			formattedDate,
			data.Subject,
			data.GroupName,
			data.Topic,
			data.TotalStudents,
			data.AttendedStudents,
			fmt.Sprintf("%.1f%%", attendancePercent),
		}, -1)
	}

	db.LogAction(h.DB, userInfo.ID, "Admin Export Attendance",
		fmt.Sprintf("Exported attendance data for teacher %s (ID: %d)", teacherFIO, teacherID))

	// Send file to user
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=attendance_%s.xlsx", strings.ReplaceAll(teacherFIO, " ", "_")))
	file.Write(w)
}

// AdminLabsHandler handles admin management of lab grades
func (h *AdminHandler) AdminLabsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get all teachers for dropdown
	var teacherList []struct {
		ID  int
		FIO string
	}
	if err := h.DB.Model(&models.User{}).
		Select("id, fio").
		Where("role = ?", "teacher").
		Order("fio").
		Find(&teacherList).Error; err != nil {
		HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
		return
	}

	// Check if a teacher is selected
	teacherIDParam := r.URL.Query().Get("teacher_id")
	var selectedTeacherID int
	if teacherIDParam != "" {
		selectedTeacherID, _ = strconv.Atoi(teacherIDParam)
	}

	// Initialize template data
	data := struct {
		User        db.UserInfo
		TeacherList []struct {
			ID  int
			FIO string
		}
		SelectedTeacherID int
		TeacherFIO        string
		SubjectGroups     []struct {
			Subject string
			Groups  []struct {
				GroupName    string
				TotalLabs    int
				GroupAverage float64
			}
		}
	}{
		User:              userInfo,
		TeacherList:       teacherList,
		SelectedTeacherID: selectedTeacherID,
	}

	// If a teacher is selected, get their subject-group data
	if selectedTeacherID > 0 {
		// Get teacher info
		if err := h.DB.Model(&models.User{}).
			Where("id = ?", selectedTeacherID).
			Pluck("fio", &data.TeacherFIO).Error; err != nil {
			HandleError(w, err, "Error retrieving teacher info", http.StatusInternalServerError)
			return
		}

		// Get subjects for this teacher
		subjects, err := db.GetTeacherSubjects(h.DB, selectedTeacherID)
		if err != nil {
			HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
			return
		}

		// For each subject, get the groups
		for _, subject := range subjects {
			sg := struct {
				Subject string
				Groups  []struct {
					GroupName    string
					TotalLabs    int
					GroupAverage float64
				}
			}{
				Subject: subject,
			}

			// Get groups for this subject
			var groupNames []string
			if err := h.DB.Model(&models.Lesson{}).
				Where("teacher_id = ? AND subject = ?", selectedTeacherID, subject).
				Distinct("group_name").
				Order("group_name").
				Pluck("group_name", &groupNames).Error; err != nil {
				// Just continue if we can't get group data
				continue
			}

			for _, groupName := range groupNames {
				// Get lab settings and summary
				var totalLabs int = 5        // Default if not set
				var groupAverage float64 = 0 // Default if not set or error

				// Try to get settings if they exist
				var labSettings models.LabSettings
				if err := h.DB.Where("teacher_id = ? AND subject = ? AND group_name = ?",
					selectedTeacherID, subject, groupName).First(&labSettings).Error; err == nil {
					totalLabs = labSettings.TotalLabs
				}

				// Get average grade
				err := h.DB.Model(&models.LabGrade{}).
					Select("AVG(grade)").
					Joins("JOIN students s ON lab_grades.student_id = s.id").
					Where("lab_grades.teacher_id = ? AND lab_grades.subject = ? AND s.group_name = ?",
						selectedTeacherID, subject, groupName).
					Scan(&groupAverage).Error

				if err != nil || groupAverage == 0 {
					groupAverage = 0 // Default if not set or error
				}

				sg.Groups = append(sg.Groups, struct {
					GroupName    string
					TotalLabs    int
					GroupAverage float64
				}{
					GroupName:    groupName,
					TotalLabs:    totalLabs,
					GroupAverage: groupAverage,
				})
			}

			if len(sg.Groups) > 0 {
				data.SubjectGroups = append(data.SubjectGroups, sg)
			}
		}
	}

	renderTemplate(w, h.Tmpl, "admin_labs.html", data)
}

// AdminViewLabGradesHandler handles viewing lab grades for a specific teacher, subject, and group
func (h *AdminHandler) AdminViewLabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	subject := vars["subject"]
	groupName := vars["group"]

	// Get teacher name
	var teacherFIO string
	if err := h.DB.Model(&models.User{}).
		Where("id = ?", teacherID).
		Pluck("fio", &teacherFIO).Error; err != nil {
		HandleError(w, err, "Error retrieving teacher info", http.StatusInternalServerError)
		return
	}

	// Get lab summary
	summary, err := db.GetGroupLabSummary(h.DB, teacherID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab grades", http.StatusInternalServerError)
		return
	}

	data := struct {
		User       db.UserInfo
		TeacherID  int
		TeacherFIO string
		Summary    db.GroupLabSummary
	}{
		User:       userInfo,
		TeacherID:  teacherID,
		TeacherFIO: teacherFIO,
		Summary:    summary,
	}

	renderTemplate(w, h.Tmpl, "admin_view_labs.html", data)
}

// AdminEditLabGradesHandler handles editing lab grades for a specific teacher, subject, and group
func (h *AdminHandler) AdminEditLabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	subject := vars["subject"]
	groupName := vars["group"]

	// Get teacher name
	var teacherFIO string
	if err := h.DB.Model(&models.User{}).
		Where("id = ?", teacherID).
		Pluck("fio", &teacherFIO).Error; err != nil {
		HandleError(w, err, "Error retrieving teacher info", http.StatusInternalServerError)
		return
	}

	// Handle form submission
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			HandleError(w, err, "Error parsing form", http.StatusBadRequest)
			return
		}

		action := r.FormValue("action")

		switch action {
		case "update_settings":
			totalLabs, err := strconv.Atoi(r.FormValue("total_labs"))
			if err != nil || totalLabs < 1 {
				http.Error(w, "Invalid number of labs", http.StatusBadRequest)
				return
			}

			settings := db.LabSettings{
				TeacherID: teacherID,
				GroupName: groupName,
				Subject:   subject,
				TotalLabs: totalLabs,
			}

			if err := db.SaveLabSettings(h.DB, settings); err != nil {
				HandleError(w, err, "Error saving lab settings", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Admin Update Lab Settings",
				fmt.Sprintf("Updated lab settings for teacher ID %d, %s, %s: %d labs",
					teacherID, subject, groupName, totalLabs))

		case "update_grades":
			// Process all grade inputs
			for key, values := range r.Form {
				// Keys should be in format grade_studentID_labNumber
				if len(values) == 0 {
					continue
				}

				var studentID, labNumber int

				// Parse the key to get studentID and labNumber
				if n, err := fmt.Sscanf(key, "grade_%d_%d", &studentID, &labNumber); err != nil || n != 2 {
					continue // Skip if not in expected format
				}

				// Parse the grade
				gradeStr := values[0]
				if gradeStr == "" {
					continue // Skip empty grades
				}

				grade, err := strconv.Atoi(gradeStr)
				if err != nil || grade < 1 || grade > 5 {
					continue // Skip invalid grades
				}

				// Save the grade - note we're using the teacher's ID, not the admin's
				if err := db.SaveLabGrade(h.DB, teacherID, studentID, subject, labNumber, grade); err != nil {
					HandleError(w, err, "Error saving grade", http.StatusInternalServerError)
					return
				}
			}

			db.LogAction(h.DB, userInfo.ID, "Admin Update Lab Grades",
				fmt.Sprintf("Updated lab grades for teacher ID %d, %s, %s",
					teacherID, subject, groupName))
		}

		// Redirect to prevent form resubmission
		http.Redirect(w, r, fmt.Sprintf("/admin/labs/edit/%d/%s/%s", teacherID, subject, groupName),
			http.StatusSeeOther)
		return
	}

	// Get lab summary
	summary, err := db.GetGroupLabSummary(h.DB, teacherID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab grades", http.StatusInternalServerError)
		return
	}

	data := struct {
		User       db.UserInfo
		TeacherID  int
		TeacherFIO string
		Summary    db.GroupLabSummary
	}{
		User:       userInfo,
		TeacherID:  teacherID,
		TeacherFIO: teacherFIO,
		Summary:    summary,
	}

	renderTemplate(w, h.Tmpl, "admin_edit_labs.html", data)
}

// AdminExportLabGradesHandler exports lab grades to Excel
func (h *AdminHandler) AdminExportLabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	subject := vars["subject"]
	groupName := vars["group"]

	// Get teacher name
	var teacherFIO string
	if err := h.DB.Model(&models.User{}).
		Where("id = ?", teacherID).
		Pluck("fio", &teacherFIO).Error; err != nil {
		HandleError(w, err, "Error retrieving teacher info", http.StatusInternalServerError)
		return
	}

	// Get lab summary
	summary, err := db.GetGroupLabSummary(h.DB, teacherID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab grades", http.StatusInternalServerError)
		return
	}

	// Create Excel file
	file := xlsx.NewFile()
	sheet, err := file.AddSheet(fmt.Sprintf("%s - %s", subject, groupName))
	if err != nil {
		HandleError(w, err, "Error creating Excel sheet", http.StatusInternalServerError)
		return
	}

	// Add header row
	header := sheet.AddRow()
	header.AddCell().SetString("ФИО")

	// Add columns for each lab
	for i := 1; i <= summary.TotalLabs; i++ {
		header.AddCell().SetString(fmt.Sprintf("%d", i))
	}

	// Add average column
	header.AddCell().SetString("Средний балл")

	// Add student rows
	for _, student := range summary.Students {
		row := sheet.AddRow()
		row.AddCell().SetString(student.StudentFIO)

		// Add grades for each lab
		for _, grade := range student.Grades {
			cell := row.AddCell()
			if grade > 0 {
				cell.SetInt(grade)
			}
			// If grade is 0, leave cell empty
		}

		// Add average
		if student.Average > 0 {
			row.AddCell().SetString(fmt.Sprintf("%.2f", student.Average))
		} else {
			row.AddCell().SetString("")
		}
	}

	// Add group average row
	if len(summary.Students) > 0 {
		avgRow := sheet.AddRow()
		avgRow.AddCell().SetString("Средний балл группы")

		// Empty cells for each lab
		for i := 0; i < summary.TotalLabs; i++ {
			avgRow.AddCell().SetString("")
		}

		// Group average in the last cell
		avgRow.AddCell().SetString(fmt.Sprintf("%.2f", summary.GroupAverage))
	}

	// Log the export action
	db.LogAction(h.DB, userInfo.ID, "Admin Export Lab Grades",
		fmt.Sprintf("Exported lab grades for teacher %s (ID: %d), subject %s, group %s",
			teacherFIO, teacherID, subject, groupName))

	// Set headers for file download
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=lab_grades_%s_%s.xlsx", subject, groupName))

	// Write the file to the response
	err = file.Write(w)
	if err != nil {
		HandleError(w, err, "Error writing Excel file", http.StatusInternalServerError)
		return
	}
}

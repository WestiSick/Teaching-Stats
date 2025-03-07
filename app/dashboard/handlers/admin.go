package handlers

import (
	db2 "TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/utils"
	"TeacherJournal/config"
	"bufio"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/tealeg/xlsx"
)

// AdminHandler handles admin-related routes
type AdminHandler struct {
	DB   *sql.DB
	Tmpl *template.Template
}

// NewAdminHandler creates a new AdminHandler
func NewAdminHandler(database *sql.DB, tmpl *template.Template) *AdminHandler {
	return &AdminHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// AdminDashboardHandler handles the admin dashboard
func (h *AdminHandler) AdminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	rows, err := h.DB.Query(query, args...)
	if err != nil {
		HandleError(w, err, "Error retrieving statistics", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Process teacher statistics
	var teachers []db2.TeacherStats
	for rows.Next() {
		var t db2.TeacherStats
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

		subjRows, err := h.DB.Query(subjQuery, subjArgs...)
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
		rows, err := h.DB.Query(exportQuery, exportArgs...)
		if err != nil {
			HandleError(w, err, "Error exporting data", http.StatusInternalServerError)
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
	teacherRows, err := h.DB.Query("SELECT id, fio FROM users ORDER BY fio")
	if err != nil {
		HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
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
	subjectRows, err := h.DB.Query("SELECT DISTINCT subject FROM lessons ORDER BY subject")
	if err != nil {
		HandleError(w, err, "Error retrieving subject list", http.StatusInternalServerError)
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
		User        db2.UserInfo
		Teachers    []db2.TeacherStats
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
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
			h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", userID).Scan(&fio)

			// Delete user data
			tx, err := h.DB.Begin()
			if err != nil {
				HandleError(w, err, "Error starting transaction", http.StatusInternalServerError)
				return
			}

			// Delete lessons first (foreign key constraint)
			_, err = tx.Exec("DELETE FROM lessons WHERE teacher_id = ?", userID)
			if err != nil {
				tx.Rollback()
				HandleError(w, err, "Error deleting user lessons", http.StatusInternalServerError)
				return
			}

			// Delete user
			_, err = tx.Exec("DELETE FROM users WHERE id = ?", userID)
			if err != nil {
				tx.Rollback()
				HandleError(w, err, "Error deleting user", http.StatusInternalServerError)
				return
			}

			err = tx.Commit()
			if err != nil {
				HandleError(w, err, "Error committing transaction", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Delete User", fmt.Sprintf("Deleted user: %s (ID: %s)", fio, userID))

		case "update_role":
			// Update user role
			newRole := r.FormValue("role")
			if newRole != "teacher" && newRole != "admin" {
				http.Error(w, "Invalid role", http.StatusBadRequest)
				return
			}

			var fio string
			h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", userID).Scan(&fio)

			_, err := db2.ExecuteQuery(h.DB, "UPDATE users SET role = ? WHERE id = ?", newRole, userID)
			if err != nil {
				HandleError(w, err, "Error updating role", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Change Role",
				fmt.Sprintf("Changed role of user %s (ID: %s) to %s", fio, userID, newRole))
		}

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	// Get user list
	rows, err := h.DB.Query("SELECT id, fio, login, role FROM users ORDER BY fio")
	if err != nil {
		HandleError(w, err, "Error retrieving user list", http.StatusInternalServerError)
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
		User  db2.UserInfo
		Users []UserData
	}{
		User:  userInfo,
		Users: users,
	}
	renderTemplate(w, h.Tmpl, "admin_users.html", data)
}

// AdminLogsHandler handles viewing system logs (admin)
func (h *AdminHandler) AdminLogsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

	// Build the base query with WHERE clause if user filter is specified
	countQuery := "SELECT COUNT(*) FROM logs l"
	logsQuery := `
		SELECT l.id, u.fio, l.action, l.details, l.timestamp
		FROM logs l
		JOIN users u ON l.user_id = u.id
	`

	var args []interface{}
	if userIDFilter != "" {
		countQuery += " WHERE l.user_id = ?"
		logsQuery += " WHERE l.user_id = ?"
		args = append(args, userIDFilter)
	}

	// Append ordering and limit
	logsQuery += " ORDER BY l.timestamp DESC LIMIT ? OFFSET ?"

	// Get total count for pagination
	var totalEntries int
	if userIDFilter != "" {
		err = h.DB.QueryRow(countQuery, userIDFilter).Scan(&totalEntries)
	} else {
		err = h.DB.QueryRow(countQuery).Scan(&totalEntries)
	}

	if err != nil {
		HandleError(w, err, "Error retrieving log count", http.StatusInternalServerError)
		return
	}

	totalPages := (totalEntries + entriesPerPage - 1) / entriesPerPage // Ceiling division

	// Add pagination parameters to args
	args = append(args, entriesPerPage, offset)

	// Get system logs with pagination and filtering
	rows, err := h.DB.Query(logsQuery, args...)
	if err != nil {
		HandleError(w, err, "Error retrieving logs", http.StatusInternalServerError)
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

	// Get all users for the dropdown
	userRows, err := h.DB.Query("SELECT id, fio FROM users ORDER BY fio")
	if err != nil {
		HandleError(w, err, "Error retrieving user list", http.StatusInternalServerError)
		return
	}
	defer userRows.Close()

	type UserOption struct {
		ID  int
		FIO string
	}
	var userList []UserOption
	for userRows.Next() {
		var u UserOption
		userRows.Scan(&u.ID, &u.FIO)
		userList = append(userList, u)
	}

	data := struct {
		User           db2.UserInfo
		Logs           []LogEntry
		UserList       []UserOption
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
		Logs:           logs,
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
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get all teachers for dropdown
	teacherRows, err := h.DB.Query("SELECT id, fio FROM users WHERE role = 'teacher' ORDER BY fio")
	if err != nil {
		HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
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
		err = h.DB.QueryRow("SELECT id, fio FROM users WHERE id = ?", teacherIDInt).
			Scan(&selectedTeacher.ID, &selectedTeacher.FIO)
		if err != nil {
			http.Error(w, "Teacher not found", http.StatusNotFound)
			return
		}

		// Get groups for this teacher
		groupRows, err := h.DB.Query(`
			SELECT DISTINCT group_name 
			FROM (
				SELECT group_name FROM lessons WHERE teacher_id = ? 
				UNION 
				SELECT group_name FROM students WHERE teacher_id = ?
			) ORDER BY group_name`,
			teacherIDInt, teacherIDInt)
		if err != nil {
			HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
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
			err := h.DB.QueryRow("SELECT COUNT(*) FROM students WHERE teacher_id = ? AND group_name = ?",
				teacherIDInt, groupName).Scan(&group.StudentCount)
			if err != nil {
				HandleError(w, err, "Error counting students", http.StatusInternalServerError)
				return
			}

			// Get students in this group
			studentRows, err := h.DB.Query(
				"SELECT id, student_fio FROM students WHERE teacher_id = ? AND group_name = ? ORDER BY student_fio",
				teacherIDInt, groupName)
			if err != nil {
				HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
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
		User        db2.UserInfo
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
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	err = h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
	if err != nil {
		http.Error(w, "Teacher not found", http.StatusNotFound)
		return
	}

	if r.Method == "GET" {
		data := struct {
			User       db2.UserInfo
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
	var exists int
	err = h.DB.QueryRow(`
		SELECT COUNT(*) 
		FROM (
			SELECT group_name FROM lessons WHERE teacher_id = ? 
			UNION 
			SELECT group_name FROM students WHERE teacher_id = ?
		) WHERE group_name = ?`,
		teacherID, teacherID, groupName).Scan(&exists)
	if err != nil {
		HandleError(w, err, "Error checking group", http.StatusInternalServerError)
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
				_, err := db2.ExecuteQuery(h.DB,
					"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
					teacherID, groupName, studentFIO)
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
			_, err := db2.ExecuteQuery(h.DB,
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
	db2.LogAction(h.DB, userInfo.ID, "Admin Create Group", logMessage)

	http.Redirect(w, r, fmt.Sprintf("/admin/groups?teacher_id=%d", teacherID), http.StatusSeeOther)
}

// AdminEditGroupHandler handles admin to edit a teacher's group
func (h *AdminHandler) AdminEditGroupHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	err = h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
	if err != nil {
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
					_, err := db2.ExecuteQuery(h.DB,
						"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
						teacherID, groupName, studentFIO)
					if err == nil {
						studentsAdded++
					}
				}
			}
			if err := scanner.Err(); err != nil {
				HandleError(w, err, "Error reading file", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Admin Upload Student List",
				fmt.Sprintf("Uploaded list of %d students to group %s for teacher ID %d", studentsAdded, groupName, teacherID))

		case "delete":
			// Delete student
			studentID := r.FormValue("student_id")
			_, err := db2.ExecuteQuery(h.DB, "DELETE FROM students WHERE id = ? AND teacher_id = ?", studentID, teacherID)
			if err != nil {
				HandleError(w, err, "Error deleting student", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Admin Delete Student",
				fmt.Sprintf("Deleted student from group %s for teacher ID %d (Student ID: %s)", groupName, teacherID, studentID))

		case "update":
			// Update student name
			studentID := r.FormValue("student_id")
			newFIO := r.FormValue("new_fio")
			_, err := db2.ExecuteQuery(h.DB,
				"UPDATE students SET student_fio = ? WHERE id = ? AND teacher_id = ?",
				newFIO, studentID, teacherID)
			if err != nil {
				HandleError(w, err, "Error updating name", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Admin Update Student Name",
				fmt.Sprintf("Updated student ID %s in group %s for teacher ID %d to %s", studentID, groupName, teacherID, newFIO))

		case "move":
			// Move student to another group
			studentID := r.FormValue("student_id")
			newGroup := r.FormValue("new_group")
			_, err := db2.ExecuteQuery(h.DB,
				"UPDATE students SET group_name = ? WHERE id = ? AND teacher_id = ?",
				newGroup, studentID, teacherID)
			if err != nil {
				HandleError(w, err, "Error moving student", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Admin Move Student",
				fmt.Sprintf("Moved student ID %s from group %s to %s for teacher ID %d", studentID, groupName, newGroup, teacherID))

		case "add_student":
			// Add new student
			studentFIO := r.FormValue("student_fio")
			if studentFIO == "" {
				http.Error(w, "Student name not specified", http.StatusBadRequest)
				return
			}

			_, err := db2.ExecuteQuery(h.DB,
				"INSERT INTO students (teacher_id, group_name, student_fio) VALUES (?, ?, ?)",
				teacherID, groupName, studentFIO)
			if err != nil {
				HandleError(w, err, "Error adding student", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Admin Add Student",
				fmt.Sprintf("Added student %s to group %s for teacher ID %d", studentFIO, groupName, teacherID))
		}

		http.Redirect(w, r, fmt.Sprintf("/admin/groups/edit/%d/%s", teacherID, groupName), http.StatusSeeOther)
		return
	}

	// Get students in this group
	students, err := db2.GetStudentsInGroup(h.DB, teacherID, groupName)
	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
		return
	}

	// Get all groups for move operation
	groupRows, err := h.DB.Query(`
		SELECT DISTINCT group_name 
		FROM (
			SELECT group_name FROM lessons WHERE teacher_id = ? 
			UNION 
			SELECT group_name FROM students WHERE teacher_id = ?
		) ORDER BY group_name`, teacherID, teacherID)
	if err != nil {
		HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
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
		User       db2.UserInfo
		TeacherID  int
		TeacherFIO string
		GroupName  string
		Students   []db2.StudentData
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
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
		_, err := db2.ExecuteQuery(h.DB,
			"DELETE FROM attendance WHERE lesson_id = ?", attendanceID)
		if err != nil {
			HandleError(w, err, "Error deleting attendance", http.StatusInternalServerError)
			return
		}

		db2.LogAction(h.DB, userInfo.ID, "Admin Delete Attendance",
			fmt.Sprintf("Deleted attendance records for lesson ID %s", attendanceID))

		http.Redirect(w, r, "/admin/attendance", http.StatusSeeOther)
		return
	}

	// Get filter parameters
	teacherIDParam := r.URL.Query().Get("teacher_id")
	groupParam := r.URL.Query().Get("group")
	subjectParam := r.URL.Query().Get("subject")

	// Get all teachers for dropdown
	teacherRows, err := h.DB.Query("SELECT id, fio FROM users ORDER BY fio")
	if err != nil {
		HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
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
		User        db2.UserInfo
		TeacherList []struct {
			ID  int
			FIO string
		}
		SelectedTeacherID string
		SelectedGroup     string
		SelectedSubject   string
		Groups            []string
		Subjects          []string
		AttendanceData    []db2.AttendanceData
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
		groups, err := db2.GetTeacherGroups(h.DB, teacherID)
		if err != nil {
			HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
			return
		}
		data.Groups = groups

		// Get subjects for this teacher
		subjects, err := db2.GetTeacherSubjects(h.DB, teacherID)
		if err != nil {
			HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
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
		rows, err := h.DB.Query(query, args...)
		if err != nil {
			HandleError(w, err, "Error retrieving attendance list", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Process attendance records
		var attendances []db2.AttendanceData
		for rows.Next() {
			var a db2.AttendanceData
			var dateStr string
			err := rows.Scan(&a.LessonID, &dateStr, &a.Subject, &a.GroupName, &a.TotalStudents, &a.AttendedStudents)
			if err != nil {
				HandleError(w, err, "Error processing attendance data", http.StatusInternalServerError)
				return
			}
			a.Date = utils.FormatDate(dateStr)
			attendances = append(attendances, a)
		}
		data.AttendanceData = attendances
	}

	// Render template
	renderTemplate(w, h.Tmpl, "admin_attendance.html", data)
}

// AdminEditAttendanceHandler handles admin to edit attendance
func (h *AdminHandler) AdminEditAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

	err = h.DB.QueryRow(`
		SELECT l.id, l.teacher_id, l.group_name, l.subject, l.topic, l.date, l.type, u.fio
		FROM lessons l
		JOIN users u ON l.teacher_id = u.id
		WHERE l.id = ?`,
		lessonID).Scan(&lesson.ID, &lesson.TeacherID, &lesson.Group, &lesson.Subject, &lesson.Topic, &lesson.Date, &lesson.Type, &lesson.TeacherFIO)
	if err != nil {
		if err == sql.ErrNoRows {
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
		err = db2.SaveAttendance(h.DB, lessonID, lesson.TeacherID, attendedIDs)
		if err != nil {
			HandleError(w, err, "Error updating attendance", http.StatusInternalServerError)
			return
		}

		db2.LogAction(h.DB, userInfo.ID, "Admin Edit Attendance",
			fmt.Sprintf("Updated attendance for lesson ID %d, group %s", lessonID, lesson.Group))

		http.Redirect(w, r, "/admin/attendance?teacher_id="+strconv.Itoa(lesson.TeacherID), http.StatusSeeOther)
		return
	}

	// For GET requests, display the form
	// Get student attendance records
	students, err := db2.GetAttendanceForLesson(h.DB, lessonID, lesson.TeacherID)
	if err != nil {
		HandleError(w, err, "Error retrieving students", http.StatusInternalServerError)
		return
	}

	data := struct {
		User   db2.UserInfo
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
		Students   []db2.StudentAttendance
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
	renderTemplate(w, h.Tmpl, "admin_edit_attendance.html", data)
}

// AdminViewAttendanceHandler handles viewing attendance details
func (h *AdminHandler) AdminViewAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

	err = h.DB.QueryRow(`
		SELECT l.id, l.teacher_id, l.group_name, l.subject, l.topic, l.date, l.type, u.fio
		FROM lessons l
		JOIN users u ON l.teacher_id = u.id
		WHERE l.id = ?`,
		lessonID).Scan(&lesson.ID, &lesson.TeacherID, &lesson.Group, &lesson.Subject, &lesson.Topic, &lesson.Date, &lesson.Type, &lesson.TeacherFIO)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Lesson not found", http.StatusNotFound)
		} else {
			HandleError(w, err, "Error retrieving lesson", http.StatusInternalServerError)
		}
		return
	}

	// Format date for display
	lesson.Date = utils.FormatDate(lesson.Date)

	// Get student attendance records
	students, err := db2.GetAttendanceForLesson(h.DB, lessonID, lesson.TeacherID)
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
		User   db2.UserInfo
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
		Students          []db2.StudentAttendance
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

	renderTemplate(w, h.Tmpl, "admin_view_attendance.html", data)
}

// AdminExportAttendanceHandler handles exporting attendance data
func (h *AdminHandler) AdminExportAttendanceHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	err = h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
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
	rows, err := h.DB.Query(query, args...)
	if err != nil {
		HandleError(w, err, "Error retrieving attendance data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Add data rows
	for rows.Next() {
		var dateStr, subject, group, topic, studentFIO string
		var attended int

		err := rows.Scan(&dateStr, &subject, &group, &topic, &studentFIO, &attended)
		if err != nil {
			HandleError(w, err, "Error processing attendance data", http.StatusInternalServerError)
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
	summaryRows, err := h.DB.Query(summaryQuery, summaryArgs...)
	if err != nil {
		HandleError(w, err, "Error retrieving summary data", http.StatusInternalServerError)
		return
	}
	defer summaryRows.Close()

	// Add summary rows
	for summaryRows.Next() {
		var dateStr, subject, group, topic string
		var totalStudents, attendedStudents int

		err := summaryRows.Scan(&dateStr, &subject, &group, &topic, &totalStudents, &attendedStudents)
		if err != nil {
			HandleError(w, err, "Error processing summary data", http.StatusInternalServerError)
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

	db2.LogAction(h.DB, userInfo.ID, "Admin Export Attendance",
		fmt.Sprintf("Exported attendance data for teacher %s (ID: %d)", teacherFIO, teacherID))

	// Send file to user
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=attendance_%s.xlsx", strings.ReplaceAll(teacherFIO, " ", "_")))
	file.Write(w)
}

// AdminLabsHandler handles admin management of lab grades
func (h *AdminHandler) AdminLabsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get all teachers for dropdown
	teacherRows, err := h.DB.Query("SELECT id, fio FROM users WHERE role = 'teacher' ORDER BY fio")
	if err != nil {
		HandleError(w, err, "Error retrieving teacher list", http.StatusInternalServerError)
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
	teacherIDParam := r.URL.Query().Get("teacher_id")
	var selectedTeacherID int
	if teacherIDParam != "" {
		selectedTeacherID, _ = strconv.Atoi(teacherIDParam)
	}

	// Initialize template data
	data := struct {
		User        db2.UserInfo
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
		err = h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", selectedTeacherID).Scan(&data.TeacherFIO)
		if err != nil {
			HandleError(w, err, "Error retrieving teacher info", http.StatusInternalServerError)
			return
		}

		// Get subjects for this teacher
		subjects, err := db2.GetTeacherSubjects(h.DB, selectedTeacherID)
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
			rows, err := h.DB.Query(`
				SELECT DISTINCT group_name 
				FROM lessons 
				WHERE teacher_id = ? AND subject = ? 
				ORDER BY group_name`,
				selectedTeacherID, subject)
			if err != nil {
				HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
				return
			}

			for rows.Next() {
				var groupName string
				rows.Scan(&groupName)

				// Get lab settings and summary
				var totalLabs int
				var groupAverage float64

				// Try to get settings if they exist
				err := h.DB.QueryRow(`
					SELECT total_labs 
					FROM lab_settings 
					WHERE teacher_id = ? AND subject = ? AND group_name = ?`,
					selectedTeacherID, subject, groupName).Scan(&totalLabs)
				if err != nil {
					totalLabs = 5 // Default if not set
				}

				// Get average grade
				err = h.DB.QueryRow(`
					SELECT AVG(grade) 
					FROM lab_grades lg
					JOIN students s ON lg.student_id = s.id
					WHERE lg.teacher_id = ? AND lg.subject = ? AND s.group_name = ?`,
					selectedTeacherID, subject, groupName).Scan(&groupAverage)
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
			rows.Close()

			if len(sg.Groups) > 0 {
				data.SubjectGroups = append(data.SubjectGroups, sg)
			}
		}
	}

	renderTemplate(w, h.Tmpl, "admin_labs.html", data)
}

// AdminViewLabGradesHandler handles viewing lab grades for a specific teacher, subject, and group
func (h *AdminHandler) AdminViewLabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	err = h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
	if err != nil {
		HandleError(w, err, "Error retrieving teacher info", http.StatusInternalServerError)
		return
	}

	// Get lab summary
	summary, err := db2.GetGroupLabSummary(h.DB, teacherID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab grades", http.StatusInternalServerError)
		return
	}

	data := struct {
		User       db2.UserInfo
		TeacherID  int
		TeacherFIO string
		Summary    db2.GroupLabSummary
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
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	err = h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
	if err != nil {
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

			settings := db2.LabSettings{
				TeacherID: teacherID,
				GroupName: groupName,
				Subject:   subject,
				TotalLabs: totalLabs,
			}

			if err := db2.SaveLabSettings(h.DB, settings); err != nil {
				HandleError(w, err, "Error saving lab settings", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Admin Update Lab Settings",
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
				if err := db2.SaveLabGrade(h.DB, teacherID, studentID, subject, labNumber, grade); err != nil {
					HandleError(w, err, "Error saving grade", http.StatusInternalServerError)
					return
				}
			}

			db2.LogAction(h.DB, userInfo.ID, "Admin Update Lab Grades",
				fmt.Sprintf("Updated lab grades for teacher ID %d, %s, %s",
					teacherID, subject, groupName))
		}

		// Redirect to prevent form resubmission
		http.Redirect(w, r, fmt.Sprintf("/admin/labs/edit/%d/%s/%s", teacherID, subject, groupName),
			http.StatusSeeOther)
		return
	}

	// Get lab summary
	summary, err := db2.GetGroupLabSummary(h.DB, teacherID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab grades", http.StatusInternalServerError)
		return
	}

	data := struct {
		User       db2.UserInfo
		TeacherID  int
		TeacherFIO string
		Summary    db2.GroupLabSummary
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
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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
	err = h.DB.QueryRow("SELECT fio FROM users WHERE id = ?", teacherID).Scan(&teacherFIO)
	if err != nil {
		HandleError(w, err, "Error retrieving teacher info", http.StatusInternalServerError)
		return
	}

	// Get lab summary
	summary, err := db2.GetGroupLabSummary(h.DB, teacherID, groupName, subject)
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
	db2.LogAction(h.DB, userInfo.ID, "Admin Export Lab Grades",
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

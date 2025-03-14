package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/config"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tealeg/xlsx"
	"gorm.io/gorm"
)

// LabHandler handles lab-related routes
type LabHandler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

// NewLabHandler creates a new LabHandler
func NewLabHandler(database *gorm.DB, tmpl *template.Template) *LabHandler {
	return &LabHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// GroupLabsHandler shows all groups by subject with Lab Submissions buttons
func (h *LabHandler) GroupLabsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get subjects for this teacher
	subjects, err := db.GetTeacherSubjects(h.DB, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
		return
	}

	// Get groups for this teacher
	groups, err := db.GetGroupsByTeacher(h.DB, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	// Group the groups by subject
	type SubjectGroups struct {
		Subject string
		Groups  []db.GroupData
	}

	var subjectGroupsList []SubjectGroups

	for _, subject := range subjects {
		sg := SubjectGroups{
			Subject: subject,
		}

		// For this subject, find all groups that have lessons in this subject
		var groupNames []string
		h.DB.Model(&models.Lesson{}).
			Distinct("group_name").
			Where("teacher_id = ? AND subject = ?", userInfo.ID, subject).
			Order("group_name").
			Pluck("group_name", &groupNames)

		// Map group names to group data
		groupMap := make(map[string]db.GroupData)
		for _, g := range groups {
			groupMap[g.Name] = g
		}

		for _, groupName := range groupNames {
			// If we have this group in our map, add it to the subject's groups
			if group, ok := groupMap[groupName]; ok {
				sg.Groups = append(sg.Groups, group)
			}
		}

		if len(sg.Groups) > 0 {
			subjectGroupsList = append(subjectGroupsList, sg)
		}
	}

	data := struct {
		User          db.UserInfo
		SubjectGroups []SubjectGroups
	}{
		User:          userInfo,
		SubjectGroups: subjectGroupsList,
	}

	renderTemplate(w, h.Tmpl, "labs.html", data)
}

// LabGradesHandler shows and manages lab grades for a specific group and subject
func (h *LabHandler) LabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	subject := vars["subject"]
	groupName := vars["group"]

	// For POST requests, update lab settings or grades
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
				TeacherID: userInfo.ID,
				GroupName: groupName,
				Subject:   subject,
				TotalLabs: totalLabs,
			}

			if err := db.SaveLabSettings(h.DB, settings); err != nil {
				HandleError(w, err, "Error saving lab settings", http.StatusInternalServerError)
				return
			}

			db.LogAction(h.DB, userInfo.ID, "Update Lab Settings",
				fmt.Sprintf("Updated lab settings for %s, %s: %d labs", subject, groupName, totalLabs))

		case "update_grades":
			// Get all form values and update grades
			for key, values := range r.Form {
				// Keys should be in format grade_studentID_labNumber
				if len(values) == 0 {
					continue
				}

				var studentID, labNumber int
				var err error

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

				// Save the grade
				if err := db.SaveLabGrade(h.DB, userInfo.ID, studentID, subject, labNumber, grade); err != nil {
					HandleError(w, err, "Error saving grade", http.StatusInternalServerError)
					return
				}
			}

			db.LogAction(h.DB, userInfo.ID, "Update Lab Grades",
				fmt.Sprintf("Updated lab grades for %s, %s", subject, groupName))
		}

		// Redirect to the same page to prevent form resubmission
		http.Redirect(w, r, fmt.Sprintf("/labs/grades/%s/%s", subject, groupName), http.StatusSeeOther)
		return
	}

	// Get lab summary for the group
	summary, err := db.GetGroupLabSummary(h.DB, userInfo.ID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab summary", http.StatusInternalServerError)
		return
	}

	data := struct {
		User    db.UserInfo
		Summary db.GroupLabSummary
	}{
		User:    userInfo,
		Summary: summary,
	}

	renderTemplate(w, h.Tmpl, "lab_grades.html", data)
}

// ExportLabGradesHandler exports lab grades for a specific group and subject to Excel
func (h *LabHandler) ExportLabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	subject := vars["subject"]
	groupName := vars["group"]

	// Get lab summary for the group
	summary, err := db.GetGroupLabSummary(h.DB, userInfo.ID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab summary", http.StatusInternalServerError)
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

	// Log the export action
	db.LogAction(h.DB, userInfo.ID, "Export Lab Grades",
		fmt.Sprintf("Exported lab grades for subject %s, group %s", subject, groupName))

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

// ViewLabGradesHandler handles viewing lab grades (without editing) for a specific group and subject
func (h *LabHandler) ViewLabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	subject := vars["subject"]
	groupName := vars["group"]

	// Get lab summary for the group
	summary, err := db.GetGroupLabSummary(h.DB, userInfo.ID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab summary", http.StatusInternalServerError)
		return
	}

	data := struct {
		User    db.UserInfo
		Summary db.GroupLabSummary
	}{
		User:    userInfo,
		Summary: summary,
	}

	renderTemplate(w, h.Tmpl, "view_labs.html", data)
}

// ShareLabGradesHandler generates a shareable link for lab grades
func (h *LabHandler) ShareLabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	subject := vars["subject"]
	groupName := vars["group"]

	// Check if the group belongs to this teacher
	groups, err := db.GetGroupsByTeacher(h.DB, userInfo.ID)
	if err != nil {
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	// Verify the teacher has access to this group
	hasAccess := false
	for _, group := range groups {
		if group.Name == groupName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		http.Error(w, "No access to this group", http.StatusForbidden)
		return
	}

	// For POST requests, generate a new shareable link
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}

		// Parse expiration days
		expirationDays := 7 // Default to 7 days
		if expStr := r.FormValue("expiration"); expStr != "" {
			if exp, err := strconv.Atoi(expStr); err == nil {
				expirationDays = exp
			}
		}

		// Generate the shared link
		token, err := db.CreateSharedLabLink(h.DB, userInfo.ID, groupName, subject, expirationDays)
		if err != nil {
			http.Error(w, "Error creating shared link", http.StatusInternalServerError)
			return
		}

		// Construct the full URL
		shareURL := fmt.Sprintf("%s/s/%s", getBaseURL(r), token)

		// Log the action
		db.LogAction(h.DB, userInfo.ID, "Share Lab Grades",
			fmt.Sprintf("Created shared link for %s, %s", subject, groupName))

		// Return the URL as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"shareUrl": shareURL,
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// SharedLabViewHandler handles viewing lab grades via a shared link
func (h *LabHandler) SharedLabViewHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	// Get the shared link details
	sharedLink, err := db.GetSharedLabLink(h.DB, token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "The link is invalid or has expired", http.StatusNotFound)
		} else {
			http.Error(w, "Error retrieving shared link", http.StatusInternalServerError)
		}
		return
	}

	// Get teacher info
	var teacher models.User
	if err := h.DB.First(&teacher, sharedLink.TeacherID).Error; err != nil {
		http.Error(w, "Error retrieving teacher information", http.StatusInternalServerError)
		return
	}

	// Get lab summary for the group
	summary, err := db.GetGroupLabSummary(h.DB, sharedLink.TeacherID, sharedLink.GroupName, sharedLink.Subject)
	if err != nil {
		http.Error(w, "Error retrieving lab summary", http.StatusInternalServerError)
		return
	}

	// Prepare expiration date string if applicable
	var expirationDate string
	if sharedLink.ExpiresAt != nil {
		expirationDate = sharedLink.ExpiresAt.Format("02.01.2006")
	}

	data := struct {
		Summary        db.GroupLabSummary
		TeacherName    string
		ExpirationDate string
	}{
		Summary:        summary,
		TeacherName:    teacher.FIO,
		ExpirationDate: expirationDate,
	}

	// Log the access
	db.LogAction(h.DB, sharedLink.TeacherID, "Shared Link Accessed",
		fmt.Sprintf("Shared link for %s, %s was accessed", sharedLink.Subject, sharedLink.GroupName))

	renderTemplate(w, h.Tmpl, "shared_lab_view.html", data)
}

// Helper function to get the base URL from a request
func getBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

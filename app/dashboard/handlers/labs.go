package handlers

import (
	db2 "TeacherJournal/app/dashboard/db"
	"TeacherJournal/config"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tealeg/xlsx"
)

// LabHandler handles lab-related routes
type LabHandler struct {
	DB   *sql.DB
	Tmpl *template.Template
}

// NewLabHandler creates a new LabHandler
func NewLabHandler(database *sql.DB, tmpl *template.Template) *LabHandler {
	return &LabHandler{
		DB:   database,
		Tmpl: tmpl,
	}
}

// GroupLabsHandler shows all groups by subject with Lab Submissions buttons
func (h *LabHandler) GroupLabsHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get subjects for this teacher
	subjects, err := db2.GetTeacherSubjects(h.DB, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving subjects", http.StatusInternalServerError)
		return
	}

	// Get groups for this teacher
	groups, err := db2.GetGroupsByTeacher(h.DB, userInfo.ID)
	if err != nil {
		HandleError(w, err, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	// Group the groups by subject
	type SubjectGroups struct {
		Subject string
		Groups  []db2.GroupData
	}

	var subjectGroupsList []SubjectGroups

	for _, subject := range subjects {
		sg := SubjectGroups{
			Subject: subject,
		}

		// For this subject, find all groups that have lessons in this subject
		rows, err := h.DB.Query(
			"SELECT DISTINCT group_name FROM lessons WHERE teacher_id = ? AND subject = ? ORDER BY group_name",
			userInfo.ID, subject)
		if err != nil {
			HandleError(w, err, "Error retrieving groups for subject", http.StatusInternalServerError)
			return
		}

		// Map group names to group data
		groupMap := make(map[string]db2.GroupData)
		for _, g := range groups {
			groupMap[g.Name] = g
		}

		for rows.Next() {
			var groupName string
			if err := rows.Scan(&groupName); err != nil {
				rows.Close()
				HandleError(w, err, "Error scanning group name", http.StatusInternalServerError)
				return
			}

			// If we have this group in our map, add it to the subject's groups
			if group, ok := groupMap[groupName]; ok {
				sg.Groups = append(sg.Groups, group)
			}
		}
		rows.Close()

		if len(sg.Groups) > 0 {
			subjectGroupsList = append(subjectGroupsList, sg)
		}
	}

	data := struct {
		User          db2.UserInfo
		SubjectGroups []SubjectGroups
	}{
		User:          userInfo,
		SubjectGroups: subjectGroupsList,
	}

	renderTemplate(w, h.Tmpl, "labs.html", data)
}

// LabGradesHandler shows and manages lab grades for a specific group and subject
func (h *LabHandler) LabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
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

			settings := db2.LabSettings{
				TeacherID: userInfo.ID,
				GroupName: groupName,
				Subject:   subject,
				TotalLabs: totalLabs,
			}

			if err := db2.SaveLabSettings(h.DB, settings); err != nil {
				HandleError(w, err, "Error saving lab settings", http.StatusInternalServerError)
				return
			}

			db2.LogAction(h.DB, userInfo.ID, "Update Lab Settings",
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
				if err := db2.SaveLabGrade(h.DB, userInfo.ID, studentID, subject, labNumber, grade); err != nil {
					HandleError(w, err, "Error saving grade", http.StatusInternalServerError)
					return
				}
			}

			db2.LogAction(h.DB, userInfo.ID, "Update Lab Grades",
				fmt.Sprintf("Updated lab grades for %s, %s", subject, groupName))
		}

		// Redirect to the same page to prevent form resubmission
		http.Redirect(w, r, fmt.Sprintf("/labs/grades/%s/%s", subject, groupName), http.StatusSeeOther)
		return
	}

	// Get lab summary for the group
	summary, err := db2.GetGroupLabSummary(h.DB, userInfo.ID, groupName, subject)
	if err != nil {
		HandleError(w, err, "Error retrieving lab summary", http.StatusInternalServerError)
		return
	}

	data := struct {
		User    db2.UserInfo
		Summary db2.GroupLabSummary
	}{
		User:    userInfo,
		Summary: summary,
	}

	renderTemplate(w, h.Tmpl, "lab_grades.html", data)
}

// ExportLabGradesHandler exports lab grades for a specific group and subject to Excel
func (h *LabHandler) ExportLabGradesHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := db2.GetUserInfo(h.DB, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	subject := vars["subject"]
	groupName := vars["group"]

	// Get lab summary for the group
	summary, err := db2.GetGroupLabSummary(h.DB, userInfo.ID, groupName, subject)
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
	db2.LogAction(h.DB, userInfo.ID, "Export Lab Grades",
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

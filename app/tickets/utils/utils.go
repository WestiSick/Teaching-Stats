package utils

import (
	"TeacherJournal/app/tickets/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// HandleError handles HTTP errors with logging
func HandleError(w http.ResponseWriter, err error, message string, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, statusCode)
}

// FormatDate formats date from DB format to display format
func FormatDate(dateStr string) string {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		log.Printf("Error parsing date: %v", err)
		return dateStr
	}
	return date.Format("02.01.06")
}

// JSONError sends an error response in JSON format
func JSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := models.APIResponse{
		Success: false,
		Message: message,
	}
	json.NewEncoder(w).Encode(response)
}

// GenerateStatusClass returns a CSS class based on ticket status
func GenerateStatusClass(status string) string {
	switch status {
	case "New":
		return "status-new"
	case "Open":
		return "status-open"
	case "InProgress":
		return "status-progress"
	case "Resolved":
		return "status-resolved"
	case "Closed":
		return "status-closed"
	default:
		return ""
	}
}

// GeneratePriorityClass returns a CSS class based on ticket priority
func GeneratePriorityClass(priority string) string {
	switch priority {
	case "Low":
		return "priority-low"
	case "Medium":
		return "priority-medium"
	case "High":
		return "priority-high"
	case "Critical":
		return "priority-critical"
	default:
		return ""
	}
}

// FormatTimeAgo formats a time.Time into a human-readable "X time ago" format
func FormatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	} else {
		years := int(duration.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

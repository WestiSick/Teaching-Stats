package utils

import (
	"encoding/json"
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

// RespondJSON responds with JSON data
func RespondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

package middleware

import (
	"TeacherJournal/app/dashboard/db"
	ticketDB "TeacherJournal/app/tickets/db"
	"TeacherJournal/config"
	"log"
	"net/http"
	"strings"
)

// TicketSystemMiddleware forwards ticket system requests to the ticket system handler
func TicketSystemMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is for the ticket system
		if strings.HasPrefix(r.URL.Path, "/tickets") {
			// Forward to ticket system
			ticketSystemHandler(w, r)
			return
		}

		// Not a ticket system request, continue with the next handler
		next.ServeHTTP(w, r)
	})
}

// ticketSystemHandler handles requests for the ticket system
func ticketSystemHandler(w http.ResponseWriter, r *http.Request) {
	// Verify the user is authenticated
	session, _ := config.Store.Get(r, config.SessionName)
	userID, ok := session.Values["userID"]
	if !ok || userID == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Forward the request to the ticket system server
	ticketSystemURL := "http://localhost:" + string(config.TicketSystemPort) + r.URL.Path

	// Create a new request to the ticket system
	ticketReq, err := http.NewRequest(r.Method, ticketSystemURL, r.Body)
	if err != nil {
		http.Error(w, "Error forwarding request to ticket system", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			ticketReq.Header.Add(key, value)
		}
	}

	// Set user session token
	ticketReq.Header.Set("X-Session-Token", session.ID)
	ticketReq.Header.Set("X-User-ID", string(userID.(int)))

	// Forward the request
	client := &http.Client{}
	resp, err := client.Do(ticketReq)
	if err != nil {
		http.Error(w, "Error communicating with ticket system", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
}

// GetUserTicketCounts gets the count of open tickets for a user
func GetUserTicketCounts(userID int) (int, error) {
	// Initialize the database connection
	database := ticketDB.InitTicketDB()

	// Get count of open tickets for this user
	var count int64
	err := database.Table("tickets").
		Where("created_by = ? AND status NOT IN ('Closed', 'Resolved')", userID).
		Count(&count).Error

	return int(count), err
}

// GetAdminTicketCounts gets the count of unassigned and open tickets
func GetAdminTicketCounts() (int, error) {
	// Initialize the database connection
	database := ticketDB.InitTicketDB()

	// Get count of unassigned tickets
	var unassignedCount int64
	err := database.Table("tickets").
		Where("assigned_to IS NULL AND status NOT IN ('Closed', 'Resolved')").
		Count(&unassignedCount).Error

	if err != nil {
		return 0, err
	}

	// Get count of open tickets
	var openCount int64
	err = database.Table("tickets").
		Where("status IN ('New', 'Open')").
		Count(&openCount).Error

	// Return the sum
	return int(unassignedCount + openCount), err
}

// AddTicketLinks adds ticket system links and notifications to the dashboard
func AddTicketLinks(w http.ResponseWriter, r *http.Request, next http.Handler) {
	// Get user info from session
	userInfo, err := db.GetUserInfo(db.DB, r, config.Store, config.SessionName)
	if err != nil {
		// User not logged in, just continue
		next.ServeHTTP(w, r)
		return
	}

	// Declare ticketCount at function scope
	var ticketCount int

	// Get ticket counts for the user
	if userInfo.Role == "admin" {
		ticketCount, err = GetAdminTicketCounts()
	} else {
		ticketCount, err = GetUserTicketCounts(userInfo.ID)
	}

	// Log the ticket count or any errors
	if err != nil {
		log.Printf("Error getting ticket counts: %v", err)
	} else if ticketCount > 0 {
		log.Printf("User %d has %d active tickets", userInfo.ID, ticketCount)
	}

	// Continue with the next handler
	next.ServeHTTP(w, r)
}

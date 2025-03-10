package config

import (
	"fmt"
	"os"

	"github.com/gorilla/sessions"
)

// Application constants
const (
	CookieStoreKey = "super-secret-key"
	SessionName    = "session-name"
)

// Get DB connection string from environment or use default
var DBConnectionString = getEnv("DB_CONNECTION_STRING", fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
	getEnv("DB_USER", "postgres"),
	getEnv("DB_PASSWORD", "vadimvadimvadim13"),
	getEnv("DB_HOST", "localhost"),
	getEnv("DB_PORT", "5432"),
	getEnv("DB_NAME", "teacher")))

// Store is the global store for sessions (shared with Teaching Stats)
var Store = sessions.NewCookieStore([]byte(CookieStoreKey))

// TicketSystemPort is the port on which the ticket system runs
const TicketSystemPort = 8090

// TicketStatusValues defines the valid status values for tickets
var TicketStatusValues = []string{"New", "Open", "InProgress", "Resolved", "Closed"}

// TicketPriorityValues defines the valid priority values for tickets
var TicketPriorityValues = []string{"Low", "Medium", "High", "Critical"}

// TicketCategoryValues defines the valid category values for tickets
var TicketCategoryValues = []string{"Technical", "Administrative", "Account", "Feature", "Bug", "Other"}

// AttachmentStoragePath defines where file attachments are stored
const AttachmentStoragePath = "./attachments"

// MaxFileSize defines the maximum size for uploaded files (5MB)
const MaxFileSize = 5 * 1024 * 1024

// Helper function to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

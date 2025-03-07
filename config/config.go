package config

import (
	"github.com/gorilla/sessions"
)

// Application constants
const (
	CookieStoreKey     = "super-secret-key"
	SessionName        = "session-name"
	DBConnectionString = "../../../.././teaching_stats.db?_busy_timeout=5000"
)

// TicketSystemPort is the port on which the ticket system runs
const TicketSystemPort = 8090

// Store is the global store for sessions (shared with Teaching Stats)
var Store = sessions.NewCookieStore([]byte(CookieStoreKey))

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

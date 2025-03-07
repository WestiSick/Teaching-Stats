package config

import (
	"github.com/gorilla/sessions"
)

// Application constants
const (
	CookieStoreKey     = "super-secret-key"
	SessionName        = "session-name"
	DBConnectionString = "./teaching_stats.db?_busy_timeout=5000"
)

// Store is the global store for sessions
var Store = sessions.NewCookieStore([]byte(CookieStoreKey))

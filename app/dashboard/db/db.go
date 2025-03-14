package db

import (
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/config"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance
var DB *gorm.DB

// InitDB initializes the database and creates tables if they don't exist
func InitDB() *gorm.DB {
	var err error

	// Create a new GORM DB connection
	DB, err = gorm.Open(postgres.Open(config.DBConnectionString), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Get underlying SQL DB to set connection pool settings
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Failed to get SQL DB:", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	// Auto-migrate the schema
	err = DB.AutoMigrate(
		&models.User{},
		&models.Lesson{},
		&models.Student{},
		&models.Attendance{},
		&models.Log{},
		&models.LabSettings{},
		&models.LabGrade{},
		&models.UserNotificationSettings{},
		&models.SharedLabLink{},
	)

	if err != nil {
		log.Fatal("Failed to auto-migrate database:", err)
	}

	log.Println("Database initialized successfully")
	return DB
}

// LogAction records an action in the system log
func LogAction(db *gorm.DB, userID int, action, details string) {
	logEntry := models.Log{
		UserID:  userID,
		Action:  action,
		Details: details,
	}

	result := db.Create(&logEntry)
	if result.Error != nil {
		log.Printf("Error logging action: %v", result.Error)
	}
}

package main

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/schedule/handlers"
	"TeacherJournal/app/schedule/middleware"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Инициализируем базу данных
	database := db.InitDB()

	// Получаем sql.DB для отложенного закрытия
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatal("Failed to get SQL DB for closing:", err)
	}
	defer sqlDB.Close()

	// Определяем абсолютный путь к директории с шаблонами
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}

	// Получаем директорию, в которой находится исполняемый файл
	baseDir := filepath.Dir(execPath)

	// Устанавливаем путь к директории с шаблонами
	templatesDir := filepath.Join(baseDir, "templates")

	// Проверяем, существует ли директория
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		// Если директория не найдена, пробуем найти относительно текущей рабочей директории
		workDir, err := os.Getwd()
		if err != nil {
			log.Fatalf("Error getting working directory: %v", err)
		}

		// Ищем шаблоны относительно рабочей директории
		templatesDir = filepath.Join(workDir, "app", "schedule", "templates")

		// Проверяем существование этой директории
		if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
			log.Fatalf("Templates directory not found at: %s", templatesDir)
		}
	}

	fmt.Printf("Using templates directory: %s\n", templatesDir)

	// Инициализируем обработчики и передаем путь к шаблонам
	handlers.InitTemplates(templatesDir)
	handlers.InitDB(database)

	// Регистрируем обработчики
	http.HandleFunc("/", middleware.AuthMiddleware(database, handlers.ScheduleHandler))

	// Запуск сервера
	fmt.Println("Server started at http://localhost:8091")
	log.Fatal(http.ListenAndServe(":8091", nil))
}

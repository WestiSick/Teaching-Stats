package main

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/schedule/handlers"
	"TeacherJournal/app/schedule/middleware"
	"fmt"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Функция-обработчик для маршрутизации в контексте Docker/Nginx
func scheduleRouter(db *gorm.DB, templatesDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Инициализируем обработчики и передаем путь к шаблонам, если это еще не сделано
		handlers.InitTemplates(templatesDir)
		handlers.InitDB(db)

		// Удаляем префикс /schedule из URL, если он есть
		// Это нужно для корректной работы при проксировании через Nginx
		path := r.URL.Path
		if strings.HasPrefix(path, "/schedule") {
			r.URL.Path = strings.TrimPrefix(path, "/schedule")
			// Если после удаления префикса путь пустой, делаем его "/"
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
		}

		// Обрабатываем запрос на добавление пары
		if r.URL.Path == "/add-lesson" || r.URL.Path == "add-lesson" {
			middleware.AuthMiddleware(db, handlers.ScheduleHandler).ServeHTTP(w, r)
			return
		}

		// Обрабатываем основной запрос
		middleware.AuthMiddleware(db, handlers.ScheduleHandler).ServeHTTP(w, r)
	}
}

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

	// Регистрируем маршруты с учетом Nginx проксирования
	http.HandleFunc("/", scheduleRouter(database, templatesDir))
	http.HandleFunc("/schedule/", scheduleRouter(database, templatesDir))
	http.HandleFunc("/schedule", scheduleRouter(database, templatesDir))

	// Запуск сервера
	fmt.Println("Server started at http://localhost:8091")
	log.Fatal(http.ListenAndServe(":8091", nil))
}

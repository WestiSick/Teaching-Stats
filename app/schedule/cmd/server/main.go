package main

import (
	"TeacherJournal/app/schedule/handlers"
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Регистрируем обработчики
	http.HandleFunc("/", handlers.ScheduleHandler)

	// Запуск сервера
	fmt.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

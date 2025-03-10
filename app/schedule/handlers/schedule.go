package handlers

import (
	"TeacherJournal/app/schedule/models"
	"TeacherJournal/app/schedule/utils"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// ScheduleHandler обрабатывает запросы для страницы расписания
func ScheduleHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем текущую дату
	currentDate := time.Now().Format("2006-01-02")

	// Инициализируем данные страницы
	data := models.PageData{
		HasResults:  false,
		CurrentDate: currentDate,
		Date:        currentDate, // По умолчанию используем текущую дату
	}

	// Обрабатываем отправку формы
	if r.Method == http.MethodPost {
		// Парсим данные формы
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Получаем имя преподавателя и дату из формы
		teacher := r.FormValue("teacher")
		if teacher == "" {
			http.Error(w, "Teacher name is required", http.StatusBadRequest)
			return
		}

		date := r.FormValue("date")
		if date == "" {
			date = currentDate // Используем текущую дату, если не указана
		}

		data.Teacher = teacher
		data.Date = date
		data.HasResults = true

		// Получаем и обрабатываем расписание
		scheduleResp, err := fetchSchedule(teacher, date)
		if err != nil {
			http.Error(w, "Error fetching schedule: "+err.Error(), http.StatusInternalServerError)
			return
		}

		data.Schedule = template.HTML(scheduleResp.HTML)
		data.DebugInfo = template.HTML(scheduleResp.DebugInfo)
		data.ResponseSize = scheduleResp.Size
		data.MatchCount = scheduleResp.MatchesCount
	}

	// Определяем путь к шаблону
	tmplPath := filepath.Join("templates", "schedule.html")

	// Парсим и выполняем шаблон
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "Error parsing template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Error rendering page: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// fetchSchedule получает расписание с API и обрабатывает даты
func fetchSchedule(teacher, date string) (models.ScheduleResponse, error) {
	var response models.ScheduleResponse
	var debugInfo strings.Builder

	// URL-кодируем имя преподавателя
	encodedTeacher := url.QueryEscape(teacher)

	// URL API
	apiURL := fmt.Sprintf("https://apivgltu2.ru/schedule?teacher=%s&date=%s", encodedTeacher, date)
	debugInfo.WriteString(fmt.Sprintf("Fetching URL: %s\n", apiURL))

	// Выполняем HTTP-запрос
	resp, err := http.Get(apiURL)
	if err != nil {
		return response, fmt.Errorf("error making API request: %w", err)
	}
	defer resp.Body.Close()

	debugInfo.WriteString(fmt.Sprintf("Response status: %s\n", resp.Status))

	// Читаем тело ответа
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, fmt.Errorf("error reading response: %w", err)
	}

	// Конвертируем в строку
	content := string(body)
	debugInfo.WriteString(fmt.Sprintf("Response length: %d bytes\n", len(content)))

	// Показываем превью содержимого
	preview := content
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	debugInfo.WriteString(fmt.Sprintf("Content preview: %s\n\n", preview))

	// Обрабатываем даты
	processedContent, matches, dateDebugInfo := utils.ProcessDates(content)

	// Добавляем отладочную информацию о датах
	debugInfo.WriteString(dateDebugInfo)

	// Формируем объект ответа
	response.HTML = processedContent
	response.DebugInfo = debugInfo.String()
	response.Size = len(processedContent)
	response.MatchesCount = len(matches)

	return response, nil
}

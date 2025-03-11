package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/dashboard/models"
	scheduleModels "TeacherJournal/app/schedule/models"
	"TeacherJournal/config"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

// templateDir хранит путь к директории шаблонов
var templateDir string

// database хранит экземпляр базы данных
var database *gorm.DB

// Карта соответствия сокращений типов занятий их полным названиям
var classTypeMap = map[string]string{
	"пр":  "Практика",
	"лек": "Лекция",
	"лаб": "Лабораторная работа",
}

// InitTemplates инициализирует путь к директории шаблонов
func InitTemplates(templatesPath string) {
	templateDir = templatesPath
}

// InitDB инициализирует соединение с базой данных
func InitDB(db *gorm.DB) {
	database = db
}

// ScheduleHandler обрабатывает запросы для страницы расписания
func ScheduleHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем информацию о пользователе
	userInfo, err := db.GetUserInfo(database, r, config.Store, config.SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Получаем текущую дату
	currentDate := time.Now().Format("2006-01-02")

	// Проверяем параметр success в URL
	successParam := r.URL.Query().Get("success")

	// Инициализируем данные страницы
	data := scheduleModels.PageData{
		HasResults:  false,
		CurrentDate: currentDate,
		Date:        currentDate,            // По умолчанию используем текущую дату
		User:        userInfo,               // Добавляем информацию о пользователе
		Success:     successParam == "true", // Устанавливаем флаг успеха
	}

	// Проверяем добавление пары
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/add-lesson") {
		if err := handleAddLesson(w, r, userInfo); err != nil {
			http.Error(w, fmt.Sprintf("Error adding lesson: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Обрабатываем отправку формы поиска расписания
	if r.Method == http.MethodPost && (r.URL.Path == "/" || r.URL.Path == "/schedule/" || r.URL.Path == "/schedule") {
		// Окружаем обработку формы в try-catch блок
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic in form processing: %v", r)
					http.Error(w, "Internal server error while processing the form", http.StatusInternalServerError)
				}
			}()

			// Парсим данные формы
			if err := r.ParseForm(); err != nil {
				log.Printf("Error parsing form: %v", err)
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

			// Выполняем запрос напрямую
			debugInfo, htmlContent, err := fetchDirectSchedule(teacher, date)
			if err != nil {
				log.Printf("Error fetching schedule: %v", err)
				// Не возвращаем ошибку пользователю, а просто отображаем пустой результат
				data.Schedule = template.HTML("<div class='no-data'>Не удалось получить расписание. Пожалуйста, проверьте правильность введенных данных или попробуйте позже.</div>")
				data.DebugInfo = template.HTML(fmt.Sprintf("Error: %v", err))
				data.ResponseSize = 0
				data.MatchCount = 0
				return
			}

			// Декодируем HTML-сущности
			decodedHTML := html.UnescapeString(htmlContent)

			// Парсим расписание из HTML
			processedHTML, itemCount, scheduleItems := parseScheduleHTMLWithEntities(decodedHTML)

			// Сохраняем расписание в сессии для добавления пар
			// Используем try-catch блок для обработки возможных ошибок
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Recovered from panic in session handling: %v", r)
					}
				}()

				session, err := config.Store.Get(r, "schedule-session")
				if err != nil {
					log.Printf("Error getting session: %v", err)
					return
				}

				sessionData, err := json.Marshal(scheduleItems)
				if err != nil {
					log.Printf("Error serializing data: %v", err)
					return
				}

				session.Values["scheduleItems"] = string(sessionData)

				if err := session.Save(r, w); err != nil {
					log.Printf("Error saving session: %v", err)
				}
			}()

			// Добавляем информацию для отладки
			debugInfo += fmt.Sprintf("\nExtracted %d schedule items\n", itemCount)

			data.Schedule = template.HTML(processedHTML)
			data.DebugInfo = template.HTML(debugInfo)
			data.ResponseSize = len(htmlContent)
			data.MatchCount = itemCount
		}()
	}

	// Определяем путь к шаблону
	tmplPath := filepath.Join(templateDir, "schedule.html")

	// Парсим и выполняем шаблон
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "Error parsing template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Оборачиваем выполнение шаблона в try-catch блок
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in template execution: %v", r)
				http.Error(w, "Error rendering page", http.StatusInternalServerError)
			}
		}()

		err = tmpl.Execute(w, data)
		if err != nil {
			log.Printf("Error rendering page: %v", err)
			http.Error(w, "Error rendering page: "+err.Error(), http.StatusInternalServerError)
		}
	}()
}

// handleAddLesson обрабатывает добавление пары в систему
func handleAddLesson(w http.ResponseWriter, r *http.Request, userInfo db.UserInfo) error {
	// Окружаем в try-catch блок для отлова паник
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleAddLesson: %v", r)
		}
	}()

	// Парсим форму
	if err := r.ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return err
	}

	// Получаем индекс пары из формы
	indexStr := r.FormValue("lesson_index")
	if indexStr == "" {
		http.Error(w, "Lesson index is required", http.StatusBadRequest)
		return fmt.Errorf("lesson index is required")
	}

	// Получаем сохраненные данные расписания из сессии
	session, err := config.Store.Get(r, "schedule-session")
	if err != nil {
		log.Printf("Error getting session: %v", err)
		http.Error(w, "Error getting session", http.StatusInternalServerError)
		return fmt.Errorf("error getting session: %w", err)
	}

	scheduleItemsStr, ok := session.Values["scheduleItems"].(string)
	if !ok || scheduleItemsStr == "" {
		log.Printf("No schedule data available in session")
		http.Error(w, "No schedule data available", http.StatusBadRequest)
		return fmt.Errorf("no schedule data available")
	}

	// Десериализуем данные расписания
	var scheduleItems []scheduleModels.ScheduleItem
	if err := json.Unmarshal([]byte(scheduleItemsStr), &scheduleItems); err != nil {
		log.Printf("Error parsing schedule data: %v", err)
		http.Error(w, "Error parsing schedule data", http.StatusInternalServerError)
		return fmt.Errorf("error parsing schedule data: %w", err)
	}

	// Проверяем корректность индекса
	index := -1
	for i, item := range scheduleItems {
		if item.ID == indexStr {
			index = i
			break
		}
	}

	if index == -1 {
		log.Printf("Invalid lesson index: %s", indexStr)
		http.Error(w, "Invalid lesson index", http.StatusBadRequest)
		return fmt.Errorf("invalid lesson index: %s", indexStr)
	}

	// Получаем данные о паре
	lessonItem := scheduleItems[index]

	// Формируем имя группы с подгруппой
	groupName := lessonItem.Group
	if lessonItem.Subgroup != "Вся группа" && lessonItem.Subgroup != "Поток" {
		groupName = fmt.Sprintf("%s %s", groupName, lessonItem.Subgroup)
	}

	// Создаем новую пару
	lesson := models.Lesson{
		TeacherID: userInfo.ID,
		GroupName: groupName,
		Subject:   lessonItem.Subject,
		Topic:     "Импортировано из расписания", // Дефолтная тема
		Hours:     2,                             // Всегда 2 часа
		Date:      lessonItem.Date,               // Дата из расписания
		Type:      lessonItem.ClassType,          // Тип пары
	}

	// Сохраняем пару в базу данных
	if err := database.Create(&lesson).Error; err != nil {
		log.Printf("Error saving lesson: %v", err)
		http.Error(w, "Error saving lesson: "+err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("error saving lesson: %w", err)
	}

	// Логируем добавление пары
	db.LogAction(database, userInfo.ID, "Import Lesson from Schedule",
		fmt.Sprintf("Added %s: %s, %s, %s hours, %s", lessonItem.ClassType, lessonItem.Subject, groupName, "2", lessonItem.Date))

	// Перенаправляем обратно на страницу с сообщением об успехе
	// Проверяем контекст маршрутизации и используем правильный путь для перенаправления
	redirectPath := "/"
	if strings.HasPrefix(r.RequestURI, "/schedule") {
		redirectPath = "/schedule/"
	}

	http.Redirect(w, r, redirectPath+"?success=true", http.StatusSeeOther)
	return nil
}

// parseScheduleHTMLWithEntities извлекает расписание из HTML-ответа API с учетом HTML-сущностей
// Возвращает HTML-код для отображения, количество элементов и структурированные данные о парах
func parseScheduleHTMLWithEntities(html string) (string, int, []scheduleModels.ScheduleItem) {
	// Окружаем в try-catch блок для отлова паник
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in parseScheduleHTMLWithEntities: %v", r)
		}
	}()

	var result strings.Builder
	itemCount := 0
	var scheduleItems []scheduleModels.ScheduleItem

	// Проверка на пустой HTML
	if html == "" {
		result.WriteString("<div class='no-data'>Получен пустой ответ от сервера расписания</div>")
		return result.String(), 0, scheduleItems
	}

	// Регулярное выражение для поиска блоков дней (учитываем структуру с маржином)
	dayBlockRegex := regexp.MustCompile(`(?s)<div[^>]*margin-bottom: 25px[^>]*>\s*<div>\s*<strong>(\d+) ([а-яА-Я]+) (\d{4})</strong>\s*</div>\s*<div>\s*([а-яА-Я]+)\s*</div>\s*<table>(.*?)</table>\s*</div>`)
	dayBlocks := dayBlockRegex.FindAllStringSubmatch(html, -1)

	if len(dayBlocks) == 0 {
		result.WriteString("<div class='no-data'>Не удалось найти блоки расписания в ответе сервера</div>")
		return result.String(), 0, scheduleItems
	}

	for _, dayBlock := range dayBlocks {
		if len(dayBlock) < 6 {
			continue
		}

		day := dayBlock[1]
		monthRussian := strings.ToLower(dayBlock[2])
		year := dayBlock[3]
		scheduleTable := dayBlock[5]

		// Если в этот день нет пар, пропускаем этот день
		if strings.Contains(scheduleTable, "Нет пар") {
			continue
		}

		// Получаем числовой месяц
		var month string
		switch monthRussian {
		case "января":
			month = "01"
		case "февраля":
			month = "02"
		case "марта":
			month = "03"
		case "апреля":
			month = "04"
		case "мая":
			month = "05"
		case "июня":
			month = "06"
		case "июля":
			month = "07"
		case "августа":
			month = "08"
		case "сентября":
			month = "09"
		case "октября":
			month = "10"
		case "ноября":
			month = "11"
		case "декабря":
			month = "12"
		default:
			continue
		}

		// Форматируем дату в виде ГГГГ-ММ-ДД для БД и ДД.ММ.ГГГГ для отображения
		dbFormatDate := fmt.Sprintf("%s-%s-%s", year, month, day)
		displayDate := fmt.Sprintf("%s.%s.%s", day, month, year)

		// Ищем все классы (лекции, лабы и т.д.) в расписании этого дня
		// Обратите внимание на структуру td с width:75px и width:auto
		classRegex := regexp.MustCompile(`(?s)<tr>\s*<td[^>]*>(\d+:\d+-\d+:\d+)</td>\s*<td[^>]*>(.*?)</td>\s*</tr>`)
		classes := classRegex.FindAllStringSubmatch(scheduleTable, -1)

		if len(classes) == 0 {
			// Не найдены пары в этот день
			continue
		}

		for _, class := range classes {
			if len(class) < 3 {
				continue
			}

			// Получаем время занятия
			classTime := class[1]

			// Получаем детали занятия
			classContent := class[2]

			// Ищем тип и название предмета (лаб., пр., лек.)
			// Используем более точное сопоставление для извлечения предмета
			subjectRegex := regexp.MustCompile(`(лаб|пр|лек)\.\s+([^<\r\n]+)`)
			subjectMatch := subjectRegex.FindStringSubmatch(classContent)

			if len(subjectMatch) < 3 {
				continue
			}

			classType := subjectMatch[1]
			subjectName := strings.TrimSpace(subjectMatch[2])

			// Получаем полное название типа занятия
			classTypeFull, ok := classTypeMap[classType]
			if !ok {
				classTypeFull = classType + "." // Если не найдено, оставляем как есть
			}

			// Ищем все группы (например, ИС1-227-ОТ)
			// Для лекций может быть несколько групп
			groupRegex := regexp.MustCompile(`(ИС\d+-\d+-[А-Я]{2})`)
			groupMatches := groupRegex.FindAllStringSubmatch(classContent, -1)

			groups := []string{}
			for _, match := range groupMatches {
				if len(match) >= 2 {
					groups = append(groups, match[1])
				}
			}

			// Если группы не найдены, пропускаем эту пару
			if len(groups) == 0 {
				continue
			}

			// Преобразуем список групп в строку
			groupsStr := strings.Join(groups, ", ")

			// Ищем подгруппу (например, 1 п.г. или 2 п.г.)
			// Обычно подгруппы есть только для лабораторных и практических занятий
			subgroupRegex := regexp.MustCompile(`(\d+)\s+п\.г\.`)
			subgroupMatch := subgroupRegex.FindStringSubmatch(classContent)

			subgroup := "Вся группа"
			if len(subgroupMatch) >= 2 {
				subgroup = fmt.Sprintf("%s п.г.", subgroupMatch[1])
			}

			// Для лекций обычно не указывается подгруппа
			if classType == "лек" && len(subgroupMatch) == 0 {
				subgroup = "Поток"
			}

			// Создаем уникальный ID для этой пары
			lessonID := fmt.Sprintf("lesson_%d", itemCount)

			// Создаем объект пары для сохранения
			scheduleItem := scheduleModels.ScheduleItem{
				ID:        lessonID,
				Date:      dbFormatDate,
				Time:      classTime,
				ClassType: classTypeFull,
				Subject:   subjectName,
				Group:     groups[0], // Берем первую группу (если их несколько)
				Subgroup:  subgroup,
			}
			scheduleItems = append(scheduleItems, scheduleItem)

			// Обратите внимание, что мы убираем hardcoded пути для action в форме
			// JavaScript в шаблоне schedule.html определит правильный путь во время выполнения
			result.WriteString(fmt.Sprintf(`<div class="schedule-item">
<div class="date-line">Дата: %s</div>
<div class="time-line">Время: %s</div>
<div class="type-line">Тип: %s</div>
<div class="subject-line">Предмет: %s</div>
<div class="group-line">Группа: %s</div>
<div class="subgroup-line">Подгруппа: %s</div>
<form method="post" class="add-lesson-form">
  <input type="hidden" name="lesson_index" value="%s">
  <button type="submit" class="add-lesson-btn">Добавить в пары</button>
</form>
</div>`, displayDate, classTime, classTypeFull, subjectName, groupsStr, subgroup, lessonID))

			itemCount++
		}
	}

	// Если ничего не нашли, выводим сообщение
	if itemCount == 0 {
		result.WriteString("<div class='no-data'>Не найдено предметов для отображения</div>")
	}

	return result.String(), itemCount, scheduleItems
}

// fetchDirectSchedule выполняет прямой запрос к API и возвращает отладочную информацию и HTML-контент
func fetchDirectSchedule(teacher, date string) (string, string, error) {
	var debugBuilder strings.Builder

	// URL-кодируем имя преподавателя
	encodedTeacher := url.QueryEscape(teacher)

	// URL API
	apiURL := fmt.Sprintf("https://apivgltu2.ru/schedule?teacher=%s&date=%s", encodedTeacher, date)
	debugBuilder.WriteString(fmt.Sprintf("Fetching URL: %s\n", apiURL))

	// Создаем HTTP-клиент с настройками
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Создаем запрос
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return debugBuilder.String(), "", fmt.Errorf("error creating request: %w", err)
	}

	// Добавляем заголовки
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Add("Accept-Language", "ru-RU,ru;q=0.8,en-US;q=0.5,en;q=0.3")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Выполняем запрос
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making API request: %v", err)
		return debugBuilder.String(), "", fmt.Errorf("error making API request: %w", err)
	}
	defer resp.Body.Close()

	// Журналируем статус ответа
	debugBuilder.WriteString(fmt.Sprintf("Response status: %s\n", resp.Status))

	// Журналируем заголовки ответа
	debugBuilder.WriteString("Response headers:\n")
	for key, values := range resp.Header {
		for _, value := range values {
			debugBuilder.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
		}
	}

	// Читаем тело ответа
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return debugBuilder.String(), "", fmt.Errorf("error reading response: %w", err)
	}

	// Конвертируем в строку
	content := string(body)
	debugBuilder.WriteString(fmt.Sprintf("\nResponse length: %d bytes\n", len(content)))

	// Проверяем, не пустой ли ответ
	if content == "" {
		log.Printf("Empty response received from API")
		return debugBuilder.String(), "", fmt.Errorf("empty response received from API")
	}

	// Показываем превью содержимого
	preview := content
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	debugBuilder.WriteString(fmt.Sprintf("\nContent preview:\n%s\n", preview))

	return debugBuilder.String(), content, nil
}

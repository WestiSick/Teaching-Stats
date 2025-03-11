package handlers

import (
	"TeacherJournal/app/dashboard/db"
	"TeacherJournal/app/schedule/models"
	"TeacherJournal/config"
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
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

	// Инициализируем данные страницы
	data := models.PageData{
		HasResults:  false,
		CurrentDate: currentDate,
		Date:        currentDate, // По умолчанию используем текущую дату
		User:        userInfo,    // Добавляем информацию о пользователе
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

		// Выполняем запрос напрямую
		debugInfo, htmlContent, err := fetchDirectSchedule(teacher, date)
		if err != nil {
			http.Error(w, "Error fetching schedule: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Декодируем HTML-сущности
		decodedHTML := html.UnescapeString(htmlContent)

		// Парсим расписание из HTML
		processedHTML, itemCount := parseScheduleHTMLWithEntities(decodedHTML)

		// Добавляем информацию для отладки
		debugInfo += fmt.Sprintf("\nExtracted %d schedule items\n", itemCount)

		data.Schedule = template.HTML(processedHTML)
		data.DebugInfo = template.HTML(debugInfo)
		data.ResponseSize = len(htmlContent)
		data.MatchCount = itemCount
	}

	// Определяем путь к шаблону
	tmplPath := filepath.Join(templateDir, "schedule.html")

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

// parseScheduleHTMLWithEntities извлекает расписание из HTML-ответа API с учетом HTML-сущностей
func parseScheduleHTMLWithEntities(html string) (string, int) {
	var result strings.Builder
	itemCount := 0

	// Регулярное выражение для поиска блоков дней (учитываем структуру с маржином)
	dayBlockRegex := regexp.MustCompile(`(?s)<div[^>]*margin-bottom: 25px[^>]*>\s*<div>\s*<strong>(\d+) ([а-яА-Я]+) (\d{4})</strong>\s*</div>\s*<div>\s*([а-яА-Я]+)\s*</div>\s*<table>(.*?)</table>\s*</div>`)
	dayBlocks := dayBlockRegex.FindAllStringSubmatch(html, -1)

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

		// Форматируем дату в виде ДД.ММ.ГГГГ
		formattedDate := fmt.Sprintf("%s.%s.%s", day, month, year)

		// Ищем все классы (лекции, лабы и т.д.) в расписании этого дня
		// Обратите внимание на структуру td с width:75px и width:auto
		classRegex := regexp.MustCompile(`(?s)<tr>\s*<td[^>]*>(\d+:\d+-\d+:\d+)</td>\s*<td[^>]*>(.*?)</td>\s*</tr>`)
		classes := classRegex.FindAllStringSubmatch(scheduleTable, -1)

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

			// Преобразуем список групп в строку
			groupsStr := "Нет информации"
			if len(groups) > 0 {
				groupsStr = strings.Join(groups, ", ")
			}

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

			// Формируем элемент расписания
			result.WriteString(fmt.Sprintf(`<div class="schedule-item">
<div class="date-line">Дата: %s</div>
<div class="time-line">Время: %s</div>
<div class="type-line">Тип: %s</div>
<div class="subject-line">Предмет: %s</div>
<div class="group-line">Группа: %s</div>
<div class="subgroup-line">Подгруппа: %s</div>
</div>`, formattedDate, classTime, classTypeFull, subjectName, groupsStr, subgroup))

			itemCount++
		}
	}

	// Если ничего не нашли, выводим сообщение
	if itemCount == 0 {
		result.WriteString("<div class='no-data'>Не найдено предметов для отображения</div>")
	}

	return result.String(), itemCount
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
		return debugBuilder.String(), "", fmt.Errorf("error creating request: %w", err)
	}

	// Добавляем заголовки
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Add("Accept-Language", "ru-RU,ru;q=0.8,en-US;q=0.5,en;q=0.3")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Выполняем запрос
	resp, err := client.Do(req)
	if err != nil {
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
		return debugBuilder.String(), "", fmt.Errorf("error reading response: %w", err)
	}

	// Конвертируем в строку
	content := string(body)
	debugBuilder.WriteString(fmt.Sprintf("\nResponse length: %d bytes\n", len(content)))

	// Показываем превью содержимого
	preview := content
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	debugBuilder.WriteString(fmt.Sprintf("\nContent preview:\n%s\n", preview))

	return debugBuilder.String(), content, nil
}

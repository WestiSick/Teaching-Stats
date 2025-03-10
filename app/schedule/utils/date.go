package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// MonthMap карта русских названий месяцев на числовые значения
var MonthMap = map[string]string{
	"января":   "01",
	"февраля":  "02",
	"марта":    "03",
	"апреля":   "04",
	"мая":      "05",
	"июня":     "06",
	"июля":     "07",
	"августа":  "08",
	"сентября": "09",
	"октября":  "10",
	"ноября":   "11",
	"декабря":  "12",
}

// ConvertDate преобразует дату из русского формата в формат ДД.ММ.ГГ
func ConvertDate(russianDate string) (string, error) {
	// Очищаем строку от лишних пробелов
	russianDate = strings.TrimSpace(russianDate)

	// Разбиваем дату на компоненты
	parts := strings.Fields(russianDate)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid date format: %s", russianDate)
	}

	day := parts[0]
	monthRussian := strings.ToLower(parts[1])
	year := parts[2]

	// Если год полный (4 цифры), оставляем только последние 2 цифры для формата ДД.ММ.ГГ
	if len(year) == 4 {
		year = year[2:]
	}

	// Получаем числовой месяц
	month, ok := MonthMap[monthRussian]
	if !ok {
		return "", fmt.Errorf("unknown month: %s", monthRussian)
	}

	// Форматируем как "ДД.ММ.ГГ"
	return fmt.Sprintf("%s.%s.%s", day, month, year), nil
}

// ProcessDates обрабатывает все даты в HTML-контенте и переструктурирует расписание
func ProcessDates(content string) (processedContent string, matches []string, debugInfo string) {
	var debugBuilder strings.Builder

	// Регулярное выражение для поиска дат в формате "ДД месяц ГГГГ"
	dateRegex := regexp.MustCompile(`(\d{1,2}\s+[а-яА-Я]+\s+\d{4})`)
	matches = dateRegex.FindAllString(content, -1)

	debugBuilder.WriteString(fmt.Sprintf("Found %d date matches\n", len(matches)))
	for i, match := range matches {
		if i < 10 { // Ограничим до первых 10 совпадений, чтобы избежать лишнего вывода
			debugBuilder.WriteString(fmt.Sprintf("Matched date: '%s'\n", match))
		}

		// Конвертируем дату в формат ДД.ММ.ГГ
		formattedDate, err := ConvertDate(match)
		if err == nil {
			// Ищем блок даты и изменяем его структуру
			// Выделяем дату в отдельный элемент с классом date
			oldDateBlock := regexp.MustCompile(regexp.QuoteMeta(match) + `\s*\n\s*([^\n]+)(\s*\n\s*Нет пар)?`)
			replacement := fmt.Sprintf(`<span class="date">%s</span>`, formattedDate)

			// Если после даты идет день недели и "Нет пар"
			if oldDateBlock.MatchString(content) {
				submatches := oldDateBlock.FindStringSubmatch(content)
				if len(submatches) >= 3 {
					weekday := strings.TrimSpace(submatches[1])
					noPairs := strings.TrimSpace(submatches[2])

					if noPairs != "" {
						replacement = fmt.Sprintf(`<span class="date">%s</span><span class="weekday">%s</span><span class="no-classes">Нет пар</span>`,
							formattedDate, weekday)
					} else {
						replacement = fmt.Sprintf(`<span class="date">%s</span><span class="weekday">%s</span>`,
							formattedDate, weekday)
					}

					content = oldDateBlock.ReplaceAllString(content, replacement)
				}
			} else {
				// Просто заменяем дату
				content = strings.Replace(content, match, replacement, -1)
			}
		}
	}

	// Ищем и выделяем названия пар (лабораторные, практики и т.д.)
	classRegex := regexp.MustCompile(`\s*(лаб|практ|лек)[\.а-яА-Я\s]+([\d\.]+)\s+[а-яА-Я\.]+`)
	content = classRegex.ReplaceAllStringFunc(content, func(match string) string {
		return fmt.Sprintf(`<span class="class-name">%s</span>`, strings.TrimSpace(match))
	})

	// Добавляем классы для времени занятий
	timeRegex := regexp.MustCompile(`(\d{1,2}[:]\d{2}[-]\d{1,2}[:]\d{2})`)
	content = timeRegex.ReplaceAllString(content, `<span class="time">$1</span>`)

	// Структурируем классы в блоки
	content = strings.Replace(content, "<span class=\"class-name\">", "<div class=\"class\"><span class=\"class-name\">", -1)

	// Закрываем div для классов перед следующей датой
	dateStartRegex := regexp.MustCompile(`<span class="date">`)
	content = dateStartRegex.ReplaceAllStringFunc(content, func(match string) string {
		return "</div>" + match
	})

	// Удаляем первый закрывающий тег, если он в начале
	if strings.HasPrefix(content, "</div>") {
		content = content[6:]
	}

	// Добавляем закрывающий тег в конец
	content = content + "</div>"

	// Обновляем информацию для отладки
	debugBuilder.WriteString("\nRestructured HTML for better display\n")

	return content, matches, debugBuilder.String()
}

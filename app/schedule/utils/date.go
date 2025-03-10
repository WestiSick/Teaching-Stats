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

// ConvertDate преобразует дату из русского формата в формат ДД.ММ.ГГГГ
func ConvertDate(russianDate string) (string, error) {
	// Разбиваем дату на компоненты
	parts := strings.Fields(russianDate)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid date format: %s", russianDate)
	}

	day := parts[0]
	monthRussian := parts[1]
	year := parts[2]

	// Получаем числовой месяц
	month, ok := MonthMap[monthRussian]
	if !ok {
		return "", fmt.Errorf("unknown month: %s", monthRussian)
	}

	// Форматируем как "ДД.ММ.ГГГГ"
	return fmt.Sprintf("%s.%s.%s", day, month, year), nil
}

// ProcessDates обрабатывает все даты в HTML-контенте
func ProcessDates(content string) (processedContent string, matches []string, debugInfo string) {
	var debugBuilder strings.Builder

	// Регулярное выражение для поиска дат в формате "ДД месяц ГГГГ"
	dateRegex := regexp.MustCompile(`(\d{2}\s+[а-яА-Я]+\s+\d{4})`)
	matches = dateRegex.FindAllString(content, -1)

	debugBuilder.WriteString(fmt.Sprintf("Found %d date matches\n", len(matches)))
	for i, match := range matches {
		if i < 10 { // Ограничим до первых 10 совпадений, чтобы избежать лишнего вывода
			debugBuilder.WriteString(fmt.Sprintf("Matched date: '%s'\n", match))
		}
	}

	// Заменяем все найденные даты форматом ДД.ММ.ГГГГ
	processedContent = dateRegex.ReplaceAllStringFunc(content, func(match string) string {
		formattedDate, err := ConvertDate(match)
		if err != nil {
			debugBuilder.WriteString(fmt.Sprintf("Error converting date '%s': %s\n", match, err.Error()))
			// Возвращаем оригинал, если конвертация не удалась
			return match
		}
		debugBuilder.WriteString(fmt.Sprintf("Converted '%s' to '%s'\n", match, formattedDate))
		return formattedDate
	})

	return processedContent, matches, debugBuilder.String()
}

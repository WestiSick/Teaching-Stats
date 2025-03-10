package models

import "html/template"

// PageData содержит данные для отображения на странице
type PageData struct {
	Schedule     template.HTML
	DebugInfo    template.HTML
	ResponseSize int
	MatchCount   int
	Teacher      string
	Date         string
	HasResults   bool
	CurrentDate  string
}

// ScheduleResponse представляет ответ от API расписания
type ScheduleResponse struct {
	HTML         string
	DebugInfo    string
	Size         int
	MatchesCount int
}

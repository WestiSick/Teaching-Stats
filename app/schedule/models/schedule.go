package models

import (
	"TeacherJournal/app/dashboard/db"
	"html/template"
)

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
	User         db.UserInfo // Информация о пользователе
	Success      bool        // Флаг успешного добавления пары
}

// ScheduleResponse представляет ответ от API расписания
type ScheduleResponse struct {
	HTML         string
	DebugInfo    string
	Size         int
	MatchesCount int
}

// ScheduleItem представляет информацию о паре в расписании
type ScheduleItem struct {
	ID        string // Уникальный идентификатор пары
	Date      string // Дата в формате YYYY-MM-DD
	Time      string // Время пары
	ClassType string // Тип занятия (Лекция, Практика, Лабораторная работа)
	Subject   string // Название предмета
	Group     string // Группа
	Subgroup  string // Подгруппа
}

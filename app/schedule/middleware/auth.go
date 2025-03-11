package middleware

import (
	"TeacherJournal/app/dashboard/models"
	"TeacherJournal/config"
	"net/http"

	"gorm.io/gorm"
)

// AuthMiddleware проверяет аутентификацию пользователя и его роль
func AuthMiddleware(db *gorm.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Получаем сессию пользователя
		session, _ := config.Store.Get(r, config.SessionName)
		userID, ok := session.Values["userID"]

		// Проверяем, залогинен ли пользователь
		if !ok || userID == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Проверяем роль пользователя
		var user models.User
		err := db.Model(&models.User{}).
			Select("role").
			Where("id = ?", userID).
			First(&user).Error

		// Если ошибка или роль "free", запрещаем доступ
		if err != nil || user.Role == "free" {
			http.Redirect(w, r, "/subscription", http.StatusSeeOther)
			return
		}

		// Если роль "teacher" или "admin", разрешаем доступ
		next.ServeHTTP(w, r)
	}
}

/**
 * attendance.js - Функции для работы с посещаемостью
 */

/**
 * Объект для работы с посещаемостью
 */
const Attendance = {
    /**
     * Загрузить уроки для выбранного предмета
     * @param {string} subjectId - ID предмета
     */
    loadLessons: async (subjectId) => {
        const subjectElement = document.getElementById('subject');
        const lessonSelect = document.getElementById('lesson_id');
        const studentsContainer = document.getElementById('students');
        const saveButton = document.getElementById('saveBtn');

        // Если предмет не выбран, сбрасываем всё
        if (!subjectElement || !subjectElement.value) {
            if (lessonSelect) {
                lessonSelect.innerHTML = '<option value="">Выберите пару</option>';
            }
            if (studentsContainer) {
                studentsContainer.style.display = 'none';
            }
            if (saveButton) {
                saveButton.style.display = 'none';
            }
            return;
        }

        try {
            // Получаем ID преподавателя из элемента на странице
            const teacherId = document.querySelector('.user-info')
                ? document.querySelector('.user-info').textContent.match(/ID: (\d+)/)
                : null;

            // Получаем список уроков
            const response = await fetch(`/api/lessons?subject=${encodeURIComponent(subjectElement.value)}`, {
                headers: { 'X-Teacher-ID': teacherId ? teacherId[1] : '' }
            });

            if (!response.ok) {
                throw new Error('Failed to load lessons');
            }

            const lessons = await response.json();

            // Обновляем выпадающий список с уроками
            if (lessonSelect) {
                lessonSelect.innerHTML = '<option value="">Выберите пару</option>';
                lessons.forEach(lesson => {
                    const option = document.createElement('option');
                    option.value = lesson.id;
                    option.textContent = `${lesson.date} - ${lesson.group_name}`;
                    lessonSelect.appendChild(option);
                });
            }

            // Скрываем список студентов и кнопку сохранения
            if (studentsContainer) {
                studentsContainer.style.display = 'none';
            }
            if (saveButton) {
                saveButton.style.display = 'none';
            }
        } catch (error) {
            console.error('Error loading lessons:', error);

            // В случае ошибки показываем сообщение пользователю
            if (typeof Modal !== 'undefined') {
                Modal.showMessage('Ошибка', 'Не удалось загрузить список пар', 'error');
            } else {
                alert('Не удалось загрузить список пар');
            }
        }
    },

    /**
     * Загрузить студентов для выбранного урока
     * @param {string} lessonId - ID урока
     */
    loadStudents: async (lessonId) => {
        const lessonElement = document.getElementById('lesson_id');
        const studentsContainer = document.getElementById('students');
        const saveButton = document.getElementById('saveBtn');

        // Если урок не выбран, скрываем список студентов и кнопку сохранения
        if (!lessonElement || !lessonElement.value) {
            if (studentsContainer) {
                studentsContainer.style.display = 'none';
            }
            if (saveButton) {
                saveButton.style.display = 'none';
            }
            return;
        }

        try {
            // Получаем ID преподавателя из элемента на странице
            const teacherId = document.querySelector('.user-info')
                ? document.querySelector('.user-info').textContent.match(/ID: (\d+)/)
                : null;

            // Получаем список студентов
            const response = await fetch(`/api/students?lesson_id=${lessonElement.value}`, {
                headers: { 'X-Teacher-ID': teacherId ? teacherId[1] : '' }
            });

            if (!response.ok) {
                throw new Error('Failed to load students');
            }

            const students = await response.json();

            // Обновляем список студентов
            if (studentsContainer) {
                studentsContainer.innerHTML = '<h2>Студенты:</h2>';
                students.forEach(student => {
                    const label = document.createElement('label');
                    label.innerHTML = `<input type="checkbox" name="attended" value="${student.id}"> ${student.fio}`;
                    studentsContainer.appendChild(label);
                });
                studentsContainer.style.display = 'block';
            }

            // Показываем кнопку сохранения
            if (saveButton) {
                saveButton.style.display = 'block';
            }
        } catch (error) {
            console.error('Error loading students:', error);

            // В случае ошибки показываем сообщение пользователю
            if (typeof Modal !== 'undefined') {
                Modal.showMessage('Ошибка', 'Не удалось загрузить список студентов', 'error');
            } else {
                alert('Не удалось загрузить список студентов');
            }
        }
    },

    /**
     * Обновить счетчик посещаемости
     */
    updateAttendanceCount: () => {
        const checkboxes = document.querySelectorAll('input[name="attended"]:checked');
        const totalCount = document.querySelectorAll('input[name="attended"]').length;
        const countElement = document.getElementById('attendanceCount');

        if (countElement) {
            countElement.textContent = checkboxes.length;
        }
    },

    /**
     * Выбрать всех студентов
     */
    selectAll: () => {
        const checkboxes = document.querySelectorAll('input[name="attended"]');
        checkboxes.forEach(checkbox => checkbox.checked = true);
        Attendance.updateAttendanceCount();
    },

    /**
     * Снять выделение со всех студентов
     */
    deselectAll: () => {
        const checkboxes = document.querySelectorAll('input[name="attended"]');
        checkboxes.forEach(checkbox => checkbox.checked = false);
        Attendance.updateAttendanceCount();
    },

    /**
     * Инвертировать выбор студентов
     */
    invertSelection: () => {
        const checkboxes = document.querySelectorAll('input[name="attended"]');
        checkboxes.forEach(checkbox => checkbox.checked = !checkbox.checked);
        Attendance.updateAttendanceCount();
    },

    /**
     * Инициализировать обработчики событий для редактирования посещаемости
     */
    initEditHandlers: () => {
        const selectAllBtn = document.querySelector('[onclick="selectAll()"]');
        const deselectAllBtn = document.querySelector('[onclick="deselectAll()"]');
        const invertSelectionBtn = document.querySelector('[onclick="invertSelection()"]');

        if (selectAllBtn) {
            selectAllBtn.onclick = (e) => {
                e.preventDefault();
                Attendance.selectAll();
            };
        }

        if (deselectAllBtn) {
            deselectAllBtn.onclick = (e) => {
                e.preventDefault();
                Attendance.deselectAll();
            };
        }

        if (invertSelectionBtn) {
            invertSelectionBtn.onclick = (e) => {
                e.preventDefault();
                Attendance.invertSelection();
            };
        }

        // Добавляем обработчики на чекбоксы
        document.querySelectorAll('input[name="attended"]').forEach(checkbox => {
            checkbox.addEventListener('change', Attendance.updateAttendanceCount);
        });

        // Инициализируем счетчик посещаемости
        Attendance.updateAttendanceCount();
    }
};

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    // Назначаем обработчики событий для выпадающих списков
    const subjectElement = document.getElementById('subject');
    const lessonElement = document.getElementById('lesson_id');

    if (subjectElement) {
        subjectElement.addEventListener('change', () => {
            Attendance.loadLessons(subjectElement.value);
        });
    }

    if (lessonElement) {
        lessonElement.addEventListener('change', () => {
            Attendance.loadStudents(lessonElement.value);
        });
    }

    // Инициализируем обработчики для редактирования посещаемости
    Attendance.initEditHandlers();

    // Если страница редактирования посещаемости
    if (document.querySelector('.attendance-controls')) {
        // Переопределяем глобальные функции для работы с селекторами
        if (typeof window !== 'undefined') {
            window.selectAll = Attendance.selectAll;
            window.deselectAll = Attendance.deselectAll;
            window.invertSelection = Attendance.invertSelection;
            window.updateAttendanceCount = Attendance.updateAttendanceCount;
        }
    }
});
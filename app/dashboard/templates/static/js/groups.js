/**
 * groups.js - Функции для работы с группами
 */

/**
 * Объект для работы с группами
 */
const Groups = {
    /**
     * Добавить поле для ввода студента
     */
    addStudentField: () => {
        const studentList = document.getElementById('student-list') || document.getElementById('studentFields');
        if (!studentList) return;

        const newEntry = document.createElement('div');
        newEntry.className = 'student-entry';
        newEntry.innerHTML = '<input type="text" name="student_fio" placeholder="ФИО студента">';
        studentList.appendChild(newEntry);
    },

    /**
     * Показать форму редактирования студента
     * @param {number} studentId - ID студента
     * @param {string} studentFio - ФИО студента
     */
    showEditForm: (studentId, studentFio) => {
        const editForm = document.getElementById('editForm');
        const editStudentId = document.getElementById('edit_student_id');
        const newFio = document.getElementById('new_fio');

        if (editForm && editStudentId && newFio) {
            editForm.style.display = 'block';
            editStudentId.value = studentId;
            newFio.value = studentFio;

            // Скрываем другие формы
            const moveForm = document.getElementById('moveForm');
            if (moveForm) {
                moveForm.style.display = 'none';
            }
        }
    },

    /**
     * Скрыть форму редактирования студента
     */
    hideEditForm: () => {
        const editForm = document.getElementById('editForm');
        if (editForm) {
            editForm.style.display = 'none';
        }
    },

    /**
     * Показать форму перемещения студента
     * @param {number} studentId - ID студента
     */
    showMoveForm: (studentId) => {
        const moveForm = document.getElementById('moveForm');
        const moveStudentId = document.getElementById('move_student_id');

        if (moveForm && moveStudentId) {
            moveForm.style.display = 'block';
            moveStudentId.value = studentId;

            // Скрываем другие формы
            const editForm = document.getElementById('editForm');
            if (editForm) {
                editForm.style.display = 'none';
            }
        }
    },

    /**
     * Скрыть форму перемещения студента
     */
    hideMoveForm: () => {
        const moveForm = document.getElementById('moveForm');
        if (moveForm) {
            moveForm.style.display = 'none';
        }
    },

    /**
     * Удалить студента
     * @param {number} studentId - ID студента
     * @param {string} form - ID формы или селектор
     */
    deleteStudent: (studentId, form = 'deleteStudentForm') => {
        if (confirm('Вы уверены, что хотите удалить студента?')) {
            const deleteForm = document.getElementById(form) || document.querySelector(form);

            if (deleteForm) {
                const studentIdInput = deleteForm.querySelector('[name="student_id"]');
                if (studentIdInput) {
                    studentIdInput.value = studentId;
                    deleteForm.submit();
                }
            }
        }
    },

    /**
     * Удалить группу
     * @param {string} groupName - Название группы
     * @param {string} form - ID формы или селектор
     */
    deleteGroup: (groupName, form = 'deleteGroupForm') => {
        if (confirm(`Вы уверены, что хотите удалить группу ${groupName} со всеми парами и студентами?`)) {
            const deleteForm = document.getElementById(form) || document.querySelector(form);

            if (deleteForm) {
                const groupNameInput = deleteForm.querySelector('[name="group_name"]');
                if (groupNameInput) {
                    groupNameInput.value = groupName;
                    deleteForm.submit();
                }
            }
        }
    }
};

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    // Назначаем обработчики для кнопок добавления студентов
    const addStudentBtn = document.querySelector('.add-student-btn') || document.querySelector('[onclick="addStudentField()"]');
    if (addStudentBtn) {
        addStudentBtn.addEventListener('click', (e) => {
            e.preventDefault();
            Groups.addStudentField();
        });
    }

    // Назначаем обработчики для кнопок редактирования
    document.querySelectorAll('[onclick*="showEditForm"]').forEach(button => {
        button.addEventListener('click', function(e) {
            e.preventDefault();

            // Получаем параметры из атрибута onclick
            const onclickAttr = this.getAttribute('onclick');
            const params = onclickAttr.match(/showEditForm\(([^)]+)\)/);

            if (params && params[1]) {
                const args = params[1].split(',').map(arg => arg.trim());
                const studentId = parseInt(args[0]);
                // Удаляем кавычки из строки ФИО
                const studentFio = args[1].replace(/^['"]|['"]$/g, '');

                Groups.showEditForm(studentId, studentFio);
            }
        });
    });

    // Назначаем обработчики для кнопок перемещения
    document.querySelectorAll('[onclick*="showMoveForm"]').forEach(button => {
        button.addEventListener('click', function(e) {
            e.preventDefault();

            // Получаем параметры из атрибута onclick
            const onclickAttr = this.getAttribute('onclick');
            const params = onclickAttr.match(/showMoveForm\(([^)]+)\)/);

            if (params && params[1]) {
                const studentId = parseInt(params[1]);
                Groups.showMoveForm(studentId);
            }
        });
    });

    // Назначаем обработчики для кнопок отмены
    const hideEditFormBtn = document.querySelector('[onclick="hideEditForm()"]');
    if (hideEditFormBtn) {
        hideEditFormBtn.addEventListener('click', (e) => {
            e.preventDefault();
            Groups.hideEditForm();
        });
    }

    const hideMoveFormBtn = document.querySelector('[onclick="hideMoveForm()"]');
    if (hideMoveFormBtn) {
        hideMoveFormBtn.addEventListener('click', (e) => {
            e.preventDefault();
            Groups.hideMoveForm();
        });
    }

    // Если в глобальной области нужны функции
    if (typeof window !== 'undefined') {
        window.addStudentField = Groups.addStudentField;
        window.showEditForm = Groups.showEditForm;
        window.hideEditForm = Groups.hideEditForm;
        window.showMoveForm = Groups.showMoveForm;
        window.hideMoveForm = Groups.hideMoveForm;
    }
});
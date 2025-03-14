/**
 * labs.js - Функции для работы с лабораторными работами
 */

/**
 * Объект для работы с лабораторными работами
 */
const Labs = {
    /**
     * Валидация оценок
     * @param {HTMLInputElement} input - Поле ввода оценки
     */
    validateGradeInput: (input) => {
        const value = parseInt(input.value);
        if (input.value !== '' && (value < 1 || value > 5 || isNaN(value))) {
            alert('Оценка должна быть от 1 до 5');
            input.value = '';
        }
    },

    /**
     * Инициализировать валидацию оценок
     */
    initGradeValidation: () => {
        const inputs = document.querySelectorAll('.grade-input');
        inputs.forEach(input => {
            input.addEventListener('change', function() {
                Labs.validateGradeInput(this);
            });
        });
    },

    /**
     * Показать/скрыть студентов в группе
     * @param {string} containerId - ID контейнера студентов
     */
    toggleStudents: (containerId) => {
        const container = document.getElementById(containerId);
        if (container) {
            container.style.display = container.style.display === 'block' ? 'none' : 'block';
        }
    },

    /**
     * Обработка формы поделиться
     * @param {HTMLFormElement} form - Форма
     */
    handleShareForm: async (form) => {
        try {
            // Получаем значение срока действия
            const expirationSelect = form.querySelector('[name="expiration"]');
            if (!expirationSelect) return;

            const expirationValue = expirationSelect.value;

            // Создаем данные формы
            const params = new URLSearchParams();
            params.append('expiration', expirationValue);

            // Отправляем запрос
            const response = await fetch(form.action, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: params
            });

            const data = await response.json();

            // Обрабатываем ответ
            if (data.success) {
                const shareUrlContainer = document.getElementById('shareUrlContainer');
                const shareUrl = document.getElementById('shareUrl');

                if (shareUrlContainer && shareUrl) {
                    shareUrl.textContent = data.shareUrl;
                    shareUrlContainer.style.display = 'block';
                }
            } else {
                alert('Произошла ошибка: ' + (data.message || 'Не удалось создать ссылку'));
            }
        } catch (error) {
            console.error('Error sharing labs:', error);
            alert('Произошла ошибка при создании ссылки');
        }
    },

    /**
     * Копировать ссылку в буфер обмена
     * @param {HTMLElement} button - Кнопка копирования
     * @param {string} selector - Селектор элемента с текстом для копирования
     */
    copyShareUrl: (button, selector) => {
        const urlElement = document.querySelector(selector);
        if (!urlElement) return;

        const text = urlElement.textContent;

        // Копируем текст
        const textarea = document.createElement('input');
        textarea.value = text;
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);

        // Визуальный отклик
        const originalText = button.textContent;
        button.textContent = 'Скопировано!';
        setTimeout(() => {
            button.textContent = originalText;
        }, 2000);
    },

    /**
     * Инициализация формы поделиться
     */
    initShareForm: () => {
        const shareForm = document.getElementById('shareForm');
        const shareBtn = document.getElementById('shareButton');
        const closeBtn = document.querySelector('.close-btn');
        const modal = document.getElementById('shareModal');
        const copyBtn = document.getElementById('copyButton');

        if (shareForm) {
            shareForm.addEventListener('submit', function(e) {
                e.preventDefault();
                Labs.handleShareForm(this);
            });
        }

        if (shareBtn && modal) {
            shareBtn.addEventListener('click', function() {
                modal.style.display = 'flex';
            });
        }

        if (closeBtn && modal) {
            closeBtn.addEventListener('click', function() {
                modal.style.display = 'none';
            });
        }

        if (modal) {
            modal.addEventListener('click', function(event) {
                if (event.target === modal) {
                    modal.style.display = 'none';
                }
            });
        }

        if (copyBtn) {
            copyBtn.addEventListener('click', function() {
                Labs.copyShareUrl(this, '#shareUrl');
            });
        }
    }
};

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    // Инициализируем валидацию оценок
    Labs.initGradeValidation();

    // Инициализируем форму поделиться
    Labs.initShareForm();

    // Назначаем обработчики для переключения студентов
    document.querySelectorAll('.toggle-students').forEach(toggler => {
        toggler.addEventListener('click', function() {
            const containerId = this.getAttribute('data-container');
            if (containerId) {
                Labs.toggleStudents(containerId);
            } else {
                // Для обратной совместимости с существующим кодом
                const toggleFunc = this.getAttribute('onclick');
                if (toggleFunc && toggleFunc.includes('toggleStudents')) {
                    const match = toggleFunc.match(/toggleStudents\(['"](.*)['"]\)/);
                    if (match && match[1]) {
                        Labs.toggleStudents(match[1]);
                    }
                }
            }
        });
    });

    // Если в глобальной области нужна функция toggleStudents
    if (typeof window !== 'undefined') {
        window.toggleStudents = Labs.toggleStudents;
    }
});
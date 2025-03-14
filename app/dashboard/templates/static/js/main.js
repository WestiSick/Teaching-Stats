/**
 * main.js - Основные JavaScript функции для всего приложения
 */

// Вспомогательные функции для работы с DOM
const DOM = {
    /**
     * Получить элемент по ID
     * @param {string} id - ID элемента
     * @returns {HTMLElement|null} - Найденный элемент или null
     */
    getById: (id) => document.getElementById(id),

    /**
     * Получить элементы по селектору
     * @param {string} selector - CSS селектор
     * @returns {NodeList} - Список найденных элементов
     */
    getAll: (selector) => document.querySelectorAll(selector),

    /**
     * Получить первый элемент по селектору
     * @param {string} selector - CSS селектор
     * @returns {HTMLElement|null} - Найденный элемент или null
     */
    get: (selector) => document.querySelector(selector),

    /**
     * Добавить обработчик события
     * @param {HTMLElement|string} element - Элемент или селектор
     * @param {string} event - Название события
     * @param {Function} callback - Функция-обработчик
     */
    on: (element, event, callback) => {
        const el = typeof element === 'string' ? DOM.get(element) : element;
        if (el) {
            el.addEventListener(event, callback);
        }
    },

    /**
     * Добавить обработчик события для всех элементов
     * @param {NodeList|string} elements - Элементы или селектор
     * @param {string} event - Название события
     * @param {Function} callback - Функция-обработчик
     */
    onAll: (elements, event, callback) => {
        const els = typeof elements === 'string' ? DOM.getAll(elements) : elements;
        els.forEach(el => el.addEventListener(event, callback));
    },

    /**
     * Показать элемент
     * @param {HTMLElement|string} element - Элемент или селектор
     */
    show: (element) => {
        const el = typeof element === 'string' ? DOM.get(element) : element;
        if (el) {
            el.style.display = '';
        }
    },

    /**
     * Скрыть элемент
     * @param {HTMLElement|string} element - Элемент или селектор
     */
    hide: (element) => {
        const el = typeof element === 'string' ? DOM.get(element) : element;
        if (el) {
            el.style.display = 'none';
        }
    },

    /**
     * Переключить видимость элемента
     * @param {HTMLElement|string} element - Элемент или селектор
     */
    toggle: (element) => {
        const el = typeof element === 'string' ? DOM.get(element) : element;
        if (el) {
            el.style.display = el.style.display === 'none' ? '' : 'none';
        }
    },

    /**
     * Создать элемент
     * @param {string} tag - Название тега
     * @param {Object} attrs - Атрибуты элемента
     * @param {string|HTMLElement} content - Содержимое элемента
     * @returns {HTMLElement} - Созданный элемент
     */
    create: (tag, attrs, content) => {
        const element = document.createElement(tag);

        if (attrs) {
            Object.keys(attrs).forEach(key => {
                if (key === 'className') {
                    element.className = attrs[key];
                } else if (key === 'innerHTML') {
                    element.innerHTML = attrs[key];
                } else {
                    element.setAttribute(key, attrs[key]);
                }
            });
        }

        if (content) {
            if (typeof content === 'string') {
                element.innerHTML = content;
            } else {
                element.appendChild(content);
            }
        }

        return element;
    }
};

// Вспомогательные функции для работы с формами
const Forms = {
    /**
     * Сериализация формы в объект
     * @param {HTMLFormElement} form - Форма
     * @returns {Object} - Объект с данными формы
     */
    serialize: (form) => {
        const formData = new FormData(form);
        const data = {};

        for (let [key, value] of formData.entries()) {
            if (data[key] !== undefined) {
                if (!Array.isArray(data[key])) {
                    data[key] = [data[key]];
                }
                data[key].push(value);
            } else {
                data[key] = value;
            }
        }

        return data;
    },

    /**
     * Валидация формы
     * @param {HTMLFormElement} form - Форма
     * @returns {boolean} - Результат валидации
     */
    validate: (form) => {
        let isValid = true;

        // Проверка обязательных полей
        form.querySelectorAll('[required]').forEach(input => {
            if (!input.value.trim()) {
                isValid = false;
                input.classList.add('error');
            } else {
                input.classList.remove('error');
            }
        });

        return isValid;
    },

    /**
     * Очистка формы
     * @param {HTMLFormElement} form - Форма
     */
    reset: (form) => {
        form.reset();
        form.querySelectorAll('.error').forEach(input => {
            input.classList.remove('error');
        });
    }
};

// Функции для работы с запросами
const API = {
    /**
     * Выполнить GET запрос
     * @param {string} url - URL запроса
     * @param {Object} params - Параметры запроса
     * @returns {Promise<any>} - Промис с результатом запроса
     */
    get: async (url, params = {}) => {
        const queryString = Object.keys(params)
            .map(key => `${encodeURIComponent(key)}=${encodeURIComponent(params[key])}`)
            .join('&');

        const fullUrl = queryString ? `${url}?${queryString}` : url;

        try {
            const response = await fetch(fullUrl, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json'
                }
            });

            return await response.json();
        } catch (error) {
            console.error('API GET Error:', error);
            throw error;
        }
    },

    /**
     * Выполнить POST запрос
     * @param {string} url - URL запроса
     * @param {Object} data - Данные запроса
     * @returns {Promise<any>} - Промис с результатом запроса
     */
    post: async (url, data = {}) => {
        try {
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(data)
            });

            return await response.json();
        } catch (error) {
            console.error('API POST Error:', error);
            throw error;
        }
    },

    /**
     * Выполнить POST запрос с данными формы
     * @param {string} url - URL запроса
     * @param {FormData} formData - Данные формы
     * @returns {Promise<any>} - Промис с результатом запроса
     */
    postForm: async (url, formData) => {
        try {
            const response = await fetch(url, {
                method: 'POST',
                body: formData
            });

            return await response.json();
        } catch (error) {
            console.error('API POST Form Error:', error);
            throw error;
        }
    }
};

// Вспомогательные утилиты
const Utils = {
    /**
     * Копировать текст в буфер обмена
     * @param {string} text - Текст для копирования
     * @returns {boolean} - Результат операции
     */
    copyToClipboard: (text) => {
        const textarea = document.createElement('textarea');
        textarea.value = text;
        document.body.appendChild(textarea);
        textarea.select();

        let success = false;
        try {
            success = document.execCommand('copy');
        } catch (err) {
            console.error('Unable to copy to clipboard', err);
        }

        document.body.removeChild(textarea);
        return success;
    },

    /**
     * Форматировать дату в локальный формат
     * @param {string|Date} date - Дата для форматирования
     * @returns {string} - Отформатированная дата
     */
    formatDate: (date) => {
        const d = new Date(date);
        return d.toLocaleDateString('ru-RU');
    },

    /**
     * Предотвратить слишком частые вызовы функции
     * @param {Function} func - Функция для вызова
     * @param {number} wait - Время ожидания в мс
     * @returns {Function} - Функция с задержкой
     */
    debounce: (func, wait) => {
        let timeout;

        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };

            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }
};

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    console.log('Teaching Stats application loaded');

    // Обработчик для кнопок копирования
    DOM.onAll('.copy-btn', 'click', function() {
        const textToCopy = this.getAttribute('data-copy-text');
        if (textToCopy) {
            if (Utils.copyToClipboard(textToCopy)) {
                const originalText = this.textContent;
                this.textContent = 'Скопировано!';

                setTimeout(() => {
                    this.textContent = originalText;
                }, 2000);
            }
        }
    });
});
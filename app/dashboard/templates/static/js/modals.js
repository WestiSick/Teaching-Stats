/**
 * modals.js - Функции для работы с модальными окнами
 */

/**
 * Управление модальными окнами
 */
const Modal = {
    /**
     * Открыть модальное окно
     * @param {string} modalId - ID модального окна
     */
    open: (modalId) => {
        const modal = document.getElementById(modalId);
        if (modal) {
            modal.style.display = 'flex';
            document.body.style.overflow = 'hidden';
        }
    },

    /**
     * Закрыть модальное окно
     * @param {string} modalId - ID модального окна
     */
    close: (modalId) => {
        const modal = document.getElementById(modalId);
        if (modal) {
            modal.style.display = 'none';
            document.body.style.overflow = '';
        }
    },

    /**
     * Закрыть все модальные окна
     */
    closeAll: () => {
        const modals = document.querySelectorAll('.modal');
        modals.forEach(modal => {
            modal.style.display = 'none';
        });
        document.body.style.overflow = '';
    },

    /**
     * Инициализация модального окна
     * @param {string} modalId - ID модального окна
     * @param {string} openButtonId - ID кнопки открытия
     * @param {string} closeButtonSelector - Селектор кнопки закрытия
     */
    init: (modalId, openButtonId, closeButtonSelector = '.close-btn') => {
        const modal = document.getElementById(modalId);
        const openButton = document.getElementById(openButtonId);

        if (!modal || !openButton) return;

        // Открытие модального окна
        openButton.addEventListener('click', () => {
            Modal.open(modalId);
        });

        // Закрытие модального окна при клике на кнопку закрытия
        const closeButtons = modal.querySelectorAll(closeButtonSelector);
        closeButtons.forEach(button => {
            button.addEventListener('click', () => {
                Modal.close(modalId);
            });
        });

        // Закрытие модального окна при клике вне его содержимого
        modal.addEventListener('click', (event) => {
            if (event.target === modal) {
                Modal.close(modalId);
            }
        });
    },

    /**
     * Инициализация модального окна подтверждения
     * @param {string} modalId - ID модального окна
     * @param {Function} onConfirm - Функция, вызываемая при подтверждении
     * @param {Function} onCancel - Функция, вызываемая при отмене
     */
    initConfirm: (modalId, onConfirm, onCancel = null) => {
        const modal = document.getElementById(modalId);
        if (!modal) return;

        const confirmBtn = modal.querySelector('.confirm-btn');
        const cancelBtn = modal.querySelector('.cancel-btn');

        if (confirmBtn) {
            confirmBtn.addEventListener('click', () => {
                if (typeof onConfirm === 'function') {
                    onConfirm();
                }
                Modal.close(modalId);
            });
        }

        if (cancelBtn) {
            cancelBtn.addEventListener('click', () => {
                if (typeof onCancel === 'function') {
                    onCancel();
                }
                Modal.close(modalId);
            });
        }
    },

    /**
     * Создать и показать модальное окно с сообщением
     * @param {string} title - Заголовок
     * @param {string} message - Сообщение
     * @param {string} type - Тип сообщения (info, success, error, warning)
     */
    showMessage: (title, message, type = 'info') => {
        // Удаляем существующее модальное окно, если есть
        const existingModal = document.getElementById('messageModal');
        if (existingModal) {
            existingModal.remove();
        }

        // Определение класса в зависимости от типа
        let typeClass = '';
        switch (type) {
            case 'success':
                typeClass = 'text-success';
                break;
            case 'error':
                typeClass = 'text-danger';
                break;
            case 'warning':
                typeClass = 'text-warning';
                break;
            default:
                typeClass = 'text-primary';
        }

        // Создаем элементы модального окна
        const modal = document.createElement('div');
        modal.id = 'messageModal';
        modal.className = 'modal';
        modal.style.display = 'flex';

        modal.innerHTML = `
      <div class="modal-content">
        <div class="modal-header">
          <h3 class="modal-title ${typeClass}">${title}</h3>
          <button class="close-btn">&times;</button>
        </div>
        <div class="modal-body">
          <p>${message}</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-primary close-modal-btn">OK</button>
        </div>
      </div>
    `;

        // Добавляем модальное окно в DOM
        document.body.appendChild(modal);

        // Обработчики событий
        const closeBtn = modal.querySelector('.close-btn');
        const closeModalBtn = modal.querySelector('.close-modal-btn');

        closeBtn.addEventListener('click', () => {
            modal.style.display = 'none';
            modal.remove();
        });

        closeModalBtn.addEventListener('click', () => {
            modal.style.display = 'none';
            modal.remove();
        });

        modal.addEventListener('click', (event) => {
            if (event.target === modal) {
                modal.style.display = 'none';
                modal.remove();
            }
        });

        // Блокируем прокрутку страницы
        document.body.style.overflow = 'hidden';
    }
};

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    // Инициализация всех модальных окон на странице
    document.querySelectorAll('[data-modal]').forEach(button => {
        const modalId = button.getAttribute('data-modal');
        Modal.init(modalId, button.id);
    });

    // Инициализация всех модальных окон подтверждения
    document.querySelectorAll('[data-confirm-modal]').forEach(button => {
        const modalId = button.getAttribute('data-confirm-modal');
        const actionUrl = button.getAttribute('data-action');

        Modal.initConfirm(modalId, () => {
            if (actionUrl) {
                window.location.href = actionUrl;
            }
        });
    });
});
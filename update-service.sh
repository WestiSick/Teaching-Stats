#!/bin/bash

# Скрипт для обновления только выбранного сервиса Docker

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Функция вывода справки
show_help() {
    echo "Использование: $0 [опции] [название_сервиса]"
    echo ""
    echo "Опции:"
    echo "  -h, --help     Показать справку"
    echo ""
    echo "Сервисы:"
    echo "  dashboard      Обновить только сервис dashboard"
    echo "  tickets        Обновить только сервис tickets"
    echo "  schedule       Обновить только сервис schedule"
    echo "  all            Обновить все сервисы (по умолчанию)"
    echo ""
    echo "Примеры:"
    echo "  $0 dashboard   Обновить только dashboard"
    echo "  $0 tickets     Обновить только tickets"
    echo "  $0 schedule    Обновить только schedule"
    echo "  $0 all         Обновить все сервисы"
    echo "  $0             Обновить все сервисы (аналогично 'all')"
}

# Парсинг аргументов
SERVICE="all"

if [ "$1" == "-h" ] || [ "$1" == "--help" ]; then
    show_help
    exit 0
elif [ ! -z "$1" ]; then
    SERVICE="$1"
fi

# Проверка наличия Docker и Docker Compose
if ! command -v docker &> /dev/null || ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}Docker или Docker Compose не установлены. Пожалуйста, установите их перед продолжением.${NC}"
    exit 1
fi

# Проверка наличия файла docker-compose.yml
if [ ! -f "docker-compose.yml" ]; then
    echo -e "${RED}Файл docker-compose.yml не найден в текущей директории.${NC}"
    echo -e "${RED}Пожалуйста, убедитесь, что вы находитесь в корневой директории проекта.${NC}"
    exit 1
fi

# Обновление всех сервисов
if [ "$SERVICE" == "all" ]; then
    echo -e "${YELLOW}Обновление всех сервисов...${NC}"

    echo -e "${YELLOW}Останавливаем контейнеры...${NC}"
    docker-compose down
    echo -e "${GREEN}Контейнеры остановлены.${NC}"

    echo -e "${YELLOW}Пересобираем образы...${NC}"
    docker-compose build --no-cache dashboard tickets schedule
    echo -e "${GREEN}Образы пересобраны.${NC}"

    echo -e "${YELLOW}Запускаем обновленные контейнеры...${NC}"
    docker-compose up -d
    echo -e "${GREEN}Контейнеры запущены.${NC}"

# Обновление только dashboard
elif [ "$SERVICE" == "dashboard" ]; then
    echo -e "${YELLOW}Обновление сервиса dashboard...${NC}"

    echo -e "${YELLOW}Останавливаем контейнер dashboard...${NC}"
    docker-compose stop dashboard
    echo -e "${GREEN}Контейнер dashboard остановлен.${NC}"

    echo -e "${YELLOW}Пересобираем образ dashboard...${NC}"
    docker-compose build --no-cache dashboard
    echo -e "${GREEN}Образ dashboard пересобран.${NC}"

    echo -e "${YELLOW}Запускаем обновленный контейнер dashboard...${NC}"
    docker-compose up -d dashboard
    echo -e "${GREEN}Контейнер dashboard запущен.${NC}"

# Обновление только tickets
elif [ "$SERVICE" == "tickets" ]; then
    echo -e "${YELLOW}Обновление сервиса tickets...${NC}"

    echo -e "${YELLOW}Останавливаем контейнер tickets...${NC}"
    docker-compose stop tickets
    echo -e "${GREEN}Контейнер tickets остановлен.${NC}"

    echo -e "${YELLOW}Пересобираем образ tickets...${NC}"
    docker-compose build --no-cache tickets
    echo -e "${GREEN}Образ tickets пересобран.${NC}"

    echo -e "${YELLOW}Запускаем обновленный контейнер tickets...${NC}"
    docker-compose up -d tickets
    echo -e "${GREEN}Контейнер tickets запущен.${NC}"

# Обновление только schedule
elif [ "$SERVICE" == "schedule" ]; then
    echo -e "${YELLOW}Обновление сервиса schedule...${NC}"

    echo -e "${YELLOW}Останавливаем контейнер schedule...${NC}"
    docker-compose stop schedule
    echo -e "${GREEN}Контейнер schedule остановлен.${NC}"

    echo -e "${YELLOW}Пересобираем образ schedule...${NC}"
    docker-compose build --no-cache schedule
    echo -e "${GREEN}Образ schedule пересобран.${NC}"

    echo -e "${YELLOW}Запускаем обновленный контейнер schedule...${NC}"
    docker-compose up -d schedule
    echo -e "${GREEN}Контейнер schedule запущен.${NC}"

else
    echo -e "${RED}Неизвестный сервис: $SERVICE${NC}"
    show_help
    exit 1
fi

# Проверка статуса контейнеров
echo -e "${YELLOW}Проверка статуса контейнеров:${NC}"
docker-compose ps

echo -e "${GREEN}Обновление завершено!${NC}"
echo -e "${YELLOW}Для просмотра логов используйте:${NC}"
echo -e "  - docker-compose logs -f dashboard"
echo -e "  - docker-compose logs -f tickets"
echo -e "  - docker-compose logs -f schedule"

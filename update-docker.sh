#!/bin/bash

# Скрипт для обновления Docker-контейнеров после изменения кода

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Обновление Docker-контейнеров для Teaching Stats...${NC}"

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

# Остановка контейнеров
echo -e "${YELLOW}Останавливаем контейнеры...${NC}"
docker-compose down
echo -e "${GREEN}Контейнеры остановлены.${NC}"

# Пересборка образов без использования кэша
echo -e "${YELLOW}Пересобираем образы...${NC}"
docker-compose build --no-cache dashboard tickets schedule
echo -e "${GREEN}Образы пересобраны.${NC}"

# Запуск контейнеров
echo -e "${YELLOW}Запускаем обновленные контейнеры...${NC}"
docker-compose up -d
echo -e "${GREEN}Контейнеры запущены.${NC}"

# Проверка статуса контейнеров
echo -e "${YELLOW}Проверка статуса контейнеров:${NC}"
docker-compose ps

echo -e "${GREEN}Обновление завершено!${NC}"
echo -e "${YELLOW}Для просмотра логов используйте:${NC}"
echo -e "  - docker-compose logs -f dashboard"
echo -e "  - docker-compose logs -f tickets"
echo -e "  - docker-compose logs -f schedule"
echo -e "  - docker-compose logs -f nginx"

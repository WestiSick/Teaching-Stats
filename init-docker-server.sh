#!/bin/bash

# Скрипт для инициализации Docker-окружения микросервисов Teaching Stats

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Начало настройки Docker-окружения микросервисов для Teaching Stats...${NC}"

# Проверка наличия Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker не установлен. Пожалуйста, установите Docker перед продолжением.${NC}"
    exit 1
fi

# Проверка наличия Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}Docker Compose не установлен. Пожалуйста, установите Docker Compose перед продолжением.${NC}"
    exit 1
fi

echo -e "${GREEN}Docker и Docker Compose установлены. Продолжаем...${NC}"

# Создание директории для файлов Nginx
echo -e "${YELLOW}Создание директории для конфигурации Nginx...${NC}"
mkdir -p nginx
echo -e "${GREEN}Директория nginx создана.${NC}"

# Проверка наличия файла конфигурации config.go и его копирование при необходимости
if [ -f "config/config.go" ]; then
    echo -e "${YELLOW}Создание резервной копии config.go...${NC}"
    cp config/config.go config/config.go.bak
    echo -e "${GREEN}Резервная копия создана: config/config.go.bak${NC}"

    echo -e "${YELLOW}Обновление config.go для поддержки Docker...${NC}"
    # Обновить файл конфигурации
    cat > config/config.go << 'EOF'
package config

import (
	"fmt"
	"os"

	"github.com/gorilla/sessions"
)

// Application constants
const (
	CookieStoreKey = "super-secret-key"
	SessionName    = "session-name"
)

// Get DB connection string from environment or use default
var DBConnectionString = getEnv("DB_CONNECTION_STRING", fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
	getEnv("DB_USER", "postgres"),
	getEnv("DB_PASSWORD", "vadimvadimvadim"),
	getEnv("DB_HOST", "localhost"),
	getEnv("DB_PORT", "5432"),
	getEnv("DB_NAME", "teacher")))

// Store is the global store for sessions (shared with Teaching Stats)
var Store = sessions.NewCookieStore([]byte(CookieStoreKey))

// TicketSystemPort is the port on which the ticket system runs
const TicketSystemPort = 8090

// TicketStatusValues defines the valid status values for tickets
var TicketStatusValues = []string{"Новый", "Открытый", "В работе", "Решенный", "Закрыт"}

// TicketPriorityValues defines the valid priority values for tickets
var TicketPriorityValues = []string{"Низкий", "Средний", "Высокий", "Критический"}

// TicketCategoryValues defines the valid category values for tickets
var TicketCategoryValues = []string{"Технический", "Административный", "Аккаунт", "Особенность", "Баг", "Другая"}

// AttachmentStoragePath defines where file attachments are stored
const AttachmentStoragePath = "./attachments"

// MaxFileSize defines the maximum size for uploaded files (5MB)
const MaxFileSize = 5 * 1024 * 1024

// Helper function to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
EOF
    echo -e "${GREEN}Файл config.go обновлен.${NC}"
else
    echo -e "${RED}Файл config/config.go не найден. Пожалуйста, убедитесь, что вы находитесь в корневой директории проекта.${NC}"
    exit 1
fi

# Создание Dockerfile для dashboard
echo -e "${YELLOW}Создание Dockerfile.dashboard...${NC}"
cat > Dockerfile.dashboard << 'EOF'
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install required packages
RUN apk add --no-cache git

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the dashboard application
RUN go build -o dashboard ./app/dashboard/cmd/server/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder stage
COPY --from=builder /app/dashboard .

# Copy templates and other necessary files
COPY --from=builder /app/app/dashboard/templates ./app/dashboard/templates

# Create attachments directory
RUN mkdir -p /app/attachments

# Expose dashboard port
EXPOSE 8080

# Set up the entrypoint
ENTRYPOINT ["./dashboard"]
EOF
echo -e "${GREEN}Dockerfile.dashboard создан.${NC}"

# Создание Dockerfile для tickets
echo -e "${YELLOW}Создание Dockerfile.tickets...${NC}"
cat > Dockerfile.tickets << 'EOF'
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install required packages
RUN apk add --no-cache git

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the tickets application
RUN go build -o tickets ./app/tickets/cmd/server/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder stage
COPY --from=builder /app/tickets .

# Copy templates and other necessary files
COPY --from=builder /app/app/tickets/templates ./app/tickets/templates

# Create attachments directory
RUN mkdir -p /app/attachments

# Expose tickets port
EXPOSE 8090

# Set up the entrypoint
ENTRYPOINT ["./tickets"]
EOF
echo -e "${GREEN}Dockerfile.tickets создан.${NC}"

# Создание Dockerfile для schedule
echo -e "${YELLOW}Создание Dockerfile.schedule...${NC}"
cat > Dockerfile.schedule << 'EOF'
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install required packages
RUN apk add --no-cache git

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the schedule application
RUN go build -o schedule ./app/schedule/cmd/server/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder stage
COPY --from=builder /app/schedule .

# Copy templates and other necessary files
COPY --from=builder /app/app/schedule/templates ./app/schedule/templates

# Expose schedule port
EXPOSE 8091

# Set up the entrypoint
ENTRYPOINT ["./schedule"]
EOF
echo -e "${GREEN}Dockerfile.schedule создан.${NC}"

# Создание docker-compose.yml
echo -e "${YELLOW}Создание docker-compose.yml...${NC}"
cat > docker-compose.yml << 'EOF'
version: '3.8'

services:
  dashboard:
    build:
      context: .
      dockerfile: Dockerfile.dashboard
    container_name: teaching-stats-dashboard
    restart: always
    depends_on:
      - db
    environment:
      - DB_HOST=db
      - DB_USER=postgres
      - DB_PASSWORD=vadimvadimvadim
      - DB_NAME=teacher
      - DB_PORT=5432
    volumes:
      - attachments:/app/attachments
    networks:
      - app-network

  tickets:
    build:
      context: .
      dockerfile: Dockerfile.tickets
    container_name: teaching-stats-tickets
    restart: always
    depends_on:
      - db
      - dashboard
    environment:
      - DB_HOST=db
      - DB_USER=postgres
      - DB_PASSWORD=vadimvadimvadim
      - DB_NAME=teacher
      - DB_PORT=5432
    volumes:
      - attachments:/app/attachments
    networks:
      - app-network

  schedule:
    build:
      context: .
      dockerfile: Dockerfile.schedule
    container_name: teaching-stats-schedule
    restart: always
    depends_on:
      - db
      - dashboard
    environment:
      - DB_HOST=db
      - DB_USER=postgres
      - DB_PASSWORD=vadimvadimvadim
      - DB_NAME=teacher
      - DB_PORT=5432
    networks:
      - app-network

  db:
    image: postgres:14-alpine
    container_name: teaching-stats-db
    restart: always
    environment:
      - POSTGRES_PASSWORD=vadimvadimvadim
      - POSTGRES_USER=postgres
      - POSTGRES_DB=teacher
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - app-network

  nginx:
    image: nginx:alpine
    container_name: teaching-stats-nginx
    restart: always
    ports:
      - "80:80"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/conf.d/default.conf
    depends_on:
      - dashboard
      - tickets
      - schedule
    networks:
      - app-network

  adminer:
    image: adminer
    container_name: teaching-stats-adminer
    restart: always
    ports:
      - "8888:8080"
    depends_on:
      - db
    networks:
      - app-network

volumes:
  postgres-data:
  attachments:

networks:
  app-network:
    driver: bridge
EOF
echo -e "${GREEN}docker-compose.yml создан.${NC}"

# Создание nginx.conf с оптимизированной конфигурацией для schedule
echo -e "${YELLOW}Создание конфигурации для Nginx...${NC}"
cat > nginx/nginx.conf << 'EOF'
server {
    server_name vg.vadimbuzdin.ru www.vg.vadimbuzdin.ru 89.150.34.90 ;

    location / {
        proxy_pass http://dashboard:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /tickets {
        proxy_pass http://tickets:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Обработка корневого маршрута /schedule
    location = /schedule {
        proxy_pass http://schedule:8091/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Original-URI $request_uri;
    }

    # Перенаправление всех подмаршрутов /schedule/ с сохранением пути
    location ~ ^/schedule(/.*)$ {
        proxy_pass http://schedule:8091$1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Original-URI $request_uri;
    }

    # Обработка POST-запросов для /schedule/add-lesson
    location = /schedule/add-lesson {
        proxy_pass http://schedule:8091/add-lesson;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Original-URI $request_uri;
    }

    location /attachments/ {
        proxy_pass http://dashboard:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Для статических файлов
    location /static/ {
        proxy_pass http://dashboard:8080;
    }

    # Увеличиваем размер загружаемых файлов
    client_max_body_size 10M;

    # Увеличение таймаутов для API-запросов, которые могут выполняться долго
    proxy_connect_timeout 180s;
    proxy_send_timeout 180s;
    proxy_read_timeout 180s;
}
EOF
echo -e "${GREEN}Файл nginx/nginx.conf создан.${NC}"

# Создание директории для вложений, если она еще не существует
echo -e "${YELLOW}Создание директории для вложений...${NC}"
mkdir -p attachments
echo -e "${GREEN}Директория attachments создана.${NC}"

# Создание скрипта обновления для определенного сервиса
echo -e "${YELLOW}Создание скрипта update-service.sh...${NC}"
cat > update-service.sh << 'EOF'
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
    echo "  nginx          Обновить только конфигурацию Nginx"
    echo "  all            Обновить все сервисы (по умолчанию)"
    echo ""
    echo "Примеры:"
    echo "  $0 dashboard   Обновить только dashboard"
    echo "  $0 tickets     Обновить только tickets"
    echo "  $0 schedule    Обновить только schedule"
    echo "  $0 nginx       Обновить только Nginx конфигурацию"
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

# Обновление только Nginx
elif [ "$SERVICE" == "nginx" ]; then
    echo -e "${YELLOW}Обновление конфигурации Nginx...${NC}"

    echo -e "${YELLOW}Останавливаем контейнер nginx...${NC}"
    docker-compose stop nginx
    echo -e "${GREEN}Контейнер nginx остановлен.${NC}"

    echo -e "${YELLOW}Запускаем обновленный контейнер nginx...${NC}"
    docker-compose up -d nginx
    echo -e "${GREEN}Контейнер nginx запущен.${NC}"

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
echo -e "  - docker-compose logs -f nginx"
EOF
chmod +x update-service.sh
echo -e "${GREEN}Скрипт update-service.sh создан и сделан исполняемым.${NC}"

# Создание скрипта для общего обновления
echo -e "${YELLOW}Создание скрипта update-docker.sh...${NC}"
cat > update-docker.sh << 'EOF'
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
EOF
chmod +x update-docker.sh
echo -e "${GREEN}Скрипт update-docker.sh создан и сделан исполняемым.${NC}"

echo -e "${YELLOW}Запуск Docker Compose...${NC}"
docker-compose up -d

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Docker-окружение успешно настроено и запущено!${NC}"
    echo -e "${GREEN}Приложение доступно по адресу: http://localhost${NC}"
    echo ""
    echo -e "${YELLOW}Доступные сервисы:${NC}"
    echo -e "  - Dashboard: http://localhost/"
    echo -e "  - Tickets: http://localhost/tickets"
    echo -e "  - Schedule: http://localhost/schedule"
    echo -e "  - PGAdmin: http://localhost:5050"
    echo ""
    echo -e "${YELLOW}Для просмотра логов используйте:${NC} docker-compose logs -f"
    echo -e "${YELLOW}Для остановки контейнеров используйте:${NC} docker-compose down"
else
    echo -e "${RED}Произошла ошибка при запуске Docker Compose. Проверьте логи выше.${NC}"
fi
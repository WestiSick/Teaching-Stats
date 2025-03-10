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
	getEnv("DB_PASSWORD", "vadimvadimvadim13"),
	getEnv("DB_HOST", "localhost"),
	getEnv("DB_PORT", "5432"),
	getEnv("DB_NAME", "teacher")))

// Store is the global store for sessions (shared with Teaching Stats)
var Store = sessions.NewCookieStore([]byte(CookieStoreKey))

// TicketSystemPort is the port on which the ticket system runs
const TicketSystemPort = 8090

// TicketStatusValues defines the valid status values for tickets
var TicketStatusValues = []string{"New", "Open", "InProgress", "Resolved", "Closed"}

// TicketPriorityValues defines the valid priority values for tickets
var TicketPriorityValues = []string{"Low", "Medium", "High", "Critical"}

// TicketCategoryValues defines the valid category values for tickets
var TicketCategoryValues = []string{"Technical", "Administrative", "Account", "Feature", "Bug", "Other"}

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
      - DB_PASSWORD=vadimvadimvadim13
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
      - DB_PASSWORD=vadimvadimvadim13
      - DB_NAME=teacher
      - DB_PORT=5432
    volumes:
      - attachments:/app/attachments
    networks:
      - app-network

  db:
    image: postgres:14-alpine
    container_name: teaching-stats-db
    restart: always
    environment:
      - POSTGRES_PASSWORD=vadimvadimvadim13
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
    networks:
      - app-network

  pgadmin:
    image: dpage/pgadmin4
    container_name: teaching-stats-pgadmin
    restart: always
    environment:
      - PGADMIN_DEFAULT_EMAIL=admin@example.com
      - PGADMIN_DEFAULT_PASSWORD=adminpassword
      - PGADMIN_LISTEN_PORT=5050
    ports:
      - "5050:5050"
    volumes:
      - pgadmin-data:/var/lib/pgadmin
    depends_on:
      - db
    networks:
      - app-network

volumes:
  postgres-data:
  attachments:
  pgadmin-data:

networks:
  app-network:
    driver: bridge
EOF
echo -e "${GREEN}docker-compose.yml создан.${NC}"

# Создание nginx.conf
echo -e "${YELLOW}Создание конфигурации для Nginx...${NC}"
cat > nginx/nginx.conf << 'EOF'
server {
    listen 80;
    server_name localhost;

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
}
EOF
echo -e "${GREEN}Файл nginx/nginx.conf создан.${NC}"

# Создание директории для вложений, если она еще не существует
echo -e "${YELLOW}Создание директории для вложений...${NC}"
mkdir -p attachments
echo -e "${GREEN}Директория attachments создана.${NC}"

echo -e "${YELLOW}Запуск Docker Compose...${NC}"
docker-compose up -d

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Docker-окружение успешно настроено и запущено!${NC}"
    echo -e "${GREEN}Приложение доступно по адресу: http://localhost${NC}"
    echo ""
    echo -e "${YELLOW}Для просмотра логов используйте:${NC} docker-compose logs -f"
    echo -e "${YELLOW}Для остановки контейнеров используйте:${NC} docker-compose down"
else
    echo -e "${RED}Произошла ошибка при запуске Docker Compose. Проверьте логи выше.${NC}"
fi
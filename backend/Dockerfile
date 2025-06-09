# Используем базовый образ Go для сборки
FROM golang:1.20 AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы go.mod и go.sum для загрузки зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download && go get github.com/gorilla/websocket

# Устанавливаем дополнительные зависимости
RUN go mod tidy
RUN go mod download

# Копируем остальной исходный код
COPY . .

# Сборка приложения
RUN go build -o main . && chmod +x main

# Используем Node.js для запуска mockGenerator.js
FROM node:16-alpine AS node-builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем mockGenerator.js и зависимости
COPY ./mockGenerator.js ./package.json ./package-lock.json ./

# Устанавливаем зависимости Node.js
RUN npm install

# Финальный образ с Alpine и Node.js
FROM alpine:3.18

# Устанавливаем совместимые библиотеки и Node.js
RUN apk add --no-cache libc6-compat nodejs npm

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы из предыдущих слоёв
# Копируем Go-приложение
COPY --from=builder /app/main .
# Копируем mockGenerator.js
COPY --from=node-builder /app/mockGenerator.js .
# Копируем зависимости Node.js
COPY --from=node-builder /app/node_modules ./node_modules

# Открываем порт для связи
EXPOSE 8080

# Устанавливаем переменные окружения для подключения к базе данных
ENV DB_HOST=postgres
ENV DB_PORT=5432
ENV DB_USER=gitverse_user
ENV DB_PASSWORD=gitverse_password
ENV DB_NAME=gitverse_db

# Запуск mockGenerator.js и Go-приложения
CMD ["sh", "-c", "node mockGenerator.js & ./main"]

# Используем Node.js для сборки Next.js приложения
FROM node:18 as builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Устанавливаем переменные окружения
ENV NODE_ENV=production

# Копируем package.json и package-lock.json и устанавливаем зависимости
COPY package.json package-lock.json ./
RUN npm install --legacy-peer-deps

# Копируем исходный код
COPY . .

# Сборка приложения Next.js
RUN npm run build

# Финальный образ для развертывания
FROM node:18-alpine
WORKDIR /app

# Копируем только необходимые файлы из сборочного образа
COPY --from=builder /app/.next .next
COPY --from=builder /app/node_modules node_modules
COPY --from=builder /app/package.json package.json
COPY --from=builder /app/public public

# Открываем порт для связи
EXPOSE 3000

# Запуск сервера Next.js
CMD ["npm", "start"]

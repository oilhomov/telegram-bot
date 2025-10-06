# Этап сборки Go
FROM golang:1.22 AS builder

WORKDIR /app

# Копируем модули отдельно (чтобы кешировались)
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Сборка статического бинаря (без зависимостей от glibc)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bot main.go

# Финальный минимальный образ
FROM alpine:latest

WORKDIR /app

# Устанавливаем зависимости для yt-dlp
RUN apk add --no-cache curl ffmpeg python3

# Скачиваем последнюю версию yt-dlp
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp \
    -o /usr/local/bin/yt-dlp && chmod +x /usr/local/bin/yt-dlp

# Копируем бинарь бота
COPY --from=builder /app/bot /app/bot

# Если будут cookies через переменную окружения, можно сохранить
ENV YTDLP_COOKIES=""
RUN if [ -n "$YTDLP_COOKIES" ]; then echo "$YTDLP_COOKIES" > /app/cookies.txt; fi

# Запуск бота
CMD ["/app/bot"]

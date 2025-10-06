# Сборка бота
FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o bot main.go

# Финальный образ
FROM debian:bullseye-slim
WORKDIR /app

# Устанавливаем yt-dlp
RUN apt-get update && apt-get install -y curl ffmpeg python3 \
    && curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp \
    && chmod +x /usr/local/bin/yt-dlp

# Копируем бинарь бота
COPY --from=builder /app/bot /app/bot

# Если есть переменная окружения с cookies — сохраняем в файл
ENV YTDLP_COOKIES=""
RUN if [ -n "$YTDLP_COOKIES" ]; then echo "$YTDLP_COOKIES" > /app/cookies.txt; fi

CMD ["/app/bot"]

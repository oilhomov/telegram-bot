# Финальный образ
FROM debian:bullseye-slim

WORKDIR /app

# Устанавливаем зависимости для yt-dlp
RUN apt-get update && apt-get install -y \
    curl ffmpeg python3 python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Ставим последнюю версию yt-dlp
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp \
    -o /usr/local/bin/yt-dlp \
    && chmod a+rx /usr/local/bin/yt-dlp

# Копируем бинарник бота
COPY --from=builder /app/bot /app/bot

# Если в переменной окружения есть cookies, сохраняем их в файл
RUN echo '#!/bin/sh\n\
if [ ! -z "$YTDLP_COOKIES" ]; then\n\
  echo "$YTDLP_COOKIES" > /app/cookies.txt\n\
fi\n\
exec /app/bot' > /app/entrypoint.sh && chmod +x /app/entrypoint.sh

# Запускаем через entrypoint
CMD ["/app/entrypoint.sh"]

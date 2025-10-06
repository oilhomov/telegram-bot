# Telegram Downloader Bot (Go + yt-dlp + Docker)

## Описание
Публичный Telegram-бот. Пришли ссылку (YouTube / Instagram Reels / TikTok ...), бот предложит кнопки "Видео" / "Аудио" и отправит файл прямо в чат. Docker-ready, можно деплоить на Render / VPS.

## Файлы
- `main.go` — исходный код
- `Dockerfile` — сборка образа
- `docker-compose.yml` — для локального теста
- `.env.example` — пример переменных окружения
- `render.yaml` — вариант для Render

## Локальная проверка (Windows + Docker Desktop)
1. Скопируйте `.env.example` → `.env` и вставьте ваш `BOT_TOKEN`.
2. Откройте Git Bash / PowerShell:
   ```bash
   docker compose up --build

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const defaultMaxFileSize = 2 * 1024 * 1024 * 1024 // 2GB
const defaultYtdlpTimeout = 600                    // сек

func getenvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return def
}
func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatalln("Требуется BOT_TOKEN в окружении")
	}
	// COOKIES_PATH может быть пустым
	cookiesPath := os.Getenv("COOKIES_PATH")
	maxSize := getenvInt64("MAX_FILE_SIZE", int64(defaultMaxFileSize))
	ytdlpTimeout := getenvInt("YTDLP_TIMEOUT_SECONDS", defaultYtdlpTimeout)

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}
	log.Printf("Бот запущен: @%s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		// Обрабатываем команды параллельно
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "video":
				go handleDownload(bot, update.Message, "video", cookiesPath, maxSize, ytdlpTimeout)
			case "audio":
				go handleDownload(bot, update.Message, "audio", cookiesPath, maxSize, ytdlpTimeout)
			case "help":
				sendHelp(bot, update.Message)
			default:
				_, _ = bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неизвестная команда. Используй /help"))
			}
		}
	}
}

func sendHelp(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	text := "🎬 Команды:\n" +
		"/video <url> — скачать видео (YouTube, Instagram Reels, TikTok...)\n" +
		"/audio <url> — скачать аудио в mp3\n\n" +
		"Если нужен доступ к приватным/ограниченным видео — укажи COOKIES_PATH в переменных окружения."
	_, _ = bot.Send(tgbotapi.NewMessage(msg.Chat.ID, text))
}

func handleDownload(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, mode string, cookiesPath string, maxSize int64, timeoutSeconds int) {
	chatID := msg.Chat.ID
	url := msg.CommandArguments()
	if url == "" {
		_, _ = bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Использование: /%s <ссылка>", mode)))
		return
	}

	statusMsg, _ := bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("📥 Начинаю %s: %s", mode, url)))

	// Проверим cookies (если указан)
	if cookiesPath != "" {
		if _, err := os.Stat(cookiesPath); os.IsNotExist(err) {
			_, _ = bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ COOKIES не найден по пути %s — продолжу без авторизации", cookiesPath)))
			cookiesPath = ""
		}
	}

	// уникальная временная папка
	tmpDir, err := os.MkdirTemp("", "tgdl-*")
	if err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка: %v", err)))
		return
	}
	defer os.RemoveAll(tmpDir)

	outPattern := filepath.Join(tmpDir, "%(title)s.%(ext)s")

	// сформируем args
	var args []string
	if mode == "audio" {
		args = []string{"-f", "bestaudio", "-x", "--audio-format", "mp3", "-o", outPattern, url}
	} else {
		args = []string{"-f", "bestvideo+bestaudio/best", "-o", outPattern, url}
	}
	if cookiesPath != "" {
		args = append([]string{"--cookies", cookiesPath}, args...)
	}

	// запустим yt-dlp с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка yt-dlp: %v", err)))
		return
	}

	// найдём последний файл
	filePath, err := findLatestFile(tmpDir)
	if err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(chatID, "❌ Не найден файл после скачивания"))
		return
	}

	// проверим размер
	info, _ := os.Stat(filePath)
	if info.Size() > maxSize {
		_, _ = bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Файл слишком большой для отправки."))
		return
	}

	_, _ = bot.Send(tgbotapi.NewMessage(chatID, "📤 Отправляю..."))
	f := tgbotapi.FilePath(filePath)
	doc := tgbotapi.NewDocument(chatID, f)
	if _, err := bot.Send(doc); err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка отправки: %v", err)))
		return
	}

	_, _ = bot.Send(tgbotapi.NewMessage(chatID, "✅ Готово"))
	// обновим статус сообщения
	_, _ = bot.EditMessageText(tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    chatID,
			MessageID: statusMsg.MessageID,
		},
		Text: "Завершено",
	})
}

func findLatestFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var latest string
	var latestTime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = filepath.Join(dir, e.Name())
		}
	}
	if latest == "" {
		return "", fmt.Errorf("нет файлов")
	}
	return latest, nil
}

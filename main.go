package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	maxFileSize = 2 * 1024 * 1024 * 1024 // 2 GB
	ytTimeout   = 15 * time.Minute
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN not set in environment")
	}

	// ensure yt-dlp exists
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		log.Fatal("yt-dlp not found in PATH. Install yt-dlp in the image.")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
	}
	bot.Debug = false
	log.Printf("Bot started: @%s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			go handleMessage(bot, update.Message)
		}
		if update.CallbackQuery != nil {
			go handleCallback(bot, update.CallbackQuery)
		}
	}
}

func handleMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	// commands
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			bot.Send(tgbotapi.NewMessage(chatID, "Привет! Пришли ссылку на видео (YouTube / Instagram Reels / TikTok и т.д.), я предложу скачать как Видео или Аудио."))
			return
		case "help":
			bot.Send(tgbotapi.NewMessage(chatID, "/start — старт\n/help — помощь\nПросто пришли ссылку на видео."))
			return
		}
	}

	// if message contains a link
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}
	if !strings.HasPrefix(text, "http://") && !strings.HasPrefix(text, "https://") {
		bot.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, пришли ссылку, начинающуюся с http:// или https://"))
		return
	}

	// Save URL in chat context (in-memory) by message ID (simple approach)
	// We'll use callback data to pass the URL via ephemeral storage: store on file per chat is unnecessary here.
	// Simpler: include the URL in the callback data is unsafe (too long). Instead we use a temp file map — but for simplicity we'll store in a simple file under /tmp keyed by chatID.
	keyFile := filepath.Join(os.TempDir(), fmt.Sprintf("tgurl_%d.txt", chatID))
	_ = os.WriteFile(keyFile, []byte(text), 0600)

	// show keyboard
	videoBtn := tgbotapi.NewInlineKeyboardButtonData("🎬 Скачать видео", "download:video")
	audioBtn := tgbotapi.NewInlineKeyboardButtonData("🎵 Скачать аудио (mp3)", "download:audio")
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(videoBtn),
		tgbotapi.NewInlineKeyboardRow(audioBtn),
	)
	msgCfg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Выбери формат для: %s", text))
	msgCfg.ReplyMarkup = kb
	if _, err := bot.Send(msgCfg); err != nil {
		log.Printf("send keyboard failed: %v", err)
	}
}

func handleCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	chatID := cb.Message.Chat.ID
	data := cb.Data

	// acknowledge callback to remove loader
	ack := tgbotapi.NewCallback(cb.ID, "Запрос принят — начинаю загрузку...")
	if _, err := bot.Request(ack); err != nil {
		log.Printf("callback ack failed: %v", err)
	}

	// read stored url
	keyFile := filepath.Join(os.TempDir(), fmt.Sprintf("tgurl_%d.txt", chatID))
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Не найдена ссылка — пришлите ссылку снова."))
		return
	}
	url := strings.TrimSpace(string(raw))
	if url == "" {
		bot.Send(tgbotapi.NewMessage(chatID, "Пустая ссылка — пришлите ссылку снова."))
		return
	}

	// inform user
	format := strings.TrimPrefix(data, "download:")
	bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Начинаю %s для: %s", format, url)))

	// run download in goroutine
	go func() {
		if err := downloadAndSend(bot, chatID, url, format); err != nil {
			log.Printf("download/send error: %v", err)
			bot.Send(tgbotapi.NewMessage(chatID, "Ошибка: "+err.Error()))
		}
		// remove stored url after job
		_ = os.Remove(keyFile)
	}()
}

func downloadAndSend(bot *tgbotapi.BotAPI, chatID int64, url, mode string) error {
	tmpDir, err := os.MkdirTemp("", "tgdl-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outPattern := filepath.Join(tmpDir, "%(title)s.%(ext)s")

	var args []string
	if mode == "audio" {
		args = []string{"-f", "bestaudio", "-x", "--audio-format", "mp3", "-o", outPattern, url}
	} else {
		args = []string{"-f", "bestvideo+bestaudio/best", "-o", outPattern, url}
	}

	// Если cookies.txt существует — добавляем в параметры
	cookiesPath := "/app/cookies.txt"
	if _, err := os.Stat(cookiesPath); err == nil {
		args = append([]string{"--cookies", cookiesPath}, args...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ytTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("время ожидания скачивания истекло (%s)", ytTimeout.String())
	}
	if err != nil {
		return fmt.Errorf("yt-dlp error: %v; output: %s", err, string(out))
	}

	filePath, ferr := findLatestFile(tmpDir)
	if ferr != nil {
		return fmt.Errorf("не найден файл после скачивания: %w", ferr)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat error: %w", err)
	}
	if info.Size() > maxFileSize {
		return fmt.Errorf("файл слишком большой для отправки через Telegram (>2GB). Размер: %d", info.Size())
	}

	// notify uploading
	bot.Send(tgbotapi.NewMessage(chatID, "📤 Загружаю в Telegram..."))

	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(filePath))
	if mode == "audio" {
		doc.Caption = "Аудио: " + filepath.Base(filePath)
	} else {
		doc.Caption = "Видео: " + filepath.Base(filePath)
	}

	if _, err := bot.Send(doc); err != nil {
		return fmt.Errorf("ошибка отправки файла: %w", err)
	}

	bot.Send(tgbotapi.NewMessage(chatID, "✅ Готово — файл отправлен."))
	return nil
}

func findLatestFile(dir string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var latest string
	var latestTime time.Time
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = filepath.Join(dir, f.Name())
		}
	}
	if latest == "" {
		return "", fmt.Errorf("нет файлов в папке")
	}
	return latest, nil
}

package main

import (
	"log"
	"os"
	"os/exec"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Токен бота из переменной окружения
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		url := update.Message.Text
		log.Printf("Got URL: %s", url)

		// Отправляем сообщение "скачиваю..."
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Скачиваю видео...")
		bot.Send(msg)

		// Запускаем скачивание
		err := runYTDLP(url)
		if err != nil {
			log.Println("yt-dlp error:", err)
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при скачивании 😢"))
			continue
		}

		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Видео скачано ✅"))
	}
}

// Запуск yt-dlp с поддержкой cookies
func runYTDLP(url string) error {
	cookies := os.Getenv("YTDLP_COOKIES")
	cookieFile := "/app/cookies.txt"

	if cookies != "" {
		err := os.WriteFile(cookieFile, []byte(cookies), 0644)
		if err != nil {
			return err
		}
	}

	// Аргументы для yt-dlp
	args := []string{url}
	if cookies != "" {
		args = append([]string{"--cookies", cookieFile}, args...)
	}

	// Кладём в папку /app/downloads
	args = append(args, "-o", "/app/downloads/%(title)s.%(ext)s")

	// Запускаем yt-dlp
	cmd := exec.Command("yt-dlp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

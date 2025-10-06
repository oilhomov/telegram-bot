package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("❌ Переменная окружения BOT_TOKEN не установлена")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Ошибка при запуске бота: %v", err)
	}

	log.Printf("🤖 Бот запущен: @%s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		text := update.Message.Text
		chatID := update.Message.Chat.ID

		switch {
		case text == "/start":
			msg := tgbotapi.NewMessage(chatID,
				"👋 Привет! Я бот для скачивания видео с YouTube и Instagram.\n\n" +
					"Просто отправь мне ссылку, и я предложу выбрать — скачать 🎥 видео или 🎵 аудио.")
			bot.Send(msg)

		case text == "/help":
			msg := tgbotapi.NewMessage(chatID,
				"📘 Доступные команды:\n" +
					"/start — начать работу\n" +
					"/help — справка\n\n" +
					"Просто пришли ссылку на видео с YouTube или Instagram Reels.")
			bot.Send(msg)

		case strings.Contains(text, "youtube.com") || strings.Contains(text, "youtu.be") || strings.Contains(text, "instagram.com"):
			// Кнопки выбора
			videoBtn := tgbotapi.NewKeyboardButton("🎥 Видео")
			audioBtn := tgbotapi.NewKeyboardButton("🎵 Аудио")
			keyboard := tgbotapi.NewReplyKeyboard([]tgbotapi.KeyboardButton{videoBtn, audioBtn})
			msg := tgbotapi.NewMessage(chatID, "Что хочешь скачать?")
			msg.ReplyMarkup = keyboard
			bot.Send(msg)

			// Сохраняем ссылку для дальнейшего использования
			go func(link string, chat int64) {
				for upd := range updates {
					if upd.Message == nil {
						continue
					}

					choice := upd.Message.Text
					if choice == "🎥 Видео" || choice == "🎵 Аудио" {
						bot.Send(tgbotapi.NewMessage(chat, "⏳ Загружаю, подожди немного..."))

						format := "best"
						if choice == "🎵 Аудио" {
							format = "bestaudio"
						}

						fileName := "output"
						cmd := exec.Command("yt-dlp", "-f", format, "-o", fileName+".%(ext)s", link)
						output, err := cmd.CombinedOutput()
						if err != nil {
							bot.Send(tgbotapi.NewMessage(chat, fmt.Sprintf("❌ Ошибка yt-dlp: %v\n%s", err, string(output))))
							return
						}

						// Найдем файл по расширению
						var ext string
						if choice == "🎥 Видео" {
							ext = ".mp4"
						} else {
							ext = ".m4a"
						}

						filePath := fileName + ext
						video := tgbotapi.NewDocument(chat, tgbotapi.FilePath(filePath))
						bot.Send(video)

						os.Remove(filePath)
						return
					}
				}
			}(text, chatID)

		default:
			msg := tgbotapi.NewMessage(chatID, "❓ Неизвестная команда. Напиши /help, если нужно напоминание.")
			bot.Send(msg)
		}
	}
}

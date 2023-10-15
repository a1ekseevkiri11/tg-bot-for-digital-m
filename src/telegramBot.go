package src

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type User struct {
	Chat_ID int64
}

type TelegramBot struct {
	API                   *tgbotapi.BotAPI        // API телеграмма
	Updates               tgbotapi.UpdatesChannel // Канал обновлений
	ActiveContactRequests []int64                 // ID чатов, от которых мы ожидаем номер
}

func (telegramBot *TelegramBot) Init() {
	botAPI, err := tgbotapi.NewBotAPI(conf.TELEGRAM_BOT_API_KEY) // Инициализация API
	if err != nil {
		log.Fatal(err)
	}
	telegramBot.API = botAPI
	botUpdate := tgbotapi.NewUpdate(0) // Инициализация канала обновлений
	botUpdate.Timeout = conf.UPDATE_CONFIG_TIMEOUT
	botUpdates, err := telegramBot.API.GetUpdatesChan(botUpdate)
	if err != nil {
		log.Fatal(err)
	}
	telegramBot.Updates = botUpdates
}

package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"health-diary/internal/ingest"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	ingest  *ingest.Service
	allowed map[int64]struct{}
	log     *slog.Logger
}

func NewHandler(ingest *ingest.Service, allowed map[int64]struct{}, log *slog.Logger) *Handler {
	return &Handler{ingest: ingest, allowed: allowed, log: log}
}

func (h *Handler) Handle(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.Message == nil || update.Message.From == nil {
		return nil
	}
	m := update.Message
	if !m.Chat.IsPrivate() {
		return nil
	}
	if _, ok := h.allowed[m.From.ID]; !ok {
		_, err := bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Этот бот пока недоступен для этого аккаунта."))
		return err
	}
	text := strings.TrimSpace(m.Text)
	if text == "" {
		return nil
	}
	if strings.HasPrefix(text, "/") {
		return h.command(bot, m.Chat.ID, text)
	}
	_, err := h.ingest.CaptureTelegramText(ctx, ingest.Capture{UpdateID: int64(update.UpdateID), TelegramUserID: int64(m.From.ID), MessageID: int64(m.MessageID), Username: m.From.UserName, Text: text, SentAt: m.Time()})
	if err != nil {
		return fmt.Errorf("capture update %d: %w", update.UpdateID, err)
	}
	_, err = bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Запись принята. Скоро пришлю распознанные события для подтверждения."))
	return err
}

func (h *Handler) command(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	command := strings.Fields(strings.TrimPrefix(text, "/"))
	if len(command) == 0 {
		return nil
	}
	var reply string
	switch strings.Split(command[0], "@")[0] {
	case "start":
		reply = "Дневник здоровья: отправьте сообщение о самочувствии, боли, лекарствах или активности."
	case "help":
		reply = "Напишите обычным текстом. Например: «В 15:00 заболела голова справа, 6 из 10. Выпил ибупрофен 400»."
	case "privacy":
		reply = "Текст сохраняется зашифрованным. Для извлечения фактов он передаётся Polza.ai без Telegram ID и имени."
	default:
		reply = "Команда пока недоступна. Используйте /help."
	}
	_, err := bot.Send(tgbotapi.NewMessage(chatID, reply))
	return err
}

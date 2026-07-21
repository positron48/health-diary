package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"health-diary/internal/auth"
	"health-diary/internal/ingest"
	"health-diary/internal/journal"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	ingest  *ingest.Service
	auth    *auth.Service
	allowed map[int64]struct{}
	log     *slog.Logger
	db      *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool, ingest *ingest.Service, authService *auth.Service, allowed map[int64]struct{}, log *slog.Logger) *Handler {
	return &Handler{db: db, ingest: ingest, auth: authService, allowed: allowed, log: log}
}

func (h *Handler) Handle(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery != nil {
		return h.callback(ctx, bot, update.CallbackQuery)
	}
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
		return h.command(ctx, bot, m.Chat.ID, int64(m.From.ID), m.From.UserName, text)
	}
	_, err := h.ingest.CaptureTelegramText(ctx, ingest.Capture{UpdateID: int64(update.UpdateID), TelegramUserID: int64(m.From.ID), MessageID: int64(m.MessageID), Username: m.From.UserName, Text: text, SentAt: m.Time()})
	if err != nil {
		return fmt.Errorf("capture update %d: %w", update.UpdateID, err)
	}
	_, err = bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Запись принята. Скоро пришлю распознанные события для подтверждения."))
	return err
}

func (h *Handler) callback(ctx context.Context, api *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) error {
	if callback.From == nil {
		return nil
	}
	if _, ok := h.allowed[callback.From.ID]; !ok {
		return nil
	}
	parts := strings.Split(callback.Data, ":")
	if len(parts) != 3 || parts[0] != "v1" || (parts[2] != "confirm" && parts[2] != "reject") {
		_, err := api.Request(tgbotapi.NewCallback(callback.ID, "Действие недоступно."))
		return err
	}
	err := journal.ApplyTelegramAction(ctx, h.db, int64(callback.From.ID), parts[1], parts[2])
	message := "Готово: события подтверждены."
	if parts[2] == "reject" {
		message = "Черновик отклонён."
	}
	if err != nil {
		message = "Это действие уже выполнено или ссылка устарела."
	}
	_, requestErr := api.Request(tgbotapi.NewCallback(callback.ID, message))
	return requestErr
}

func (h *Handler) command(ctx context.Context, bot *tgbotapi.BotAPI, chatID, telegramUserID int64, username, text string) error {
	command := strings.Fields(strings.TrimPrefix(text, "/"))
	if len(command) == 0 {
		return nil
	}
	var reply string
	switch strings.Split(command[0], "@")[0] {
	case "start":
		if len(command) == 2 && strings.HasPrefix(command[1], "login_") && h.auth != nil {
			code, err := h.auth.BindTelegram(ctx, strings.TrimPrefix(command[1], "login_"), telegramUserID, username)
			if err != nil {
				reply = "Ссылка для входа недействительна или истекла."
			} else {
				reply = "Код для входа: " + code
			}
		} else {
			reply = "Дневник здоровья: отправьте сообщение о самочувствии, боли, лекарствах или активности."
		}
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

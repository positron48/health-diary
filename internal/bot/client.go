package bot

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/net/proxy"
)

// NewAPI optionally routes Telegram API traffic through a dedicated SOCKS5
// proxy. It intentionally does not alter process-wide HTTP proxy settings.
func NewAPI(token, socks5Address string) (*tgbotapi.BotAPI, error) {
	if strings.TrimSpace(socks5Address) == "" {
		api, err := tgbotapi.NewBotAPI(token)
		if err != nil {
			return nil, telegramAPIError("initialize Telegram client")
		}
		return api, nil
	}
	raw := strings.TrimSpace(socks5Address)
	if !strings.Contains(raw, "://") {
		raw = "socks5://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "socks5" || u.Host == "" {
		return nil, fmt.Errorf("invalid Telegram SOCKS5 proxy")
	}
	var auth *proxy.Auth
	if u.User != nil {
		password, _ := u.User.Password()
		auth = &proxy.Auth{User: u.User.Username(), Password: password}
	}
	dialer, err := proxy.SOCKS5("tcp", u.Host, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("create Telegram SOCKS5 proxy")
	}
	transport := &http.Transport{DialContext: func(_ context.Context, network, address string) (net.Conn, error) {
		return dialer.Dial(network, address)
	}}
	api, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, &http.Client{Transport: transport, Timeout: 40 * time.Second})
	if err != nil {
		return nil, telegramAPIError("initialize Telegram client")
	}
	return api, nil
}

// telegramAPIError deliberately discards the original error: the Telegram
// library includes the bot token in request URLs in some transport errors.
func telegramAPIError(operation string) error {
	return fmt.Errorf("%s failed", operation)
}

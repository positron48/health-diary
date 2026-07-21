package bot

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookRejectsMissingOrWrongSecretBeforeParsingUpdate(t *testing.T) {
	handler := WebhookHandler(nil, nil, "expected-secret")
	for _, secret := range []string{"", "wrong"} {
		request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", nil)
		request.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("secret %q: status=%d", secret, response.Code)
		}
	}
}

package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

const (
	// Long polling waits up to 30s for an update. The HTTP client timeout must
	// stay above it so telego can finish the request and return a response.
	longPollingTimeoutSeconds = 30
	telegramHTTPTimeout       = 45 * time.Second

	safeCallerMaxAttempts   = 3
	maxGetUpdatesRetryAfter = 5 * time.Minute
)

var telegramTokenPattern = regexp.MustCompile(`[0-9]{3,}:[A-Za-z0-9_-]{10,}`)

// redactToken removes Telegram bot tokens from strings before they reach logs
// or callers. Errors from net/http commonly embed the full request URL.
func redactToken(value string) string {
	return telegramTokenPattern.ReplaceAllString(value, "[REDACTED]")
}

type redactedError struct {
	err error
}

func (e *redactedError) Error() string { return redactToken(e.err.Error()) }
func (e *redactedError) Unwrap() error { return e.err }

func redactError(err error) error {
	if err == nil {
		return nil
	}
	return &redactedError{err: err}
}

func waitContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// telegoSlogLogger forwards telego's library logs to slog after removing the
// bot token. The default telego logger is discarded in production, which hides
// long-polling failures such as 409 and network errors.
type telegoSlogLogger struct {
	logger *slog.Logger
}

func (l telegoSlogLogger) Debugf(format string, args ...any) {
	l.logger.Debug("telego", "message", redactToken(fmt.Sprintf(format, args...)))
}

func (l telegoSlogLogger) Errorf(format string, args ...any) {
	l.logger.Error("telego", "message", redactToken(fmt.Sprintf(format, args...)))
}

// BotAPI defines the contract for interacting with the Telegram Bot API.
// Allows mock implementations in tests.
type BotAPI interface {
	GetMe(ctx context.Context) (*telego.User, error)
	GetWebhookInfo(ctx context.Context) (*telego.WebhookInfo, error)
	SetWebhook(ctx context.Context, params *telego.SetWebhookParams) error
	DeleteWebhook(ctx context.Context, params *telego.DeleteWebhookParams) error
	SetMyCommands(ctx context.Context, params *telego.SetMyCommandsParams) error
	GetChatMember(ctx context.Context, params *telego.GetChatMemberParams) (telego.ChatMember, error)
	UpdatesViaLongPolling(ctx context.Context, params *telego.GetUpdatesParams, options ...telego.LongPollingOption) (<-chan telego.Update, error)
	SendMessage(ctx context.Context, params *telego.SendMessageParams) (*telego.Message, error)
	SendSticker(ctx context.Context, params *telego.SendStickerParams) (*telego.Message, error)
	EditMessageText(ctx context.Context, params *telego.EditMessageTextParams) (*telego.Message, error)
	EditMessageReplyMarkup(ctx context.Context, params *telego.EditMessageReplyMarkupParams) (*telego.Message, error)
	AnswerInlineQuery(ctx context.Context, params *telego.AnswerInlineQueryParams) error
	AnswerCallbackQuery(ctx context.Context, params *telego.AnswerCallbackQueryParams) error
	SetMessageReaction(ctx context.Context, params *telego.SetMessageReactionParams) error
}

// explicitAnswerInlineQueryParams ensures cache_time:0, is_personal:true and next_offset:""
// are explicitly serialized to JSON, bypassing telego's default omitempty behavior.
type explicitAnswerInlineQueryParams struct {
	InlineQueryID string                           `json:"inline_query_id"`
	Results       []telego.InlineQueryResult       `json:"results"`
	CacheTime     int                              `json:"cache_time"`
	IsPersonal    bool                             `json:"is_personal"`
	NextOffset    string                           `json:"next_offset"`
	Button        *telego.InlineQueryResultsButton `json:"button,omitempty"`
}

// InlineRequestConstructor customizes request encoding for answerInlineQuery.
type InlineRequestConstructor struct {
	defaultConstructor telegoapi.DefaultConstructor
}

func NewInlineRequestConstructor() *InlineRequestConstructor {
	return &InlineRequestConstructor{
		defaultConstructor: telegoapi.DefaultConstructor{},
	}
}

func (c *InlineRequestConstructor) JSONRequest(parameters any) (*telegoapi.RequestData, error) {
	var params *telego.AnswerInlineQueryParams
	switch p := parameters.(type) {
	case *telego.AnswerInlineQueryParams:
		params = p
	case telego.AnswerInlineQueryParams:
		params = &p
	}

	if params != nil {
		explicit := explicitAnswerInlineQueryParams{
			InlineQueryID: params.InlineQueryID,
			Results:       params.Results,
			CacheTime:     params.CacheTime,
			IsPersonal:    params.IsPersonal,
			NextOffset:    params.NextOffset,
			Button:        params.Button,
		}
		data, err := json.Marshal(explicit)
		if err != nil {
			return nil, fmt.Errorf("encode explicit answerInlineQuery json: %w", err)
		}
		return &telegoapi.RequestData{
			ContentType: telegoapi.ContentTypeJSON,
			BodyRaw:     data,
		}, nil
	}

	return c.defaultConstructor.JSONRequest(parameters)
}

func (c *InlineRequestConstructor) MultipartRequest(
	parameters map[string]string,
	filesParameters map[string]telegoapi.NamedReader,
) (*telegoapi.RequestData, error) {
	return c.defaultConstructor.MultipartRequest(parameters, filesParameters)
}

// SafeAPICaller wraps telegoapi.Caller to provide bounded retry for HTTP 429
// and sanitizes logs to prevent token leakage.
type SafeAPICaller struct {
	caller  telegoapi.Caller
	logger  *slog.Logger
	sleeper func(context.Context, time.Duration) error
}

func NewSafeAPICaller(caller telegoapi.Caller, logger *slog.Logger) *SafeAPICaller {
	if logger == nil {
		logger = slog.Default()
	}
	return &SafeAPICaller{
		caller:  caller,
		logger:  logger,
		sleeper: waitContext,
	}
}

// SanitizeMethod extracts only the Telegram API method from the full URL to ensure tokens are never logged.
func SanitizeMethod(rawURL string) string {
	idx := strings.LastIndex(rawURL, "/")
	if idx >= 0 && idx < len(rawURL)-1 {
		return rawURL[idx+1:]
	}
	return "unknown_method"
}

func (s *SafeAPICaller) Call(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
	method := SanitizeMethod(url)

	for attempt := 1; attempt <= safeCallerMaxAttempts; attempt++ {
		resp, err := s.caller.Call(ctx, url, data)
		if err != nil {
			// Network errors, timeouts or context errors are not retried here.
			// telego's long-polling loop applies its own retry policy.
			s.logger.DebugContext(ctx, "telegram transport error",
				"method", method,
				"error", "network request failed",
			)
			return nil, redactError(err)
		}

		if resp == nil {
			return nil, errors.New("nil response from caller")
		}

		if resp.Ok || resp.Error == nil || resp.ErrorCode != http.StatusTooManyRequests {
			return resp, nil
		}

		retryAfter := 0
		if resp.Parameters != nil {
			retryAfter = resp.Parameters.RetryAfter
		}
		s.logger.WarnContext(ctx, "telegram rate limited (429)",
			"method", method,
			"retry_after", retryAfter,
			"attempt", attempt,
		)

		if retryAfter <= 0 {
			return resp, nil
		}
		delay := time.Duration(retryAfter) * time.Second

		// getUpdates is the only method where waiting the full retry_after is
		// correct: it runs in the long-polling loop and is not holding a chat
		// worker. Other methods keep the short, single retry to avoid blocking
		// user-visible work for tens of seconds.
		if method == "getUpdates" {
			if attempt == safeCallerMaxAttempts {
				return resp, nil
			}
			if delay > maxGetUpdatesRetryAfter {
				delay = maxGetUpdatesRetryAfter
			}
			if sleepErr := s.sleeper(ctx, delay); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		if attempt > 1 || delay > 5*time.Second {
			return resp, nil
		}
		if sleepErr := s.sleeper(ctx, delay); sleepErr != nil {
			return nil, sleepErr
		}
	}

	return nil, errors.New("telegram retry loop exhausted")
}

// NewBot creates a production telego.Bot configured with our safe caller, custom constructor,
// and disabled raw library logging.
func NewBot(token string, httpClient *http.Client, logger *slog.Logger) (*telego.Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: telegramHTTPTimeout}
	}

	httpCaller := telegoapi.HTTPCaller{Client: httpClient}
	safeCaller := NewSafeAPICaller(httpCaller, logger)
	constructor := NewInlineRequestConstructor()

	return telego.NewBot(token,
		telego.WithAPICaller(safeCaller),
		telego.WithRequestConstructor(constructor),
		telego.WithLogger(telegoSlogLogger{logger: logger}),
	)
}

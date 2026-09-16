package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

// BotAPI defines the contract for interacting with the Telegram Bot API.
// Allows mock implementations in tests.
type BotAPI interface {
	GetMe(ctx context.Context) (*telego.User, error)
	GetWebhookInfo(ctx context.Context) (*telego.WebhookInfo, error)
	SetWebhook(ctx context.Context, params *telego.SetWebhookParams) error
	DeleteWebhook(ctx context.Context, params *telego.DeleteWebhookParams) error
	SetMyCommands(ctx context.Context, params *telego.SetMyCommandsParams) error
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
		caller: caller,
		logger: logger,
		sleeper: func(ctx context.Context, d time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
				return nil
			}
		},
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
	resp, err := s.caller.Call(ctx, url, data)
	if err != nil {
		// Network errors, timeouts or context errors are not retried.
		s.logger.DebugContext(ctx, "telegram transport error",
			"method", SanitizeMethod(url),
			"error", err.Error(),
		)
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("nil response from caller")
	}

	// Telegram API error check
	if !resp.Ok && resp.ErrorCode == http.StatusTooManyRequests {
		retryAfter := 0
		if resp.Parameters != nil {
			retryAfter = resp.Parameters.RetryAfter
		}

		s.logger.WarnContext(ctx, "telegram rate limited (429)",
			"method", SanitizeMethod(url),
			"retry_after", retryAfter,
		)

		// Retry at most once if retry_after is between 1 and 5 seconds
		if retryAfter > 0 && retryAfter <= 5 {
			if sleepErr := s.sleeper(ctx, time.Duration(retryAfter)*time.Second); sleepErr != nil {
				return nil, sleepErr
			}
			// One retry attempt
			retryResp, retryErr := s.caller.Call(ctx, url, data)
			if retryErr != nil {
				return nil, retryErr
			}
			return retryResp, nil
		}
	}

	return resp, nil
}

// NewBot creates a production telego.Bot configured with our safe caller, custom constructor,
// and disabled raw library logging.
func NewBot(token string, httpClient *http.Client, logger *slog.Logger) (*telego.Bot, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	httpCaller := telegoapi.HTTPCaller{Client: httpClient}
	safeCaller := NewSafeAPICaller(httpCaller, logger)
	constructor := NewInlineRequestConstructor()

	return telego.NewBot(token,
		telego.WithAPICaller(safeCaller),
		telego.WithRequestConstructor(constructor),
		telego.WithDiscardLogger(),
	)
}

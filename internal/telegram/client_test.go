package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

func TestInlineRequestConstructor_ExplicitFields(t *testing.T) {
	constructor := NewInlineRequestConstructor()

	params := &telego.AnswerInlineQueryParams{
		InlineQueryID: "query_123",
		Results:       []telego.InlineQueryResult{},
		CacheTime:     0,
		IsPersonal:    true,
		NextOffset:    "",
	}

	reqData, err := constructor.JSONRequest(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jsonStr := string(reqData.BodyRaw)

	// Verify cache_time is present and 0
	if !strings.Contains(jsonStr, `"cache_time":0`) {
		t.Errorf("expected json to contain '\"cache_time\":0', got: %s", jsonStr)
	}

	// Verify is_personal is present and true
	if !strings.Contains(jsonStr, `"is_personal":true`) {
		t.Errorf("expected json to contain '\"is_personal\":true', got: %s", jsonStr)
	}

	// Verify next_offset is present and empty string
	if !strings.Contains(jsonStr, `"next_offset":""`) {
		t.Errorf("expected json to contain '\"next_offset\":\"\"', got: %s", jsonStr)
	}

	// Verify unmarshaling produces expected explicit values
	var decoded map[string]any
	if err := json.Unmarshal(reqData.BodyRaw, &decoded); err != nil {
		t.Fatalf("failed to unmarshal generated json: %v", err)
	}

	if decoded["inline_query_id"] != "query_123" {
		t.Errorf("expected inline_query_id query_123, got %v", decoded["inline_query_id"])
	}
	if ct, ok := decoded["cache_time"].(float64); !ok || int(ct) != 0 {
		t.Errorf("expected cache_time 0, got %v", decoded["cache_time"])
	}
	if ip, ok := decoded["is_personal"].(bool); !ok || !ip {
		t.Errorf("expected is_personal true, got %v", decoded["is_personal"])
	}
	if no, ok := decoded["next_offset"].(string); !ok || no != "" {
		t.Errorf("expected next_offset empty string, got %v", decoded["next_offset"])
	}
}

func TestSanitizeMethod(t *testing.T) {
	cases := []struct {
		url      string
		expected string
	}{
		{"https://api.telegram.org/bot123456:SECRET_TOKEN/sendMessage", "sendMessage"},
		{"https://api.telegram.org/bot123456:SECRET_TOKEN/answerInlineQuery", "answerInlineQuery"},
		{"https://api.telegram.org/bot123456:SECRET_TOKEN/getMe", "getMe"},
		{"plain_method", "unknown_method"},
	}

	for _, tc := range cases {
		got := SanitizeMethod(tc.url)
		if got != tc.expected {
			t.Errorf("for url %q expected %q, got %q", tc.url, tc.expected, got)
		}
		if strings.Contains(got, "SECRET_TOKEN") {
			t.Errorf("sanitized method leaked secret token: %q", got)
		}
	}
}

type mockCaller struct {
	calls []string
	fn    func(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error)
}

func (m *mockCaller) Call(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
	m.calls = append(m.calls, url)
	return m.fn(ctx, url, data)
}

func TestSafeAPICaller_NoRetryOnNetworkError(t *testing.T) {
	networkErr := errors.New("connection reset by peer")
	caller := &mockCaller{
		fn: func(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
			return nil, networkErr
		},
	}

	safe := NewSafeAPICaller(caller, nil)
	_, err := safe.Call(context.Background(), "https://api.telegram.org/bot123/sendMessage", &telegoapi.RequestData{})
	if !errors.Is(err, networkErr) {
		t.Fatalf("expected network error, got %v", err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(caller.calls))
	}
}

func TestSafeAPICaller_Retry429UnderThreshold(t *testing.T) {
	attempts := 0
	caller := &mockCaller{
		fn: func(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
			attempts++
			if attempts == 1 {
				return &telegoapi.Response{
					Ok: false,
					Error: &telegoapi.Error{
						ErrorCode:   http.StatusTooManyRequests,
						Description: "Too Many Requests",
						Parameters:  &telegoapi.ResponseParameters{RetryAfter: 1},
					},
				}, nil
			}
			return &telegoapi.Response{Ok: true}, nil
		},
	}

	safe := NewSafeAPICaller(caller, nil)
	slept := false
	safe.sleeper = func(ctx context.Context, d time.Duration) error {
		slept = true
		if d != 1*time.Second {
			t.Errorf("expected sleep duration 1s, got %v", d)
		}
		return nil
	}

	resp, err := safe.Call(context.Background(), "https://api.telegram.org/bot123/sendMessage", &telegoapi.RequestData{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Ok {
		t.Errorf("expected Ok: true on retry")
	}
	if !slept {
		t.Errorf("expected sleeper to be called")
	}
	if attempts != 2 {
		t.Errorf("expected exactly 2 attempts, got %d", attempts)
	}
}

func TestSafeAPICaller_NoRetry429OverThreshold(t *testing.T) {
	caller := &mockCaller{
		fn: func(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
			return &telegoapi.Response{
				Ok: false,
				Error: &telegoapi.Error{
					ErrorCode:   http.StatusTooManyRequests,
					Description: "Too Many Requests",
					Parameters:  &telegoapi.ResponseParameters{RetryAfter: 10},
				},
			}, nil
		},
	}

	safe := NewSafeAPICaller(caller, nil)
	slept := false
	safe.sleeper = func(ctx context.Context, d time.Duration) error {
		slept = true
		return nil
	}

	resp, err := safe.Call(context.Background(), "https://api.telegram.org/bot123/sendMessage", &telegoapi.RequestData{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Ok {
		t.Errorf("expected Ok: false")
	}
	if slept {
		t.Errorf("sleeper should not be called for retry_after > 5s")
	}
	if len(caller.calls) != 1 {
		t.Errorf("expected exactly 1 attempt, got %d", len(caller.calls))
	}
}

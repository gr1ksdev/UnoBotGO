package telegram

import (
	"context"
	"sync"

	"github.com/mymmrac/telego"
)

type mockBotAPI struct {
	mu sync.Mutex

	MeUser             *telego.User
	MeErr              error
	WebhookInfo        *telego.WebhookInfo
	WebhookErr         error
	SetWebhookErr      error
	DeleteWebhookErr   error
	SetWebhookCalls    []telego.SetWebhookParams
	DeleteWebhookCalls []telego.DeleteWebhookParams
	CommandsErr        error
	ChatMemberErr      error
	ChatMembers        map[int64]telego.ChatMember

	SentMessages         []telego.SendMessageParams
	SentStickers         []telego.SendStickerParams
	SentReactions        []telego.SetMessageReactionParams
	EditedMessages       []telego.EditMessageTextParams
	EditedMarkups        []telego.EditMessageReplyMarkupParams
	AnsweredInlines      []telego.AnswerInlineQueryParams
	AnsweredCallbacks    []telego.AnswerCallbackQueryParams
	RegisteredCommands   []telego.BotCommand
	CommandRegistrations []telego.SetMyCommandsParams
	SentMessageSignal    chan struct{}

	UpdatesChan chan telego.Update
}

func newMockBotAPI() *mockBotAPI {
	return &mockBotAPI{
		MeUser: &telego.User{
			ID:                    12345,
			FirstName:             "UnoBot",
			Username:              "unobot",
			SupportsInlineQueries: true,
		},
		WebhookInfo: &telego.WebhookInfo{
			URL: "",
		},
		UpdatesChan:       make(chan telego.Update, 100),
		ChatMembers:       make(map[int64]telego.ChatMember),
		SentMessageSignal: make(chan struct{}, 100),
	}
}

func (m *mockBotAPI) GetMe(ctx context.Context) (*telego.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MeUser, m.MeErr
}

func (m *mockBotAPI) GetWebhookInfo(ctx context.Context) (*telego.WebhookInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.WebhookInfo, m.WebhookErr
}

func (m *mockBotAPI) SetWebhook(ctx context.Context, params *telego.SetWebhookParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.SetWebhookCalls = append(m.SetWebhookCalls, *params)
		if m.WebhookInfo == nil {
			m.WebhookInfo = &telego.WebhookInfo{}
		}
		m.WebhookInfo.URL = params.URL
	}
	return m.SetWebhookErr
}

func (m *mockBotAPI) DeleteWebhook(ctx context.Context, params *telego.DeleteWebhookParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.DeleteWebhookCalls = append(m.DeleteWebhookCalls, *params)
	}
	if m.WebhookInfo == nil {
		m.WebhookInfo = &telego.WebhookInfo{}
	}
	m.WebhookInfo.URL = ""
	return m.DeleteWebhookErr
}

func (m *mockBotAPI) SetMyCommands(ctx context.Context, params *telego.SetMyCommandsParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.RegisteredCommands = params.Commands
		m.CommandRegistrations = append(m.CommandRegistrations, *params)
	}
	return m.CommandsErr
}

func (m *mockBotAPI) GetChatMember(ctx context.Context, params *telego.GetChatMemberParams) (telego.ChatMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ChatMemberErr != nil {
		return nil, m.ChatMemberErr
	}
	if params == nil {
		return nil, nil
	}
	return m.ChatMembers[params.UserID], nil
}

func (m *mockBotAPI) UpdatesViaLongPolling(
	ctx context.Context,
	params *telego.GetUpdatesParams,
	options ...telego.LongPollingOption,
) (<-chan telego.Update, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.UpdatesChan, nil
}

func (m *mockBotAPI) SendMessage(ctx context.Context, params *telego.SendMessageParams) (*telego.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.SentMessages = append(m.SentMessages, *params)
	}
	select {
	case m.SentMessageSignal <- struct{}{}:
	default:
	}
	return &telego.Message{MessageID: len(m.SentMessages)}, nil
}

func (m *mockBotAPI) SendSticker(ctx context.Context, params *telego.SendStickerParams) (*telego.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.SentStickers = append(m.SentStickers, *params)
	}
	return &telego.Message{MessageID: len(m.SentStickers)}, nil
}

func (m *mockBotAPI) EditMessageText(ctx context.Context, params *telego.EditMessageTextParams) (*telego.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.EditedMessages = append(m.EditedMessages, *params)
	}
	return &telego.Message{MessageID: 1}, nil
}

func (m *mockBotAPI) EditMessageReplyMarkup(ctx context.Context, params *telego.EditMessageReplyMarkupParams) (*telego.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.EditedMarkups = append(m.EditedMarkups, *params)
	}
	return &telego.Message{MessageID: 1}, nil
}

func (m *mockBotAPI) AnswerInlineQuery(ctx context.Context, params *telego.AnswerInlineQueryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.AnsweredInlines = append(m.AnsweredInlines, *params)
	}
	return nil
}

func (m *mockBotAPI) AnswerCallbackQuery(ctx context.Context, params *telego.AnswerCallbackQueryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.AnsweredCallbacks = append(m.AnsweredCallbacks, *params)
	}
	return nil
}

func (m *mockBotAPI) SetMessageReaction(ctx context.Context, params *telego.SetMessageReactionParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if params != nil {
		m.SentReactions = append(m.SentReactions, *params)
	}
	return nil
}

func (m *mockBotAPI) LastSentMessage() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.SentMessages) == 0 {
		return ""
	}
	return m.SentMessages[len(m.SentMessages)-1].Text
}

func (m *mockBotAPI) GetRegisteredCommands() []telego.BotCommand {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]telego.BotCommand, len(m.RegisteredCommands))
	copy(res, m.RegisteredCommands)
	return res
}

func (m *mockBotAPI) GetCommandRegistrations() []telego.SetMyCommandsParams {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]telego.SetMyCommandsParams, len(m.CommandRegistrations))
	copy(res, m.CommandRegistrations)
	return res
}

func (m *mockBotAPI) GetSentReactions() []telego.SetMessageReactionParams {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]telego.SetMessageReactionParams, len(m.SentReactions))
	copy(res, m.SentReactions)
	return res
}

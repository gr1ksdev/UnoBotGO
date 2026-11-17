package telegram

import (
	"context"
	"strings"
	"sync"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

// commandID is a transport-independent name for an action. Each group accepts
// only one spelling, chosen by its configuration; there are no cross-language
// aliases.
type commandID string

const (
	cmdNew     commandID = "new"
	cmdJoin    commandID = "join"
	cmdStart   commandID = "start"
	cmdLeave   commandID = "leave"
	cmdClose   commandID = "close"
	cmdOpen    commandID = "open"
	cmdKill    commandID = "kill"
	cmdSkip    commandID = "skip"
	cmdKick    commandID = "kick"
	cmdState   commandID = "state"
	cmdRanking commandID = "ranking"
	cmdStats   commandID = "stats"
	cmdConfig  commandID = "config"
	cmdReset   commandID = "reset"
	cmdHelp    commandID = "help"
)

var portugueseCommands = map[string]commandID{
	"novo":       cmdNew,
	"entrar":     cmdJoin,
	"iniciar":    cmdStart,
	"sair":       cmdLeave,
	"trancar":    cmdClose,
	"destrancar": cmdOpen,
	"cancelar":   cmdKill,
	"pular":      cmdSkip,
	"expulsar":   cmdKick,
	"estado":     cmdState,
	"ranking":    cmdRanking,
	"stats":      cmdStats,
	"config":     cmdConfig,
	"reset":      cmdReset,
	"ajuda":      cmdHelp,
}

var englishCommands = map[string]commandID{
	"new":      cmdNew,
	"join":     cmdJoin,
	"start":    cmdStart,
	"leave":    cmdLeave,
	"close":    cmdClose,
	"open":     cmdOpen,
	"kill":     cmdKill,
	"skip":     cmdSkip,
	"kick":     cmdKick,
	"state":    cmdState,
	"ranking":  cmdRanking,
	"stats":    cmdStats,
	"settings": cmdConfig,
	"reset":    cmdReset,
	"help":     cmdHelp,
}

// Spelling of each command in each language. Shared words appear in both.
var commandSpelling = map[commandID]map[groups.CommandLanguage]string{
	cmdNew:     {groups.CommandsPT: "/novo", groups.CommandsEN: "/new"},
	cmdJoin:    {groups.CommandsPT: "/entrar", groups.CommandsEN: "/join"},
	cmdStart:   {groups.CommandsPT: "/iniciar", groups.CommandsEN: "/start"},
	cmdLeave:   {groups.CommandsPT: "/sair", groups.CommandsEN: "/leave"},
	cmdClose:   {groups.CommandsPT: "/trancar", groups.CommandsEN: "/close"},
	cmdOpen:    {groups.CommandsPT: "/destrancar", groups.CommandsEN: "/open"},
	cmdKill:    {groups.CommandsPT: "/cancelar", groups.CommandsEN: "/kill"},
	cmdSkip:    {groups.CommandsPT: "/pular", groups.CommandsEN: "/skip"},
	cmdKick:    {groups.CommandsPT: "/expulsar", groups.CommandsEN: "/kick"},
	cmdState:   {groups.CommandsPT: "/estado", groups.CommandsEN: "/state"},
	cmdRanking: {groups.CommandsPT: "/ranking", groups.CommandsEN: "/ranking"},
	cmdStats:   {groups.CommandsPT: "/stats", groups.CommandsEN: "/stats"},
	cmdConfig:  {groups.CommandsPT: "/config", groups.CommandsEN: "/settings"},
	cmdReset:   {groups.CommandsPT: "/reset", groups.CommandsEN: "/reset"},
	cmdHelp:    {groups.CommandsPT: "/ajuda", groups.CommandsEN: "/help"},
}

var commandDescriptions = map[groups.CommandLanguage]map[commandID]string{
	groups.CommandsPT: {
		cmdNew: "Criar uma nova partida de UNO", cmdJoin: "Entrar na partida de UNO",
		cmdStart: "Iniciar a partida", cmdLeave: "Sair da partida em andamento",
		cmdClose: "Impede novos jogadores de entrar", cmdOpen: "Permite novas entradas",
		cmdKill: "Cancelar a partida", cmdSkip: "Pular o turno do jogador atual",
		cmdKick: "Expulsar um jogador da partida", cmdState: "Ver o estado atual da partida",
		cmdRanking: "Ver o ranking acumulado do grupo", cmdStats: "Ver suas estatísticas no grupo",
		cmdConfig: "Configurar o ranking e o idioma dos comandos", cmdReset: "Recuperar e limpar o grupo",
		cmdHelp: "Ver comandos e instruções",
	},
	groups.CommandsEN: {
		cmdNew: "Start a new game", cmdJoin: "Join the current game",
		cmdStart: "Start the game", cmdLeave: "Leave the game you're in",
		cmdClose: "Close the game lobby", cmdOpen: "Open the game lobby",
		cmdKill: "Terminate the game", cmdSkip: "Skip the current player",
		cmdKick: "Kick a player out of the game", cmdState: "Show the current game state",
		cmdRanking: "Show the group ranking", cmdStats: "Show your statistics",
		cmdConfig: "Configure the ranking and command language", cmdReset: "Reset and clean the group",
		cmdHelp: "Show commands and instructions",
	},
}

var groupMenuOrder = []commandID{
	cmdNew, cmdJoin, cmdStart, cmdLeave, cmdClose, cmdOpen,
	cmdKill, cmdSkip, cmdKick, cmdState, cmdRanking, cmdStats,
	cmdConfig, cmdReset, cmdHelp,
}

// resolveCommand accepts only the spelling of the configured language.
func resolveCommand(raw string, lang groups.CommandLanguage) (commandID, bool) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if lang == groups.CommandsEN {
		id, ok := englishCommands[name]
		return id, ok
	}
	id, ok := portugueseCommands[name]
	return id, ok
}

func commandName(id commandID, lang groups.CommandLanguage) string {
	if names, ok := commandSpelling[id]; ok {
		if name, ok := names[lang]; ok {
			return name
		}
	}
	return string(id)
}

func commandList(lang groups.CommandLanguage) []telego.BotCommand {
	commands := make([]telego.BotCommand, 0, len(groupMenuOrder))
	for _, id := range groupMenuOrder {
		name := strings.TrimPrefix(commandName(id, lang), "/")
		commands = append(commands, telego.BotCommand{Command: name, Description: commandDescriptions[lang][id]})
	}
	return commands
}

func groupCommandNames(lang groups.CommandLanguage) []string {
	names := make([]string, 0, len(groupMenuOrder))
	for _, id := range groupMenuOrder {
		names = append(names, strings.TrimPrefix(commandName(id, lang), "/"))
	}
	return names
}

func privateCommandList(lang groups.CommandLanguage) []telego.BotCommand {
	return []telego.BotCommand{
		{Command: "start", Description: "Conhecer o UnoBotGO"},
		{Command: "help", Description: "Ver comandos e instruções"},
		{Command: "ranking", Description: "Ver seus rankings mensais"},
		{Command: "stats", Description: "Ver suas estatísticas"},
	}
}

// languageCache avoids a database lookup on every message. It is invalidated
// when an administrator changes the setting through /config.
type languageCache struct {
	mu     sync.RWMutex
	byChat map[game.ChatID]groups.CommandLanguage
	repo   groups.Repository
}

func newLanguageCache() *languageCache {
	return &languageCache{byChat: make(map[game.ChatID]groups.CommandLanguage)}
}

// resolve loads the group setting on first use and caches it. Private chats
// and any lookup failure fall back to Portuguese.
func (c *languageCache) resolve(ctx context.Context, chatID game.ChatID) groups.CommandLanguage {
	if chatID == 0 || c == nil || c.repo == nil {
		return groups.CommandsPT
	}
	c.mu.RLock()
	lang, ok := c.byChat[chatID]
	c.mu.RUnlock()
	if ok {
		return lang
	}
	lang, err := c.repo.ReadCommandLanguage(ctx, int64(chatID))
	if err != nil || !lang.Valid() {
		return groups.CommandsPT
	}
	c.mu.Lock()
	c.byChat[chatID] = lang
	c.mu.Unlock()
	return lang
}

func (c *languageCache) set(chatID game.ChatID, lang groups.CommandLanguage) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byChat[chatID] = lang
}

func (h *CommandHandler) commandLanguage(ctx context.Context, chatID game.ChatID) groups.CommandLanguage {
	return h.languages.resolve(ctx, chatID)
}

func (h *CallbackHandler) commandLanguage(ctx context.Context, chatID game.ChatID) groups.CommandLanguage {
	return h.languages.resolve(ctx, chatID)
}

// applyCommandMenu installs the per-chat override, or removes it when the
// group uses the default language so the global list takes over again.
func (b *Bot) applyCommandMenu(ctx context.Context, chatID game.ChatID, lang groups.CommandLanguage) error {
	scope := telegoutil.ScopeChat(telego.ChatID{ID: int64(chatID)})
	params := &telego.SetMyCommandsParams{
		Commands: commandList(lang),
		Scope:    scope,
	}
	if lang == groups.CommandsPT {
		return b.api.DeleteMyCommands(ctx, &telego.DeleteMyCommandsParams{Scope: scope})
	}
	return b.api.SetMyCommands(ctx, params)
}

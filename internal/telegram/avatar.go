package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mymmrac/telego"
)

// AvatarSource is separate from the gameplay BotAPI. Errors never contain URLs/token.
type AvatarSource struct{ Bot *telego.Bot }

var errPhoto = errors.New("telegram photo unavailable")

func (s AvatarSource) Photo(ctx context.Context, kind string, id int64) ([]byte, string, error) {
	var fileID string
	if kind == "group" {
		chat, err := s.Bot.GetChat(ctx, &telego.GetChatParams{ChatID: telego.ChatID{ID: id}})
		if err != nil {
			return nil, "", errPhoto
		}
		if chat.Photo != nil {
			fileID = chat.Photo.SmallFileID
		}
	} else {
		photos, err := s.Bot.GetUserProfilePhotos(ctx, &telego.GetUserProfilePhotosParams{UserID: id, Limit: 1})
		if err != nil {
			return nil, "", errPhoto
		}
		if len(photos.Photos) > 0 && len(photos.Photos[0]) > 0 {
			fileID = photos.Photos[0][0].FileID
		}
	}
	if fileID == "" {
		return nil, "", nil
	}
	file, err := s.Bot.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, "", errPhoto
	}
	raw := s.Bot.FileDownloadURL(file.FilePath)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "api.telegram.org" {
		return nil, "", errPhoto
	}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, "", errPhoto
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", errPhoto
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", errPhoto
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		return nil, "", errPhoto
	}
	mime := http.DetectContentType(data)
	if !strings.HasPrefix(mime, "image/") || (mime != "image/jpeg" && mime != "image/png" && mime != "image/webp") {
		return nil, "", errPhoto
	}
	return data, mime, nil
}

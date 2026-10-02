// Package rankingimport stages legacy ranking text without applying points.
package rankingimport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrInvalid = errors.New("rankingimport: invalid import")

type Status string

const (
	Unresolved Status = "unresolved"
	Linked     Status = "linked"
	Ambiguous  Status = "ambiguous"
	Invalid    Status = "invalid"
)

type Entry struct {
	ID           string
	Line         int
	ImportedName string
	Score        ranking.Units
	LinkedUserID *int64
	Status       Status
	Error        string
}
type Import struct {
	ID                string
	ChatID, CreatedBy int64
	CreatedAt         time.Time
	Hash, RawText     string
	Entries           []Entry
}
type Repository interface {
	StageRankingImport(context.Context, Import) (Import, error)
}

var suffix = regexp.MustCompile(`^(.*) - ([0-9]+)[\t ]*$`)

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
func New(chatID, createdBy int64, raw string) (Import, error) {
	if chatID == 0 || createdBy <= 0 || strings.TrimSpace(raw) == "" {
		return Import{}, ErrInvalid
	}
	id, err := newID()
	if err != nil {
		return Import{}, err
	}
	hash := sha256.Sum256([]byte(raw))
	result := Import{ID: id, ChatID: chatID, CreatedBy: createdBy, CreatedAt: time.Now().UTC(), Hash: hex.EncodeToString(hash[:]), RawText: raw}
	for i, line := range strings.Split(raw, "\n") {
		entryID, err := newID()
		if err != nil {
			return Import{}, err
		}
		entry := Entry{ID: entryID, Line: i + 1, ImportedName: strings.TrimSuffix(line, "\r"), Status: Invalid, Error: "expected final ' - <integer>' suffix"}
		match := suffix.FindStringSubmatch(entry.ImportedName)
		if match != nil && strings.TrimSpace(match[1]) != "" {
			count, err := strconv.ParseUint(match[2], 10, 64)
			if err == nil && count <= math.MaxInt64/100 {
				entry.ImportedName = match[1]
				entry.Score = ranking.Units(count * 100)
				entry.Status = Unresolved
				entry.Error = ""
			} else {
				entry.Error = "score exceeds supported integer range"
			}
		}
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

// Reconcile accepts only literal display-name matches or explicit @username
// matches. It refuses multiple candidates and multiple entries for one user.
// Unicode is preserved, with no lossy normalization or fuzzy name identity.
func Reconcile(entries []Entry, users []groups.KnownUser) []Entry {
	result := make([]Entry, len(entries))
	copy(result, entries)
	candidates := make([][]int64, len(entries))
	claims := map[int64]int{}
	for i, e := range entries {
		if e.Status == Invalid {
			continue
		}
		seen := map[int64]bool{}
		for _, u := range users {
			matches := e.ImportedName == u.DisplayName
			if strings.HasPrefix(e.ImportedName, "@") && u.Username != "" {
				matches = matches || strings.EqualFold(strings.TrimPrefix(e.ImportedName, "@"), u.Username)
			}
			if matches && !seen[u.UserID] {
				seen[u.UserID] = true
				candidates[i] = append(candidates[i], u.UserID)
			}
		}
		for _, id := range candidates[i] {
			claims[id]++
		}
	}
	for i := range result {
		e := &result[i]
		if e.Status == Invalid {
			continue
		}
		e.LinkedUserID = nil
		switch {
		case len(candidates[i]) == 0:
			e.Status = Unresolved
		case len(candidates[i]) == 1 && claims[candidates[i][0]] == 1:
			id := candidates[i][0]
			e.LinkedUserID = &id
			e.Status = Linked
		default:
			e.Status = Ambiguous
		}
	}
	return result
}

package httpapi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/malbs/UnoGoBot/internal/ranking"
)

type reference struct {
	Kind    string           `json:"k"`
	ID      int64            `json:"i,omitempty"`
	System  string           `json:"s,omitempty"`
	Month   string           `json:"m,omitempty"`
	Expires int64            `json:"e"`
	After   *ranking.PageKey `json:"a,omitempty"`
}

type References struct {
	cipher   cipher.AEAD
	keyBytes []byte
}

func NewReferences(secret []byte) (*References, error) {
	if len(secret) != 32 {
		return nil, errors.New("MINIAPP_SECRET must decode to 32 bytes")
	}
	derive := func(purpose string) []byte {
		h := hmac.New(sha256.New, secret)
		h.Write([]byte(purpose))
		return h.Sum(nil)
	}
	block, err := aes.NewCipher(derive("unobotgo/reference/v1"))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &References{aead, derive("unobotgo/key/v1")}, nil
}

func (r *References) seal(v reference) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, r.cipher.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(r.cipher.Seal(nonce, nonce, b, []byte("v1"))), nil
}
func (r *References) open(raw string, now time.Time) (reference, error) {
	var v reference
	if len(raw) > 4096 {
		return v, ranking.ErrInvalid
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	n := r.cipher.NonceSize()
	if err != nil || len(b) < n {
		return v, ranking.ErrInvalid
	}
	plain, err := r.cipher.Open(nil, b[:n], b[n:], []byte("v1"))
	if err != nil {
		return v, ranking.ErrInvalid
	}
	if json.Unmarshal(plain, &v) != nil || v.Expires <= now.Unix() {
		return v, ranking.ErrInvalid
	}
	return v, nil
}
func (r *References) key(kind string, id int64) string {
	h := hmac.New(sha256.New, r.keyBytes)
	h.Write([]byte(kind + ":" + strconv.FormatInt(id, 10)))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil)[:18])
}

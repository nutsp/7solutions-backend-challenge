// Package cryptography provides versioned authenticated encryption and keyed lookup indexes.
package cryptography

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
)

var ErrConfiguration = errors.New("invalid cryptography configuration")

var ErrDecrypt = errors.New("sensitive data authentication failed")

type Config struct {
	ActiveKeyID string
	Keys        map[string][]byte `json:"-"`
}

type Cipher struct {
	active string
	keys   map[string]cipher.AEAD
}

type envelope struct {
	Version    int    `json:"version"`
	KeyID      string `json:"key_id"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

type LookupConfig struct {
	Key []byte `json:"-"`
}

type LookupHasher struct {
	key []byte
}

func New(cfg Config) (*Cipher, error) {
	if cfg.ActiveKeyID == "" || len(cfg.Keys[cfg.ActiveKeyID]) != 32 {
		return nil, ErrConfiguration
	}
	c := &Cipher{
		active: cfg.ActiveKeyID,
		keys:   make(map[string]cipher.AEAD, len(cfg.Keys)),
	}
	for id, key := range cfg.Keys {
		if id == "" || len(id) > 64 || len(key) != 32 {
			return nil, ErrConfiguration
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, ErrConfiguration
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, ErrConfiguration
		}
		c.keys[id] = aead
	}
	return c, nil
}

func ParseKeys(raw string) (map[string][]byte, error) {
	var encoded map[string]string
	if json.Unmarshal([]byte(raw), &encoded) != nil || len(encoded) == 0 {
		return nil, ErrConfiguration
	}
	keys := make(map[string][]byte, len(encoded))
	for id, value := range encoded {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 || id == "" || len(id) > 64 {
			return nil, ErrConfiguration
		}
		keys[id] = key
	}
	return keys, nil
}

func (c *Cipher) Encrypt(plain, aad []byte) ([]byte, error) {
	aead := c.keys[c.active]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return json.Marshal(envelope{
		Version:    1,
		KeyID:      c.active,
		Nonce:      nonce,
		Ciphertext: aead.Seal(nil, nonce, plain, aad),
	})
}

func (c *Cipher) Decrypt(raw, aad []byte) ([]byte, error) {
	var value envelope
	if len(raw) > 131072 || json.Unmarshal(raw, &value) != nil || value.Version != 1 {
		return nil, ErrDecrypt
	}
	aead, ok := c.keys[value.KeyID]
	if !ok || len(value.Nonce) != aead.NonceSize() {
		return nil, ErrDecrypt
	}
	plain, err := aead.Open(nil, value.Nonce, value.Ciphertext, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plain, nil
}

func NewLookup(cfg LookupConfig) (*LookupHasher, error) {
	if len(cfg.Key) != 32 {
		return nil, ErrConfiguration
	}
	return &LookupHasher{
		key: append([]byte(nil), cfg.Key...),
	}, nil
}

func (h *LookupHasher) Lookup(value []byte) string {
	mac := hmac.New(sha256.New, h.key)
	_, _ = mac.Write(value)
	return hex.EncodeToString(mac.Sum(nil))
}

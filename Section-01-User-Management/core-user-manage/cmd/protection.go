package main

import (
	"encoding/base64"

	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/appconfig"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/cryptography"
)

func protection(cfg appconfig.Config) (*cryptography.Cipher, *cryptography.Cipher, *cryptography.LookupHasher, error) {
	users, err := cryptography.ParseKeys(cfg.UserDataKeys)
	if err != nil {
		return nil, nil, nil, err
	}
	audits, err := cryptography.ParseKeys(cfg.UserAuditKeys)
	if err != nil {
		return nil, nil, nil, err
	}
	userCipher, err := cryptography.New(cryptography.Config{
		ActiveKeyID: cfg.Cryptography.UserKeyID,
		Keys:        users,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	auditCipher, err := cryptography.New(cryptography.Config{
		ActiveKeyID: cfg.Cryptography.AuditKeyID,
		Keys:        audits,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(cfg.EmailLookupKey)
	if err != nil {
		return nil, nil, nil, cryptography.ErrConfiguration
	}
	lookup, err := cryptography.NewLookup(cryptography.LookupConfig{
		Key: raw,
	})
	return userCipher, auditCipher, lookup, err
}

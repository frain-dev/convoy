package hookcamp

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp/util"
)

var (
	ErrTokenExpired       = errors.New("token is expired")
	ErrTokenAlreadyExists = errors.New("token already exists")
	ErrTokenNotFound      = errors.New("token does not exists")
)

func NewToken() (Token, error) {
	str := uuid.New().String()

	h := sha1.New()
	_, err := h.Write([]byte(str))
	if err != nil {
		return Token(""), err
	}

	return Token(
		base64.URLEncoding.EncodeToString(
			h.Sum(nil))), nil
}

type Token string

func (v Token) String() string { return string(v) }

func (v Token) IsZero() bool { return util.IsStringEmpty(v.String()) }

func (v Token) Equals(val Token) bool { return v.String() == val.String() }

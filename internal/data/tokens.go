package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"github.com/danyelkeddah/go-greenlight/internal/validator"
	"time"
)

const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
)

type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	randomBytes := make([]byte, 16)
	// fill with random bytes from operating system CSPRNG, This will return an error if the CSRPNG fails to function correctly
	_, err := rand.Read(randomBytes)

	if err != nil {
		return nil, err
	}
	// encode the byte to base-32 encoded string and assign it to token plain text field, this will be token string we send to the user in their welcome email,
	// note: base32 strings may be ended with = , We don;t need this padding character for purpose of our tokens, so we use the WithPadding(base32.NoPadding) method to omit them.
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	// generate SHA-256 hash of the plaintest token string. This will be the value that we store in the `hash` field of our database table.
	// sha256.Sum256 returns an array of length 32
	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token, nil
}

func ValidateTokenPlainText(v *validator.Validator, tokenPlainText string) {
	v.Check(tokenPlainText != "", "token", "must be provided")
	v.Check(len(tokenPlainText) == 26, "token", "must be 26 bytes long")
}

type TokenModel struct {
	DB *sql.DB
}

func (m TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.Insert(token)
	return token, err
}

func (m TokenModel) Insert(token *Token) error {
	query := `INSERT INTO tokens (hash, user_id, expiry, scope) VALUES ($1, $2, $3, $4)`
	args := []any{token.Hash, token.UserID, token.Expiry, token.Scope}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := m.DB.ExecContext(ctx, query, args...)

	return err
}

func (m TokenModel) DeleteAllForUser(scope string, userID int64) error {
	query := ` DELETE FROM tokens WHERE scope = $1 AND user_id = $2`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := m.DB.ExecContext(ctx, query, scope, userID)

	return err
}

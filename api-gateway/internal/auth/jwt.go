package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Validator struct {
	parser      *jwt.Parser
	key         []byte
	userIDClaim string
}

func NewHMACValidator(
	secret string,
	issuer string,
	audience string,
	userIDClaim string,
	leeway time.Duration,
) (*Validator, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, errors.New("JWT HMAC secret is required")
	}

	userIDClaim = strings.TrimSpace(userIDClaim)
	if userIDClaim == "" {
		userIDClaim = "sub"
	}

	options := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(leeway),
	}

	if issuer = strings.TrimSpace(issuer); issuer != "" {
		options = append(options, jwt.WithIssuer(issuer))
	}

	if audience = strings.TrimSpace(audience); audience != "" {
		options = append(options, jwt.WithAudience(audience))
	}

	return &Validator{
		parser:      jwt.NewParser(options...),
		key:         []byte(secret),
		userIDClaim: userIDClaim,
	}, nil
}

func (v *Validator) Validate(tokenString string) (string, error) {
	if v == nil || v.parser == nil {
		return "", errors.New("JWT validator is not initialized")
	}

	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return "", errors.New("JWT token is required")
	}

	claims := jwt.MapClaims{}
	token, err := v.parser.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected JWT signing method %q", token.Method.Alg())
			}
			return v.key, nil
		},
	)
	if err != nil {
		return "", fmt.Errorf("validate JWT: %w", err)
	}

	if !token.Valid {
		return "", errors.New("JWT is invalid")
	}

	userID, err := claimString(claims, v.userIDClaim)
	if err != nil {
		return "", err
	}

	return userID, nil
}

func claimString(claims jwt.MapClaims, name string) (string, error) {
	value, ok := claims[name]
	if !ok {
		return "", fmt.Errorf("JWT claim %q is required", name)
	}

	userID, ok := value.(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("JWT claim %q must be a non-empty string", name)
	}

	return userID, nil
}

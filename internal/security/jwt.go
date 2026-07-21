package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload carried by authenticated requests.
type Claims struct {
	User string `json:"user"`
	jwt.RegisteredClaims
}

// GenerateToken issues a signed HS256 JWT for the given user.
func GenerateToken(user, secret string, expireMinutes int) (string, error) {
	if expireMinutes <= 0 {
		expireMinutes = 60 * 24
	}
	now := time.Now()
	claims := Claims{
		User: user,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user,
			Issuer:    "go-Term",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireMinutes) * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken validates a token string and returns its claims.
func ParseToken(tokenStr, secret string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

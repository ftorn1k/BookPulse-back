package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWT struct {
	Secret []byte
}

func NewJWT(secret string) *JWT {
	return &JWT{Secret: []byte(secret)}
}

type Claims struct {
	UserID uint `json:"userId"`
	jwt.RegisteredClaims
}

func (j *JWT) GenerateToken(userID uint) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour * 30)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(j.Secret)
}

func (j *JWT) ParseToken(tokenStr string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		return j.Secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func MustAuth(r *http.Request, jwt *JWT) (int, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || h[:len(prefix)] != prefix {
		return 0, false
	}
	token := h[len(prefix):]
	claims, err := jwt.ParseToken(token)
	if err != nil {
		return 0, false
	}
	return int(claims.UserID), true
}

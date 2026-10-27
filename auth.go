package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const cookieName = "chat_token"

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

type claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func (s *Server) newToken(userID int64, username string) (string, error) {
	c := claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmtInt64(userID),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString(s.cfg.JWTSecret)
}

func (s *Server) parseToken(tokenStr string) (int64, string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.cfg.JWTSecret, nil
	})
	if err != nil || !tok.Valid {
		return 0, "", errors.New("invalid token")
	}
	c, ok := tok.Claims.(*claims)
	if !ok {
		return 0, "", errors.New("invalid claims")
	}
	id, err := parseInt64(c.Subject)
	if err != nil {
		return 0, "", err
	}
	return id, c.Username, nil
}

// tokenFromRequest extracts a token from Authorization header, cookie, or ?token=.
func (s *Server) tokenFromRequest(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie(cookieName); err == nil {
		return c.Value
	}
	return r.URL.Query().Get("token")
}

func (s *Server) authenticate(r *http.Request) (int64, error) {
	tok := s.tokenFromRequest(r)
	if tok == "" {
		return 0, errors.New("missing token")
	}
	id, _, err := s.parseToken(tok)
	return id, err
}

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600,
	})
}

func clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
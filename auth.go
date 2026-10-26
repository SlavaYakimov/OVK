package main

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c *credentials) validate() string {
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	if _, err := mail.ParseAddress(c.Email); err != nil {
		return "некорректный email"
	}
	if len(c.Password) < 8 || len(c.Password) > 72 { // bcrypt режет после 72 байт
		return "пароль от 8 до 72 символов"
	}
	return ""
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := readJSON(w, r, &c); err != nil {
		writeErr(w, 400, "некорректный JSON")
		return
	}
	if msg := c.validate(); msg != "" {
		writeErr(w, 400, msg)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	var id int64
	err = a.db.QueryRow(r.Context(),
		`INSERT INTO users(email, password_hash) VALUES($1,$2) RETURNING id`, c.Email, string(hash)).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		writeErr(w, 409, "пользователь уже существует")
		return
	}
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "email": c.Email})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := readJSON(w, r, &c); err != nil {
		writeErr(w, 400, "некорректный JSON")
		return
	}
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	var id int64
	var hash string
	err := a.db.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE email=$1`, c.Email).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil) {
		writeErr(w, 401, "неверный email или пароль")
		return
	}
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	token, err := a.makeToken(id)
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	writeJSON(w, 200, map[string]string{"token": token})
}

func (a *App) makeToken(userID int64) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
}

type ctxKey struct{}

// auth — middleware: без валидного Bearer-токена отдаём 401.
func (a *App) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			writeErr(w, 401, "нужна авторизация")
			return
		}
		claims := &jwt.RegisteredClaims{}
		_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			return a.secret, nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			writeErr(w, 401, "токен недействителен")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, claims.Subject)
		next(w, r.WithContext(ctx))
	})
}

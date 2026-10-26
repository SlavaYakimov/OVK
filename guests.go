package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type Guest struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
	Room     string `json:"room"`
	CheckIn  string `json:"check_in"`  // YYYY-MM-DD
	CheckOut string `json:"check_out"` // YYYY-MM-DD
	Note     string `json:"note"`
}

// validate нормализует поля и возвращает map ошибок по полям (пустая — ок).
func (g *Guest) validate() map[string]string {
	errs := map[string]string{}
	g.FullName = strings.TrimSpace(g.FullName)
	g.Phone = strings.TrimSpace(g.Phone)
	g.Room = strings.TrimSpace(g.Room)
	if n := utf8.RuneCountInString(g.FullName); n < 2 || n > 200 {
		errs["full_name"] = "от 2 до 200 символов"
	}
	digits := 0
	for _, r := range g.Phone {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case strings.ContainsRune("+-() ", r):
		default:
			digits = -100
		}
	}
	if digits < 10 || digits > 15 {
		errs["phone"] = "10–15 цифр, допустимы + - ( ) и пробел"
	}
	if g.Room == "" || len(g.Room) > 10 {
		errs["room"] = "обязателен, до 10 символов"
	}
	in, err1 := time.Parse(time.DateOnly, g.CheckIn)
	out, err2 := time.Parse(time.DateOnly, g.CheckOut)
	if err1 != nil {
		errs["check_in"] = "формат YYYY-MM-DD"
	}
	if err2 != nil {
		errs["check_out"] = "формат YYYY-MM-DD"
	}
	if err1 == nil && err2 == nil && !out.After(in) {
		errs["check_out"] = "выезд должен быть позже заезда"
	}
	if utf8.RuneCountInString(g.Note) > 2000 {
		errs["note"] = "до 2000 символов"
	}
	return errs
}

const guestCols = `id, full_name, phone, room, to_char(check_in,'YYYY-MM-DD'), to_char(check_out,'YYYY-MM-DD'), note`

func scanGuest(row pgx.Row) (Guest, error) {
	var g Guest
	err := row.Scan(&g.ID, &g.FullName, &g.Phone, &g.Room, &g.CheckIn, &g.CheckOut, &g.Note)
	return g, err
}

func (a *App) listGuests(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	sql, args := `SELECT `+guestCols+` FROM guests`, []any{}
	if q != "" {
		// выражение совпадает с индексом guests_search_trgm -> Bitmap Index Scan
		sql += ` WHERE (full_name || ' ' || phone || ' ' || room) ILIKE '%' || $1 || '%'`
		args = append(args, q)
	}
	sql += ` ORDER BY check_in DESC, id DESC LIMIT 200`
	rows, err := a.db.Query(r.Context(), sql, args...)
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	defer rows.Close()
	guests := []Guest{}
	for rows.Next() {
		g, err := scanGuest(rows)
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		guests = append(guests, g)
	}
	writeJSON(w, 200, guests)
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (a *App) getGuest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, 400, "некорректный id")
		return
	}
	g, err := scanGuest(a.db.QueryRow(r.Context(), `SELECT `+guestCols+` FROM guests WHERE id=$1`, id))
	a.respondGuest(w, g, err, 200)
}

func (a *App) createGuest(w http.ResponseWriter, r *http.Request) {
	var g Guest
	if err := readJSON(w, r, &g); err != nil {
		writeErr(w, 400, "некорректный JSON")
		return
	}
	if errs := g.validate(); len(errs) > 0 {
		writeJSON(w, 422, map[string]any{"error": "ошибка валидации", "fields": errs})
		return
	}
	g, err := scanGuest(a.db.QueryRow(r.Context(), `INSERT INTO guests(full_name, phone, room, check_in, check_out, note)
		VALUES($1,$2,$3,$4,$5,$6) RETURNING `+guestCols, g.FullName, g.Phone, g.Room, g.CheckIn, g.CheckOut, g.Note))
	a.respondGuest(w, g, err, 201)
}

func (a *App) updateGuest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, 400, "некорректный id")
		return
	}
	var g Guest
	if err := readJSON(w, r, &g); err != nil {
		writeErr(w, 400, "некорректный JSON")
		return
	}
	if errs := g.validate(); len(errs) > 0 {
		writeJSON(w, 422, map[string]any{"error": "ошибка валидации", "fields": errs})
		return
	}
	g, err := scanGuest(a.db.QueryRow(r.Context(), `UPDATE guests SET full_name=$1, phone=$2, room=$3,
		check_in=$4, check_out=$5, note=$6, updated_at=now() WHERE id=$7 RETURNING `+guestCols,
		g.FullName, g.Phone, g.Room, g.CheckIn, g.CheckOut, g.Note, id))
	a.respondGuest(w, g, err, 200)
}

func (a *App) deleteGuest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, 400, "некорректный id")
		return
	}
	tag, err := a.db.Exec(r.Context(), `DELETE FROM guests WHERE id=$1`, id)
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, 404, "гость не найден")
		return
	}
	w.WriteHeader(204)
}

func (a *App) respondGuest(w http.ResponseWriter, g Guest, err error, status int) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, 404, "гость не найден")
		return
	}
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	writeJSON(w, status, g)
}

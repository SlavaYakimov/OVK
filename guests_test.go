package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuestValidate(t *testing.T) {
	cases := []struct {
		name     string
		g        Guest
		badField string
	}{
		{"ok", Guest{FullName: "Иванов Иван", Phone: "+7 (900) 123-45-67", Room: "101", CheckIn: "2026-10-20", CheckOut: "2026-10-22"}, ""},
		{"checkout before checkin", Guest{FullName: "Иванов Иван", Phone: "+79001234567", Room: "101", CheckIn: "2026-10-22", CheckOut: "2026-10-20"}, "check_out"},
		{"bad phone", Guest{FullName: "Иванов Иван", Phone: "abc", Room: "101", CheckIn: "2026-10-20", CheckOut: "2026-10-22"}, "phone"},
		{"empty name", Guest{FullName: " ", Phone: "+79001234567", Room: "101", CheckIn: "2026-10-20", CheckOut: "2026-10-22"}, "full_name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := c.g.validate()
			if c.badField == "" && len(errs) != 0 {
				t.Fatalf("ожидали без ошибок, получили %v", errs)
			}
			if c.badField != "" && errs[c.badField] == "" {
				t.Fatalf("ожидали ошибку в %s, получили %v", c.badField, errs)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	a := &App{secret: []byte("test-secret")}
	h := a.auth(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	good, _ := a.makeToken(1)
	forged, _ := (&App{secret: []byte("other")}).makeToken(1)
	for name, tc := range map[string]struct {
		header string
		want   int
	}{
		"no token":     {"", 401},
		"garbage":      {"Bearer abc", 401},
		"wrong secret": {"Bearer " + forged, 401},
		"valid":        {"Bearer " + good, 200},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/guests", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

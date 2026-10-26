package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db     *pgxpool.Pool
	secret []byte
}

func main() {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := migrate(ctx, db); err != nil {
		log.Fatal("migrate: ", err)
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return // только миграции: go run . migrate
	}
	app := &App{db: db, secret: []byte(mustEnv("JWT_SECRET"))}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, app.routes()))
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", a.register)
	mux.HandleFunc("POST /api/login", a.login)
	mux.Handle("GET /api/guests", a.auth(a.listGuests))
	mux.Handle("POST /api/guests", a.auth(a.createGuest))
	mux.Handle("GET /api/guests/{id}", a.auth(a.getGuest))
	mux.Handle("PUT /api/guests/{id}", a.auth(a.updateGuest))
	mux.Handle("DELETE /api/guests/{id}", a.auth(a.deleteGuest))
	mux.Handle("/", http.FileServer(http.Dir("web")))
	return mux
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("env %s is required", k)
	}
	return v
}

up:        ; docker compose up --build
migrate:   ; docker compose run --rm api /app/hotel migrate
test:      ; go test ./...
run:       ; set -a; . ./.env; set +a; go run .

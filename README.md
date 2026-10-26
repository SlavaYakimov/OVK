# Записная книжка отеля

Go (stdlib `net/http`, роутинг Go 1.22+) + PostgreSQL + vanilla JS.
Регистрация и логин (bcrypt + JWT HS256), CRUD гостей, поиск по подстроке (pg_trgm).

## Запуск (≈2 минуты)

```bash
cp .env.example .env          # при желании поменять JWT_SECRET
docker compose up --build     # БД + API, миграции применяются при старте
open http://localhost:8080    # UI: «Регистрация», затем работа с гостями
```

Без Docker для API (БД всё равно из compose):

```bash
docker compose up -d db
set -a; . ./.env; set +a
go run . migrate   # только миграции (одна команда)
go run .           # сервер на :8080
```

Тесты (одна команда, БД не нужна): `go test ./...`

## API

Все `/api/guests*` требуют `Authorization: Bearer <token>`, без него → `401`.

| Метод | Путь | Ответ |
|---|---|---|
| POST | /api/register `{email,password}` | 201 / 400 / 409 |
| POST | /api/login `{email,password}` | 200 `{token}` / 401 |
| GET | /api/guests?q=строка | 200 `[]Guest` (поиск по ФИО, телефону, номеру) |
| POST | /api/guests | 201 / 422 `{error, fields}` |
| GET/PUT/DELETE | /api/guests/{id} | 200 / 204 / 404 / 422 |

Guest: `full_name, phone, room, check_in, check_out (YYYY-MM-DD), note`.

```bash
curl -i localhost:8080/api/guests                                    # 401
curl -XPOST localhost:8080/api/register -d '{"email":"a@b.ru","password":"secret123"}'
TOKEN=$(curl -s -XPOST localhost:8080/api/login -d '{"email":"a@b.ru","password":"secret123"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
curl -H "Authorization: Bearer $TOKEN" -XPOST localhost:8080/api/guests \
  -d '{"full_name":"Иванов Иван","phone":"+79001234567","room":"101","check_in":"2026-10-20","check_out":"2026-10-22","note":""}'
curl -H "Authorization: Bearer $TOKEN" 'localhost:8080/api/guests?q=иван'
```

## Решения

- **Структура**: один пакет `main`, файлы по ответственности (`main.go` — конфиг и роуты,
  `auth.go`, `guests.go`, `migrate.go`, `httputil.go`). Для такого объёма слои
  handler/service/repository — лишняя церемония; при росте вынес бы `internal/store`.
- **Миграции**: свой раннер на 50 строк — `embed` + таблица `schema_migrations`, каждая
  миграция в транзакции. Не тянет зависимостей, легко объяснить. Альтернатива — golang-migrate.
- **Индекс**: `GIN (full_name || ' ' || phone || ' ' || room) gin_trgm_ops`. Поиск идёт
  `ILIKE '%строка%'`, B-tree с ведущим `%` не работает, триграммный GIN работает
  (проверено `EXPLAIN`: Bitmap Index Scan on guests_search_trgm). Выражение в запросе
  должно совпадать с выражением индекса.
- **Целостность**: `CHECK (check_out > check_in)` в БД плюс та же проверка в Go, чтобы
  отдать понятную ошибку по полю (422).
- **Безопасность**: bcrypt cost 10, пароль 8–72 байта (лимит bcrypt), одинаковый ответ
  на «нет пользователя» и «неверный пароль», JWT с `exp` 24 ч и явным `WithValidMethods`,
  тело запроса ограничено 1 МБ, `DisallowUnknownFields`, только параметризованный SQL.
  Во фронте вывод через `textContent` (без XSS).

## Что не доделано / что сделал бы иначе

- Нет пагинации (стоит `LIMIT 200`), ролей, refresh-токенов и logout на сервере.
- Токен в `localStorage` — для продакшена лучше httpOnly cookie + CSRF.
- Нет rate limit на `/login`, нет структурного логирования и graceful shutdown.
- Нет проверки пересечения бронирований одного номера (решается `EXCLUDE USING gist`).
- Тесты только unit (валидация, middleware); интеграционные — через testcontainers.
- Down-миграции лежат рядом, но раннер их не применяет.

## Link Status Service

Go-based web server that validates availability of external links, keeps per-request history, and exports PDF reports on demand. Each submitted batch receives a sequential identifier so clients can later request aggregated status reports.

### Features
- `POST /api/v1/links/check` – accepts one or many links, returns availability (`available` / `not available`) and assigns `links_num`.
- `POST /api/v1/links/report` – takes a list of `links_num` identifiers and responds with a PDF report summarising their statuses.
- Persistent on-disk storage (`data/links_store.json`) keeps request history and survives restarts.
- Pending jobs are re-processed after restart to minimise data loss if shutdown happens during processing.
- Graceful shutdown waits for in-flight HTTP requests and cancels background work safely.
- Basic health endpoint `GET /healthz`.

### Getting Started
```bash
go run ./cmd/server
```

Environment / flags:
- `PORT` or `-addr` sets the listen address (default `:8080`).
- `-storage` path to the persistence file (default `data/links_store.json`).
- `-timeout` change the per-link HTTP timeout (default `5s`).
- `-workers` tune concurrency for link checks (default `5`).

The service stores state in the path given by `-storage`. Ensure the process has read/write permissions.

### API Overview

**Check links**
```http
POST /api/v1/links/check
Content-Type: application/json

{
  "links": ["google.com", "https://example.org"]
}
```

Response:
```json
{
  "links": {
    "google.com": "available",
    "https://example.org": "not available"
  },
  "links_num": 1,
  "links_sum": 1
}
```

**Generate report**
```http
POST /api/v1/links/report
Content-Type: application/json

{
  "links_list": [1, 2]
}
```

Response: `application/pdf` attachment with aggregated link information.

### Architecture
- **Entity layer** (`internal/entity`) содержит сущности и бизнес-ошибки.
- **Use case layer** (`internal/usecase`) инкапсулирует сценарии – проверку ссылок, генерацию отчётов, восстановление незавершённых задач.
- **Service layer** (`internal/service`) адаптирует интерфейсы: REST API, HTTP-checker, PDF-генератор и пр.
- **Repository layer** (`internal/repository`) отвечает за персистентность (файловый стор).
- **App layer** (`internal/app`) собирает зависимости и управляет жизненным циклом приложения; `cmd/server` лишь парсит конфигурацию и запускает `app.Run`.

### Persistence & Reliability
- Requests are recorded in pending state before any network checks run.
- Each record stores the original URLs, timestamps, and current statuses.
- On restart, the service scans for pending records and replays them so no queued work is lost.
- `http.Server.Shutdown` ensures ongoing requests finish cleanly before exit.

### Testing
```bash
go test ./...
```

### Notes & Limitations
- Link availability is determined via HTTP `GET` requests (HTTPS first, then HTTP). Hosts without HTTP(S) will be marked `not available`.
- PDF reports are generated with [`gofpdf`](https://github.com/jung-kurt/gofpdf); no external services are required.
- Data file growth is linear with the number of requests; archive or rotate `data/links_store.json` as needed.



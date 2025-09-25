# Hotel Merge Service

This project implements a lightweight hotel data procurement and merge pipeline. It periodically pulls content from multiple suppliers, normalises and merges the payloads, stores traceable snapshots, and serves merged records via a Gin-based HTTP API.

## Features
- Periodic supplier fetch and merge loops with in-memory KV storage
- Supplier-specific adapters for ACME, Patagonia, and Paperflies endpoints
- Deterministic merge rules with amenity/image/booking-condition deduplication
- Audit log snapshots and a built-in debug UI for inspecting raw vs. merged data
- `/hotels` API with filtering by destination and hotel IDs

## Requirements
- Go 1.22 or newer
- Internet access to reach the mock supplier endpoints

## Getting Started
1. **Install dependencies** (Go handles this automatically):
   ```sh
   go mod tidy
   ```
2. **Run the service**:
   ```sh
   make run
   ```
   The API listens on `http://localhost:8080` by default (set `PORT` to override).
3. **Query merged hotels**:
   ```sh
   curl 'http://localhost:8080/hotels?destination=5432'
   ```

## Handy Commands
All convenience targets live in the `Makefile` at repository root:
- `make run` – start the server
- `make build` – compile into `./bin/hotel-merge`
- `make test` – run unit tests
- `make fmt` / `make lint` – format code or run `go vet`
- `make check` – `fmt`, `lint`, and `test` in one go
- `make clean` – remove the build artefact directory

## Debugging & Observability
- **Debug UI**: visit `http://localhost:8080/debug/records` for a table view of every stored record. Click an entry to inspect supplier payloads, normalised data, and the merged canonical hotel.
- **Audit trail**: the in-memory audit store keeps chronological entries per destination. Extend `store.AuditStore` if you need persistent auditing.
- **Cron loops**: fetch and merge tickers start immediately at launch and repeat every 5 minutes and 1 minute respectively (tune in `cmd/server/main.go`).
- **Logging**: the service logs fetch/merge issues to stdout with the `hotel-merge` prefix.

## Project Layout
```
cmd/server       Entry point wiring Gin, suppliers, and background jobs
internal/api     HTTP handlers (public API + debug UI)
internal/app     Service coordination logic and cron loops
internal/domain  Shared domain structures
internal/suppliers Supplier adapters and schema normalisation
internal/store   In-memory KV store and audit log implementations
internal/merge   Merge strategy and tests
```

## Testing
Run the unit tests:
```sh
make test
```
The suite exercises merge rules and store lifecycle behaviour (`internal/merge` and `internal/store`). Extend with integration tests by mocking supplier adapters if needed.

## Configuration Notes
- Supplier endpoints are hard-coded in `cmd/server/main.go` for the mock APIs.
- Fetch (`FetchInterval`) and merge (`MergeInterval`) timers default to 5 minutes and 1 minute; adjust via the `app.Config` initialisation if required.
- Swap `store.NewMemoryStore` / `NewMemoryAuditStore` with persistent implementations to back the service with external storage.

Happy hacking!

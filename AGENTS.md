# AGENTS.md — TSX Evaluator

## Repo layout

- `module github.com/example/tsx-evaluator` (Go 1.25+), binary output: `bin/server` via `make build` or `go run ./cmd/server`
- `internal/analyzer/` — orchestrates all 4 scoring modules per symbol; `analyzer.Analyze()` is the single entrypoint for evaluation logic
- `internal/evaluator/` — background loop that pulls symbols from tsx-tracker via gRPC and calls `analyzer.Analyze()` in batches
- `gen/tsx/v1/` — generated protobuf/gRPC code. **Regenerate before build:** `make proto` (buf) or `make proto-protoc` (protoc)
- `internal/db/` — PostgreSQL repo, migrations embedded via `//go:embed`. Schema lives in `internal/db/migrations/0001_init.sql`, auto-applied at startup
- `go.mod` has `replace github.com/example/tsx-tracker => ../tsx-tracker` — the tsx-tracker sibling directory must exist locally. Without it, `go build` fails

## Quick commands

```bash
make proto   # regenerate protobuf
make build   # proto + go build → bin/server
make run     # proto + go run ./cmd/server
make tidy    # go mod tidy
docker compose up --build   # full stack (postgres + tsx-tracker + evaluator)
```

## Must-know details

- **tsx-tracker dependency:** The local `replace` directive points to a sibling repo (`../tsx-tracker`). Agent edits to proto files in either repo require regenerating code in both. The tracker also runs as a gRPC service on port 50051 (default).
- **Default env loading:** `.env.example` is not auto-loaded by Go. Export variables or use `golang-dotenv` if configured. Key required variable: `FMP_API_KEY`. Without it, financial data fetching silently fails.
- **LLM dependency:** Ollama serves on default port 11434. Set `LLM_BASE_URL` and verify the model (default `ornith:35b`) is pulled locally before running. Long timeouts (`LLM_TIMEOUT=30m` in `.env.example`) may be needed — confirm actual requirement.
- **Evaluation cycle:** Background goroutine polls tsx-tracker for all tracked companies, fetches already-evaluated symbols from the DB, shuffles candidates, and evaluates up to `EVAL_BATCH_SIZE` per cycle (default 1). Unevaluated symbols have priority; rebalancing fills remaining slots.
- **DB is critical:** Default `DB_NAME=tsx_evaluator`. If recreating the DB, ensure `createdb tsx_evaluator` runs before first startup — migrations apply automatically but require a live connection.
- **Proto generation path conflict:** `Containerfile` uses protoc with `-I proto`, while `buf.yaml` specifies `path: proto`. Both work; keep them in sync when adding new messages or RPCs.

## Beginning Tasks
Before you begin a task, follow these steps:
1. Make sure all existing changes have been checked in; if there are existing changes, check them in.
2. Do a git fetch so that you have all of the latest changes.
3. Switch to a branch or create a branch appropriate for the changes that you will make

## Finishing Tasks
Once you complete any changes, additions, deletions, or modifications, follow these steps:
1. Check the code into a branch using git
2. Push the code to GitHub
3. Open a Pull Request for the changes you just pushed
4. Add me (bbwheeler) as a reviewer on the Pull Request

Your GitHub credentials can be found in the parent directory (../github.md)
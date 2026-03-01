# Design: Repo Restructure + Deployment

**Date:** 2026-03-01
**Status:** Approved

---

## Context

The Go app currently lives under `src/`, with legacy reference data in `data/` and docs split between `src/docs/` and the repo root. This creates friction in tooling (Makefile, Docker, gopls, air) and is not idiomatic Go. The goal is a flat, canonical Go repo layout with clearly separated archive material and a production-ready Docker + Compose setup.

---

## 1. Repo Structure

Move the Go module to the repo root. `src/` contents move up; legacy data consolidates into `archive/`.

```
steuerpilot/
├── go.mod                    (module: steuerpilot)
├── go.sum
├── main.go
├── Makefile
├── .env.example
├── .air.toml
├── .gitignore
│
├── config/config.go
├── middleware/session.go
├── handlers/
│   ├── pages.go
│   ├── wizard.go
│   ├── htmx.go
│   └── api.go
├── internal/
│   ├── models/
│   ├── tax/
│   ├── claude/
│   ├── session/
│   ├── export/
│   └── util/
├── templates/
│   ├── layout/
│   ├── pages/
│   ├── wizard/
│   ├── components/
│   └── partials/
├── static/
│
├── docs/
│   ├── steuerparameter.json  (source of truth, loaded at runtime)
│   ├── SPEC.md
│   ├── migration.md
│   ├── wegleitung2025.pdf
│   └── plans/               (this file and future design docs)
│
├── archive/                  (legacy reference, not part of build)
│   ├── drafts/               (old TypeScript source files)
│   ├── etax/                 (eTax API analysis JSONs)
│   ├── legacy_node.tar.xz
│   └── tarif2025.{pdf,txt}
│
├── Dockerfile
├── compose.yml
└── CLAUDE.md
```

**Key changes:**
- `src/` contents move to repo root
- module name: `steuerpilot-go` → `steuerpilot`
- `data/` → `archive/` (including `drafts/`, `etax/`)
- `src/docs/` → `docs/` at repo root
- `setup.sh` removed (superseded by `make tools`)

---

## 2. Dockerfile (multi-stage)

```dockerfile
# Stage 1: build
FROM golang:1.23-alpine AS builder
RUN go install github.com/a-h/templ/cmd/templ@latest
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN templ generate ./templates/... && go build -o steuerpilot .

# Stage 2: runtime
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/steuerpilot .
COPY --from=builder /app/docs/steuerparameter.json ./docs/
COPY --from=builder /app/static/ ./static/
EXPOSE 3000
CMD ["./steuerpilot"]
```

Runtime image contains only: binary, `docs/steuerparameter.json`, `static/`.

---

## 3. compose.yml

```yaml
services:
  steuerpilot:
    build: .
    ports:
      - "3000:3000"
    env_file: .env
    restart: unless-stopped
    volumes:
      - ./docs/steuerparameter.json:/app/docs/steuerparameter.json:ro
```

- `steuerparameter.json` mounted as read-only volume — updateable without image rebuild
- In-memory sessions: no database volume needed
- `.env` not committed; `.env.example` provides the template

---

## 4. Versioning

Git tags (semver: `v0.1.0`, `v0.2.0`) embedded into binary via ldflags:

```makefile
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
    templ generate ./templates/...
    go build -ldflags "-X main.version=$(VERSION)" -o steuerpilot .
```

`/health` endpoint returns `{"ok": true, "version": "v0.1.2-3-gabcdef"}`.

Tag workflow: `git tag v0.1.0 && git push --tags`

---

## 5. Documentation

```
docs/
├── steuerparameter.json      runtime data
├── SPEC.md                   technical specification
├── migration.md              Node→Go migration notes
├── wegleitung2025.pdf        reference PDF
└── plans/                    design + implementation plan docs
```

`README.md` at repo root: what the app does, quickstart (`make dev`), env vars, Docker run.

`.gitignore` additions: `.env`, `tmp/`, `steuerpilot` (binary). PDFs optional.

---

## Implementation Steps

See implementation plan (to be written).

1. Move `src/` contents to repo root (`git mv`)
2. Rename `data/` → `archive/`
3. Move `src/docs/` → `docs/`, consolidate docs
4. Update `go.mod` module name + all import paths
5. Update `Makefile`, `.air.toml`, `.env.example` paths
6. Write `Dockerfile` + `compose.yml`
7. Add version ldflags to Makefile + `/health` handler
8. Write `README.md`
9. Update `CLAUDE.md` paths
10. Test: `make build`, `make test-calc`, `docker build .`

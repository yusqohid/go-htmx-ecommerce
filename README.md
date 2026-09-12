# Sellora

Sellora is a single-vendor digital product commerce platform built with Go, HTMX, PostgreSQL, and server-rendered HTML.

## Tech Stack

- **Backend:** Go 1.26
- **Routing:** Chi Router (`github.com/go-chi/chi/v5`)
- **Database:** PostgreSQL with `pgx/v5`
- **Frontend:** Server-rendered HTML (`html/template`) + HTMX
- **Architecture:** Modular monolith, provider-agnostic payment abstraction

---

## Getting Started

### 1. Prerequisites

- Go 1.25+ installed
- PostgreSQL 14+ running

### 2. Environment Setup

Copy `.env.example` to `.env`:

```bash
cp .env.example .env
```

Adjust the `DATABASE_URL` in `.env` to match your local PostgreSQL credentials:

```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/sellora?sslmode=disable
```

### 3. Run Database Migrations

Apply all SQL migrations:

```bash
go run cmd/migrate/main.go
```

### 4. Run the Application Server

Start the web server:

```bash
go run cmd/server/main.go
```

Access the health check endpoint:

```bash
curl http://localhost:8080/healthz
```

---

## Testing

Run all unit tests:

```bash
go test ./... -v
```
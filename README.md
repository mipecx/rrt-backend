# RRT System — Backend Core

Real-time emergency response system backend: tourist and rescuer geolocation, SOS incidents, and incident management.

Go 1.26 · PostgreSQL + PostGIS · Redis · JWT auth · WebSocket

## Features

- Phone + password registration/login (E.164 format), OTP via SMS gateway
- JWT access tokens + refresh tokens (rotated, stored in Redis)
- Incident lifecycle: create → assign RRT crew → arrive → resolve
- Live geolocation of tourists and RRT crews via WebSocket broadcast
- PostGIS-based coordinates and sector polygons
- Automatic migrations (`golang-migrate`)
- Seed data for a Pattaya demo (see `scripts/seed.sql`)

## Architecture

```
cmd/main.go          Entry point: wiring, HTTP server, graceful shutdown
internal/
├── config/          Env-based configuration (.env)
├── middleware/      JWT auth + role-based access control
├── ws/              WebSocket hub (send-only broadcast)
├── logger/          Structured JSON logger with trace ids
└── domain/
    ├── auth/        OTP, register, login, refresh, logout (JWT + Redis)
    ├── incidents/   Incident lifecycle + SOS (REST + WebSocket)
    └── rrt/         RRT crews and their locations
migrations/          SQL migrations (PostGIS)
scripts/             Seed data + GPS location simulator
```

## Quick Start

```bash
cp .env.example .env   # fill in the values
docker compose up --build
```

This starts PostgreSQL (with PostGIS), Redis, applies migrations and runs the API on `:8080`.

### Standalone

```bash
go run cmd/main.go
```

## API

Base path: `/api/v1`

### Healthcheck

```http
GET /api/v1/health
```

### Auth

| Method | Path | Body |
|---|---|---|
| POST | `/auth/send-otp` | `{ "phone": "+79991234567" }` |
| POST | `/auth/register` | `{ "phone", "code", "password", "full_name", "role" }` |
| POST | `/auth/login` | `{ "phone", "password" }` |
| POST | `/auth/refresh` | `{ "refresh_token" }` |
| POST | `/auth/logout` | `{ "refresh_token" }` |

Roles: `tourist`, `rrt`, `dispatcher`.

### Incidents

| Method | Path | Access |
|---|---|---|
| POST | `/incidents` | authenticated |
| GET | `/incidents` | dispatcher, rrt |
| PUT | `/incidents/{id}/assign` | dispatcher |
| PUT | `/incidents/{id}/arrive` | dispatcher, rrt |
| PUT | `/incidents/{id}/resolve` | dispatcher, rrt |
| PUT | `/incidents/{id}/location` | tourist, rrt |

### RRT crews

| Method | Path | Access |
|---|---|---|
| POST | `/rrt` | dispatcher |
| GET | `/rrt` | dispatcher |
| PUT | `/rrt/{id}/status` | rrt |
| PUT | `/rrt/{id}/location` | rrt |

### WebSocket

```http
GET /api/v1/ws
```

Broadcasts real-time events:
- `{"type":"INCIDENT_UPDATE"}` — incident created / updated
- `{"type":"RRT_UPDATE","data":{...}}` — crew location / status change

## Configuration

See `.env.example`. In production set a strong `JWT_SECRET` and real SMS gateway credentials.

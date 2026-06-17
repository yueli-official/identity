# identity-service

User-center IdP backend — milestone 2 scope: identity management, email/password
login, Redis-backed sessions, and login rate-limiting / lockout.

## Purpose

Provides user registration, authentication, and session management for the
nuxtblog platform. Stores identities and credentials in PostgreSQL; sessions and
throttle counters live exclusively in Redis (no PG writes on hot paths).

## Environment variables

| Variable              | When required          | Description                                            |
|-----------------------|------------------------|--------------------------------------------------------|
| `IDENTITY_PG_LINK`    | Runtime                | PostgreSQL link string: `pgsql:user:pass@tcp(host:5432)/identity` |
| `IDENTITY_REDIS_ADDR` | Runtime                | Redis address: `host:6379`                             |
| `TEST_PG_LINK`        | Integration tests only | Same format as `IDENTITY_PG_LINK`, test DB             |
| `TEST_REDIS_ADDR`     | Integration tests only | Redis address for integration tests                    |

GoFrame reads runtime secrets via its `GF_*` env-var override convention:

```sh
export GF_DATABASE_DEFAULT_LINK="$IDENTITY_PG_LINK"
export GF_REDIS_DEFAULT_ADDRESS="$IDENTITY_REDIS_ADDR"
```

## Apply migrations

Uses golang-migrate. Run against the target database before starting the service:

```sh
migrate -path manifest/sql/migrations \
        -database "$IDENTITY_PG_LINK" up
```

## Running tests

Hermetic (no external deps):

```sh
go test ./...
```

Integration tests (requires PostgreSQL + Redis with migrations applied):

```sh
export TEST_PG_LINK="pgsql:user:pass@tcp(127.0.0.1:5432)/identity_test"
export TEST_REDIS_ADDR="127.0.0.1:6379"
go test -tags integration ./...
```

## Endpoints

| Method | Path                      | Description                        |
|--------|---------------------------|------------------------------------|
| POST   | `/api/v1/auth/register`   | Register a new identity            |
| POST   | `/api/v1/auth/login`      | Email/password login; sets cookie  |
| POST   | `/api/v1/auth/logout`     | Invalidate current session         |
| GET    | `/api/v1/session/me`      | Return the current session profile |
| GET    | `/healthz`                | Liveness probe                     |

OpenAPI spec: `GET /api.json`  
Swagger UI: `GET /swagger`

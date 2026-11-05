# Warehouse service

Go gRPC inventory service with its own PostgreSQL database. Its contract is
`proto/warehouse/v1/warehouse.proto`.

## Run with Docker

```powershell
docker compose up -d --build
docker compose ps
docker compose logs --tail 50 warehouse
```

This starts PostgreSQL on `localhost:5433`, Redis on `localhost:6380`, and the
warehouse gRPC server on `localhost:50051`. Docker Compose supplies the database
connection for the container; a host-local `.env` `DATABASE_URL` is not used by
the container. The Compose credentials are local development defaults.

The migration job applies only the `-- +goose Up` portion of each SQL file and
records each successful migration in `schema_migrations`, in the same database
transaction. Re-running Compose skips recorded migrations. An existing database
that was initialized manually without migration records needs its history checked
before the job can adopt it; SQL failures leave the existing schema intact.

The warehouse joins the `ecommerce-warehouse` Docker network under the hostname
`warehouse`. Start this project before the NestJS Compose project. NestJS connects
to `warehouse:50051` over that network; the databases remain separate.

Existing ecommerce ORM inventory is not copied into this database automatically.
Initialize warehouse stock through the ecommerce admin inventory endpoints.
Existing ORM orders require their original inventory backend or an explicit
reservation migration before they can be fulfilled or cancelled through gRPC.

Stop services with `docker compose stop`. Database volumes are retained. Avoid
`docker compose down -v` unless you explicitly intend to delete development data.

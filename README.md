# IT Asset Inventory (Go, Gin and MySQL)

A small full-stack IT asset inventory application. The Go backend uses Gin for routing and the standard `net/http` server to serve the embedded frontend and REST API. MySQL stores employees and assets.

## Features

- Employee CRUD: create, list, view, update and delete employees.
- Asset CRUD: register, list, view, update and delete assets.
- Asset assignment and return workflows.
- Dashboard counts and asset status breakdown.
- Search assets by tag, name, serial number, category or assigned employee; filter by status.
- `/health` checks MySQL connectivity; `/health/live` checks that the process is responding.
- Database tables are created automatically at application startup.

## Requirements

- Go 1.23 or newer
- MySQL 8.x (or a compatible MySQL server)

## 1. Create the database

Create a MySQL database and an application user. Example commands (change the password):

```sql
CREATE DATABASE it_asset_inventory CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'appuser'@'localhost' IDENTIFIED BY 'change-this-password';
GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, REFERENCES ON it_asset_inventory.* TO 'appuser'@'localhost';
FLUSH PRIVILEGES;
```

The schema is also provided in `database-schema.sql`. The application runs `CREATE TABLE IF NOT EXISTS` migrations on startup, so the database itself must exist before starting the application.

## 2. Configure and run

Set environment variables in your terminal (or use `.env.example` as a reference; the app does not automatically parse `.env` files):

```bash
export DB_HOST=127.0.0.1
export DB_PORT=3306
export DB_NAME=it_asset_inventory
export DB_USER=appuser
export DB_PASSWORD='change-this-password'
export PORT=8080
```

Then run:

```bash
go mod tidy
go run .
```

Open `http://localhost:8080` in a browser. Health endpoint: `http://localhost:8080/health`.

On Windows PowerShell, use `$env:DB_HOST="127.0.0.1"` etc. instead of `export`.

## REST API

The application API routes are under `/api/v1`. For compatibility with the original starter code, the employee CRUD endpoints are also available at the original `/users` and `/users/:id` paths. The legacy paths accept the original `name` and `email` fields; `employee_id` is generated and `department` defaults to `Unassigned` when omitted.

### Employees

| Method | Endpoint | Purpose |
|---|---|---|
| POST | `/users` | Create employee |
| GET | `/users` | List employees |
| GET | `/users/:id` | Get employee |
| PUT | `/users/:id` | Update employee |
| DELETE | `/users/:id` | Delete employee (blocked while assets are assigned) |

Example employee JSON:

```json
{"employee_id":"EMP-001","name":"Alex Morgan","email":"alex@example.com","department":"IT"}
```

### Assets

| Method | Endpoint | Purpose |
|---|---|---|
| POST | `/assets` | Register an asset (starts as available) |
| GET | `/assets` | List assets; optional `q` and `status` query parameters |
| GET | `/assets/:id` | Get asset |
| PUT | `/assets/:id` | Update asset details/status while unassigned |
| DELETE | `/assets/:id` | Delete asset while unassigned |
| PATCH | `/assets/:id/assign` | Assign asset to an employee |
| PATCH | `/assets/:id/unassign` | Return assigned asset to inventory |

Example asset JSON:

```json
{
  "asset_tag":"AST-1001",
  "asset_name":"Dell Latitude 5440",
  "category":"Laptop",
  "manufacturer":"Dell",
  "model":"Latitude 5440",
  "serial_number":"SN-EXAMPLE-1001",
  "location":"Leipzig office",
  "purchase_date":"2025-01-15"
}
```

Assignment JSON: `{"user_id":1}`. Asset statuses are `available`, `assigned`, `maintenance` and `retired`. Assignment is performed only through the assign/unassign endpoints so the foreign-key relationship and status stay consistent. Assigned assets must be returned before editing or deleting them; employees with assigned assets cannot be deleted.

### Dashboard and health

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/dashboard/stats` | Inventory and employee totals |
| GET | `/health` | Readiness check including MySQL ping |
| GET | `/health/live` | Process liveness check |

## Assignment scope notes

This package contains the application and database implementation only. It intentionally does not include Dockerfiles, Compose files or Kubernetes manifests. The assignment brief states that AI assistance is permitted for website and database implementation but not for Docker and Kubernetes work; implement those deliverables independently.

## Assumptions

- The original CRUD behaviour is represented by employee CRUD at `/api/v1/users`; asset management is added as a second resource.
- The app listens on port `8080` by default; `PORT` can override it.
- The MySQL database is created separately; the app creates/updates the two required tables at startup.
- Employee email, employee ID, asset tag and non-empty serial numbers are unique.
- One asset can be assigned to at most one employee at a time; one employee may hold multiple assets.
- No authentication/authorization was specified in the assignment brief, so this educational app does not implement login. Do not expose it to an untrusted network without adding authentication and authorization.
- List endpoints are capped at 500 records for this project-scale implementation.

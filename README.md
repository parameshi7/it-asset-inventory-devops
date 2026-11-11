# IT Asset Inventory Management System

A web-based IT asset inventory application developed using Go, Gin, and MySQL. The application helps maintain employee records, register IT assets, and track which devices are assigned to employees.

The project also demonstrates how the application can be containerized using Docker, deployed on Kubernetes using Minikube, and tested for performance and availability.

## 1. Project Overview

The main purpose of this project is to manage IT assets through a simple web interface and REST API.

The application supports employee management, asset registration, asset assignment, and basic inventory reporting. MySQL is used to store the information.

The backend was developed in Go using the Gin framework. The frontend uses HTML, CSS, and JavaScript and is embedded into the Go executable.

The DevOps part of the project focuses on containerization, deployment, health monitoring, load testing, and recovery from application failures.

### Technologies Used

| Component | Technology |
|---|---|
| Backend | Go and Gin |
| Frontend | HTML, CSS, JavaScript |
| Database | MySQL 8.0 |
| Containerization | Docker |
| Multi-container setup | Docker Compose |
| Container orchestration | Kubernetes |
| Local Kubernetes cluster | Minikube |
| Load testing | Grafana k6 |
| Monitoring | Kubernetes Metrics Server |
| Version control | Git and GitHub |
| Development environment | Windows 11, VS Code, PowerShell |

## 2. Application Features

### Employee Management

The application allows users to:

- Register employees with their employee ID, name, email, and department.
- View all registered employees.
- Update employee information.
- Delete employee records when no assets are assigned.

### Asset Management

IT assets can be registered with details such as asset tag, asset name, category, manufacturer, model, serial number, location, and purchase date.

The application supports creating, viewing, updating, and deleting assets.

The available asset statuses are:

- Available
- Assigned
- Maintenance
- Retired

### Asset Assignment

An available asset can be assigned to an employee. The system maintains the relationship between the employee and the assigned asset.

An assigned asset can also be returned to inventory using the unassign operation.

### Dashboard

The dashboard displays inventory information, including total assets, assigned assets, available assets, and registered employees.

The interface also provides asset searching and status-based filtering.

## 3. Project Structure

The project is organized as follows:

```text
DevOps Final/
|
|-- Dockerfile
|-- docker-compose.yaml
|-- .gitignore
|-- README.md
|-- database-schema.sql
|-- go.mod
|-- go.sum
|-- main.go
|
|-- internal/
|   |-- database/
|   |   |-- database.go
|   |
|   |-- handlers/
|       |-- handlers.go
|
|-- web/
|   |-- index.html
|   |-- app.css
|   |-- app.js
|
|-- k8s/
|   |-- deployment.yaml
|   |-- service.yaml
|   |-- mysql-deployment.yaml
|   |-- mysql-service.yaml
|
|-- inventory-load-test.js
|-- load-test-results.json
|-- sustained-load-results.json
|-- pod-metrics.csv
```

Database credentials are stored in a local `.env` file, which is excluded from Git.

Large raw load-testing files and compiled executables are also excluded from the repository.

## 4. Database Configuration

MySQL 8.0 is used to store employee and asset information.

The application connects to MySQL when it starts and runs database migrations to create the required tables if they do not already exist.

The database schema is also available in `database-schema.sql`.

The default database settings used in this project are:

| Setting | Value |
|---|---|
| Database | `it_asset_inventory` |
| Database user | `appuser` |
| MySQL port | `3306` |
| Database host in Docker Compose | `db` |
| Database host in Kubernetes | `inventory-mysql` |

Database passwords are supplied through environment variables rather than being stored directly in the application source code.

## 5. REST API

The application exposes REST endpoints using Gin.

The main API prefix is `/api/v1`.

### Employee Endpoints

| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/v1/users` | Create an employee |
| GET | `/api/v1/users` | List employees |
| GET | `/api/v1/users/:id` | Get employee details |
| PUT | `/api/v1/users/:id` | Update employee information |
| DELETE | `/api/v1/users/:id` | Delete an employee |

Employee CRUD operations are also available through the unversioned `/users` routes.

### Asset Endpoints

| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/v1/assets` | Register an asset |
| GET | `/api/v1/assets` | List assets |
| GET | `/api/v1/assets/:id` | Get asset details |
| PUT | `/api/v1/assets/:id` | Update an asset |
| DELETE | `/api/v1/assets/:id` | Delete an asset |
| PATCH | `/api/v1/assets/:id/assign` | Assign an asset |
| PATCH | `/api/v1/assets/:id/unassign` | Return an asset |

### Dashboard and Health Endpoints

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/v1/dashboard/stats` | Dashboard statistics |
| GET | `/health` | Application and database health |
| GET | `/health/live` | HTTP server liveness |

The `/health` endpoint checks whether the application can communicate with the MySQL database.

The `/health/live` endpoint returns a response when the HTTP server is functioning.

### Example Asset Request

```json
{
  "asset_tag": "AST-1001",
  "asset_name": "Dell Latitude 5440",
  "category": "Laptop",
  "manufacturer": "Dell",
  "model": "Latitude 5440",
  "serial_number": "SN-1001",
  "location": "Leipzig",
  "purchase_date": "2025-01-15"
}
```

## 6. Docker Containerization

Docker is used to package the application and its dependencies into a container image.

### Dockerfile

A multi-stage Docker build is used to keep the final image small and reduce unnecessary runtime dependencies.

The first stage uses:

`golang:1.25-bookworm`

This stage downloads Go modules and compiles the application into a Linux executable.

Static compilation is configured using `CGO_ENABLED=0`.

The runtime stage uses:

`gcr.io/distroless/static-debian12:nonroot`

The Distroless image contains only the components needed to run the compiled executable. The application runs as a non-root user and listens on port 8080.

### Build the Docker Image

Open PowerShell in the project directory:

```powershell
cd "D:\DevOps Final"
docker build -t it-asset-inventory:v2 .
```

Verify the image:

```powershell
docker images it-asset-inventory
```

### Docker Compose

Docker Compose is used to run the Go application and MySQL together.

Two services are defined:

**app:** Runs the Go Gin application on port 8080.

**db:** Runs MySQL 8.0 and stores application data.

Both services communicate over the named Docker network `inventory_network`.

A named volume, `mysql_data`, stores MySQL data so records remain available after containers are recreated.

### Environment Configuration

Create a `.env` file in the project directory with the following variables:

```env
MYSQL_ROOT_PASSWORD=replace-with-a-strong-root-password
MYSQL_PASSWORD=replace-with-a-strong-app-password
```

These are placeholders. Actual passwords should not be committed to GitHub.

### Start Docker Compose

```powershell
docker compose up -d
```

Check the containers:

```powershell
docker compose ps
```

Both the application and database containers should report a healthy status after initialization.

Open the application in a browser:

`http://localhost:8080`

### Health Check

The Docker application health check invokes the application's health-check command, which requests the `/health` endpoint.

The MySQL service has its own health check, and the Go application is configured to start after the database becomes healthy.

### Test Data Persistence

Database persistence was tested by creating employee and asset records, recreating the Compose containers, and checking that the records were still available.

The records remained in the MySQL volume.

To stop the environment without deleting the database volume:

```powershell
docker compose down
```

## 7. Kubernetes Deployment

The application was deployed locally using Minikube.

Kubernetes manages the application and database containers and helps maintain the configured number of application replicas.

### Start Minikube

The local cluster was started with the Docker driver:

```powershell
minikube start --driver=docker --cpus=4 --memory=4096
```

Verify the cluster:

```powershell
minikube status
kubectl get nodes
```

The Minikube node should report `Ready`.

### Load the Application Image

The application image was built locally and loaded into Minikube:

```powershell
minikube image load it-asset-inventory:v2
```

Verify:

```powershell
minikube image ls
```

### Kubernetes Configuration Files

Four YAML files are used:

| File | Purpose |
|---|---|
| `k8s/deployment.yaml` | Go application Deployment |
| `k8s/service.yaml` | Go application Service |
| `k8s/mysql-deployment.yaml` | MySQL Deployment and persistent storage |
| `k8s/mysql-service.yaml` | Internal MySQL Service |

### Database Secret

Database credentials are stored in a Kubernetes Secret.

For the local practice cluster, the Secret was created using environment values from the local `.env` file:

```powershell
kubectl create secret generic inventory-db-secret --from-env-file
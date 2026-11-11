# IT Asset Inventory — DevOps Project

This project uses a small IT asset inventory web application to demonstrate a practical DevOps workflow: building a container image, running services with Docker Compose, deploying to Kubernetes, and testing performance and recovery. The application uses Go (Gin), MySQL 8.0, and a basic HTML/CSS/JavaScript frontend. It provides asset and employee management through a REST API.

## Technology stack

| Area | Tools |
| --- | --- |
| Application | Go, Gin, HTML, CSS, JavaScript |
| Database | MySQL 8.0 |
| Containers | Docker, multi-stage Dockerfile, Docker Compose |
| Orchestration | Kubernetes, Minikube, kubectl |
| Performance testing | Grafana k6 |
| Monitoring | Kubernetes Metrics Server |
| Version control | Git, GitHub |

## Project Structure

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
## 1. Docker image

The Dockerfile uses two stages. `golang:1.25-bookworm` compiles the Go application with `CGO_ENABLED=0`, and `gcr.io/distroless/static-debian12:nonroot` runs the resulting binary. The runtime container uses a non-root user and exposes port `8080`.

Build the image from the project directory:

```powershell
docker build -t it-asset-inventory:v2 .
docker images it-asset-inventory
```

## 2. Docker Compose

Docker Compose runs the Go application and MySQL 8.0 together. The services communicate on `inventory_network`, while the named volume `mysql_data` keeps database records when the containers are recreated. Health checks are defined for both services, and the application depends on the database becoming healthy.

Create a local `.env` file in the project root:

```dotenv
MYSQL_ROOT_PASSWORD=replace-with-a-strong-root-password
MYSQL_PASSWORD=replace-with-a-strong-app-password
```

These values are examples. The real `.env` file is excluded from Git and must not be committed.

Start and check the services:

```powershell
docker compose up -d
docker compose ps
Invoke-RestMethod http://127.0.0.1:8080/health
```

The application is available at `http://localhost:8080`. A healthy response from `/health` confirms that the application can connect to MySQL.

Stop the containers without removing the database volume:

```powershell
docker compose down
```

Database persistence was verified by creating records, restarting the Compose environment, and confirming that the records remained available.

## 3. Kubernetes deployment

The application and MySQL were deployed on a local Minikube cluster. The repository contains Deployments and Services for both components. Kubernetes uses a Secret for database credentials and a PersistentVolumeClaim for MySQL storage.

Start Minikube and load the local application image:

```powershell
minikube start --driver=docker
minikube image load it-asset-inventory:v2
kubectl get nodes
```

Create the database Secret from the local `.env` file:

```powershell
kubectl create secret generic inventory-db-secret --from-env-file=.env
```

Apply the Kubernetes manifests:

```powershell
kubectl apply -f k8s/mysql-deployment.yaml
kubectl apply -f k8s/mysql-service.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

Check the deployment:

```powershell
kubectl get pods
kubectl get deployments
kubectl get services
kubectl get pvc
```

Forward the application Service to the local computer:

```powershell
kubectl port-forward service/inventory-app 8081:8080
```

In a second terminal, check connectivity:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/health
```

The verified health response reported `status: ok` and `database: connected`. The application can be opened at `http://localhost:8081` while port forwarding remains active.

### Kubernetes resources and health checks

| Resource | Go application | MySQL |
| --- | ---: | ---: |
| CPU limit | 500m | 500m |
| Memory limit | 256 MiB | 768 MiB |

The application requests `100m` CPU and `128 MiB` memory. Its Deployment uses readiness and liveness probes on `/health`. The MySQL and Go application workloads run as separate pods, and the database stores its data on persistent storage.

A `ClusterIP` Service was used because the deployment runs inside a local Minikube cluster. For access from the host computer, `kubectl port-forward` exposes the service on port `8081`. A public `NodePort` or `LoadBalancer` was not needed for this test setup.

## 4. Functional verification

Before load testing, the application was checked through the web interface and API. The tests covered creating and listing employees and assets, editing an asset, deleting an asset, and checking database connectivity. In one update test, a Lenovo asset's location was changed from Leipzig to Berlin; an Acer asset was removed in a delete test.

The main endpoints used for verification were:

| Endpoint | Purpose |
| --- | --- |
| `GET /health` | Application and database check |
| `GET /api/v1/assets` | List assets; k6 test target |
| `/api/v1/users` | Employee records |
| `/api/v1/assets` | Asset records |

## 5. Load testing with k6

The k6 script, `inventory-load-test.js`, sends requests to `GET http://127.0.0.1:8081/api/v1/assets` through Kubernetes port forwarding. It uses a ramping-VUs scenario that reaches 500 virtual users and keeps that load for 60 seconds.

| Stage | Duration | Target VUs |
| --- | ---: | ---: |
| Ramp-up | 20 s | 100 |
| Ramp-up | 20 s | 300 |
| Ramp-up | 20 s | 500 |
| Sustained load | 60 s | 500 |
| Ramp-down | 10 s | 0 |

Keep port forwarding running, then execute:

```powershell
k6 run --summary-export=sustained-load-results.json inventory-load-test.js
```

The script checks for HTTP `200` responses. Its thresholds include a p95 response time below `500 ms` and an HTTP failure rate below `5%`.

### Final test results

| Metric | Result |
| --- | ---: |
| Total requests | 45,565 |
| Peak virtual users | 500 |
| Sustained peak load | 60 s |
| Average throughput | 348.09 requests/s |
| Average response time | 3.11 ms |
| p90 response time | 5.47 ms |
| p95 response time | 6.41 ms |
| Maximum response time | 13.50 ms |
| HTTP failure rate | 0% |

All requests in the final run succeeded, and the configured thresholds passed. The results represent the read-only asset-listing endpoint on the local Minikube setup; they should not be treated as production benchmarks or as performance results for every API operation.

The final summary is saved in `sustained-load-results.json`. Detailed raw time-series output was retained locally rather than committed because of its size.

## 6. CPU and memory monitoring

Metrics Server was enabled in Minikube to observe pod resource usage:

```powershell
minikube addons enable metrics-server
kubectl top pods
```

Measurements collected during the load test are included in `pod-metrics.csv`.

| Observed peak | Go application | MySQL |
| --- | ---: | ---: |
| CPU | 166m | 139m |
| Memory | 31 MiB | 455 MiB |

Both workloads remained below their configured CPU and memory limits during the monitored period. These are sampled measurements, not a continuous trace of every moment of execution.

The application uses Go’s standard `net/http` server. No fixed request-worker pool was configured: incoming requests are handled concurrently using goroutines scheduled by the Go runtime. During the 500-VU test, the measured CPU and memory usage remained below the pod limits, so the available evidence does not indicate resource saturation.

The test completed 45,565 HTTP requests with no HTTP failures, and the p95 response time was 6.41 ms. This supports the conclusion that the local deployment handled the tested read-only workload successfully. The figures do not establish performance under write-heavy workloads or in a production cluster.

## 7. Failure and recovery tests

Two recovery checks were performed:

1. **Pod replacement:** The application pod was deleted. Its Deployment created a replacement, which returned to the Running state.
2. **Container restart:** The running application container was stopped through the Minikube container runtime using `crictl`. Kubernetes restarted the container in the same pod.

For the container restart test, the restart count increased from `0` to `1`. The stopped container had exit code `137`. After recovery, the application passed its health check and connected to MySQL successfully.

Useful inspection commands:

```powershell
kubectl get pods
kubectl describe pod <application-pod-name>
kubectl logs deployment/inventory-app
```

## 8. Assumptions and limitations

- This setup was tested locally on Windows using Docker Desktop, PowerShell and Minikube. The application listens on port `8080` and the Kubernetes port-forward uses local port `8081`.
- The Docker build uses Go `1.25`; MySQL `8.0` is the database engine. The database and its tables are initialized at startup, with configuration passed through environment variables.
- The Docker Compose and Kubernetes environments use separate MySQL databases; data is not synchronized between them.
- Database passwords belong in the local `.env` file and Kubernetes Secret, not in committed source files.
- Kubernetes port forwarding is used for local access; no public ingress or load balancer was configured.
- The application does not implement user authentication or role-based access control and is not intended to be exposed to an untrusted public network.

## Repository

https://github.com/parameshi7/it-asset-inventory-devops

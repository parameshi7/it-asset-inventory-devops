FROM golang:1.25-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -o /app/inventory-app .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/inventory-app /app/inventory-app

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/inventory-app"]

package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"it-asset-inventory/internal/database"
	"it-asset-inventory/internal/handlers"
)

//go:embed web/*
var webFiles embed.FS

func main() {
	// Docker health-check mode.
	// Runs without initializing the database or HTTP server.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := runHealthCheck(); err != nil {
			log.Printf("health check failed: %v", err)
			os.Exit(1)
		}

		os.Exit(0)
	}

	// Connect to MySQL database.
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	// Run database migrations.
	if err := database.Migrate(db); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	// Configure Gin.
	gin.SetMode(env("GIN_MODE", gin.ReleaseMode))

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	api := handlers.New(db)

	// Health endpoints.
	r.GET("/health", api.Health)

	r.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "alive",
		})
	})

	// Original User CRUD routes for compatibility.
	r.POST("/users", api.CreateUser)
	r.GET("/users", api.ListUsers)
	r.GET("/users/:id", api.GetUser)
	r.PUT("/users/:id", api.UpdateUser)
	r.DELETE("/users/:id", api.DeleteUser)

	// Version 1 API routes.
	v1 := r.Group("/api/v1")
	{
		// Dashboard.
		v1.GET("/dashboard/stats", api.DashboardStats)

		// User management.
		v1.POST("/users", api.CreateUser)
		v1.GET("/users", api.ListUsers)
		v1.GET("/users/:id", api.GetUser)
		v1.PUT("/users/:id", api.UpdateUser)
		v1.DELETE("/users/:id", api.DeleteUser)

		// Asset management.
		v1.POST("/assets", api.CreateAsset)
		v1.GET("/assets", api.ListAssets)
		v1.GET("/assets/:id", api.GetAsset)
		v1.PUT("/assets/:id", api.UpdateAsset)
		v1.DELETE("/assets/:id", api.DeleteAsset)

		// Asset assignment.
		v1.PATCH("/assets/:id/assign", api.AssignAsset)
		v1.PATCH("/assets/:id/unassign", api.UnassignAsset)
	}

	// Serve embedded frontend files directly.
	r.GET("/", func(c *gin.Context) {
		content, err := webFiles.ReadFile("web/index.html")
		if err != nil {
			log.Printf("failed to load index.html: %v", err)
			c.String(
				http.StatusInternalServerError,
				"Unable to load frontend",
			)
			return
		}

		c.Data(
			http.StatusOK,
			"text/html; charset=utf-8",
			content,
		)
	})

	// Serve CSS.
	r.GET("/app.css", func(c *gin.Context) {
		content, err := webFiles.ReadFile("web/app.css")
		if err != nil {
			log.Printf("failed to load app.css: %v", err)
			c.String(
				http.StatusInternalServerError,
				"Unable to load CSS",
			)
			return
		}

		c.Data(
			http.StatusOK,
			"text/css; charset=utf-8",
			content,
		)
	})

	// Serve JavaScript.
	r.GET("/app.js", func(c *gin.Context) {
		content, err := webFiles.ReadFile("web/app.js")
		if err != nil {
			log.Printf("failed to load app.js: %v", err)
			c.String(
				http.StatusInternalServerError,
				"Unable to load JavaScript",
			)
			return
		}

		c.Data(
			http.StatusOK,
			"application/javascript; charset=utf-8",
			content,
		)
	})

	// HTTP server configuration.
	port := env("PORT", "8080")

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start HTTP server.
	go func() {
		log.Printf(
			"IT Asset Inventory listening on :%s",
			port,
		)

		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for termination signal.
	stop := make(chan os.Signal, 1)
	signal.Notify(
		stop,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	<-stop

	log.Println("Shutting down server...")

	// Graceful shutdown.
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf(
			"graceful shutdown failed: %v",
			err,
		)
	}

	log.Println("Server stopped")
}

// Read environment variable or return fallback value.
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

// Docker health-check function.
// Checks the application's /health endpoint.
// Returns an error if the server is unreachable
// or the endpoint does not return HTTP 200.
func runHealthCheck() error {
	port := env("PORT", "8080")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/health",
		port,
	)

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf(
			"health endpoint request failed: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"health endpoint returned HTTP %d",
			response.StatusCode,
		)
	}

	return nil
}

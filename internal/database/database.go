package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func Connect() (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&timeout=5s&readTimeout=10s&writeTimeout=10s",
		env("DB_USER", "appuser"), env("DB_PASSWORD", "apppass"),
		env("DB_HOST", "127.0.0.1"), env("DB_PORT", "3306"), env("DB_NAME", "it_asset_inventory"))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(intEnv("DB_MAX_OPEN_CONNS", 25))
	db.SetMaxIdleConns(intEnv("DB_MAX_IDLE_CONNS", 10))
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	attempts := intEnv("DB_CONNECT_RETRIES", 15)
	var lastErr error
	for i := 1; i <= attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		lastErr = db.PingContext(ctx)
		cancel()
		if lastErr == nil {
			return db, nil
		}
		log.Printf("waiting for MySQL (%d/%d): %v", i, attempts, lastErr)
		time.Sleep(2 * time.Second)
	}
	_ = db.Close()
	return nil, fmt.Errorf("MySQL did not become ready: %w", lastErr)
}

func Migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			employee_id VARCHAR(50) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			department VARCHAR(100) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS assets (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			asset_tag VARCHAR(50) NOT NULL UNIQUE,
			asset_name VARCHAR(255) NOT NULL,
			category VARCHAR(100) NOT NULL,
			manufacturer VARCHAR(100) NOT NULL DEFAULT '',
			model VARCHAR(100) NOT NULL DEFAULT '',
			serial_number VARCHAR(100) NULL UNIQUE,
			status ENUM('available','assigned','maintenance','retired') NOT NULL DEFAULT 'available',
			assigned_user_id BIGINT NULL,
			location VARCHAR(100) NOT NULL DEFAULT '',
			purchase_date DATE NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			CONSTRAINT fk_assets_user FOREIGN KEY (assigned_user_id) REFERENCES users(id) ON DELETE RESTRICT ON UPDATE CASCADE,
			INDEX idx_assets_status (status),
			INDEX idx_assets_assigned_user (assigned_user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

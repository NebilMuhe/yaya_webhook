package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/spf13/viper"
	_ "github.com/mattn/go-sqlite3"
)

type yaya struct {
	SecretKey string
	log       *slog.Logger
}

type Handler interface {
	HealthCheckHandler(w http.ResponseWriter, r *http.Request)
	YayayWebhookHandler(w http.ResponseWriter, r *http.Request)
	GetWebhookHandler(w http.ResponseWriter, r *http.Request)
	DebugWebhooksHandler(w http.ResponseWriter, r *http.Request)
}

func NewHandler(secretKey string, log *slog.Logger) Handler {
	return &yaya{
		SecretKey: secretKey,
		log:       log,
	}
}

func (y *yaya) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if err := json.NewEncoder(w).Encode(Response{
		StatusCode: http.StatusOK,
		Message:    "Server is up and running",
	}); err != nil {

	}
}

func (y *yaya) YayayWebhookHandler(w http.ResponseWriter, r *http.Request) {
	var req YayaWebhook

	signature := r.Header.Get("YAYA-SIGNATURE")
	if signature == "" {
		y.log.ErrorContext(r.Context(), "Signature is missing")
		json.NewEncoder(w).Encode(Response{
			StatusCode: http.StatusBadRequest,
			Error:      "signature is missing",
		})
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		y.log.ErrorContext(r.Context(), "Error decoding request", "error", err)
		json.NewEncoder(w).Encode(Response{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid data",
		})
		return
	}

	if isValid := y.validateTimestamp(req.TimeStamp, 5*time.Minute); !isValid {
		y.log.ErrorContext(r.Context(), "Invalid timestamp")
		json.NewEncoder(w).Encode(Response{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid timestamp",
		})
		return
	}

	if isValid := y.verifySignature(signature, req); !isValid {
		y.log.ErrorContext(r.Context(), "Invalid signature")
		json.NewEncoder(w).Encode(Response{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid signature",
		})
		return
	}

	go func() {
		// Process the database operation here
		if err := y.saveWebhookToDatabase(req); err != nil {
			y.log.ErrorContext(r.Context(), "Failed to save webhook to database", "error", err, "webhook_id", req.ID)
		} else {
			y.log.InfoContext(r.Context(), "Webhook saved to database successfully", "webhook_id", req.ID)
		}
	}()

	json.NewEncoder(w).Encode(Response{
		StatusCode: http.StatusOK,
		Message:    "Webhook received successfully",
	})
}

func (y *yaya) verifySignature(signature string, payload YayaWebhook) bool {
	// Debug logging
	y.log.InfoContext(context.Background(), "Verifying signature",
		"received_signature", signature,
		"payload_id", payload.ID)

	signedPayload := y.generateSignature(y.SecretKey, payload)

	// Debug logging
	y.log.InfoContext(context.Background(), "Signature comparison",
		"generated_signature", signedPayload,
		"received_signature", signature,
		"match", signature == signedPayload)

	return signature == signedPayload
}

func (y *yaya) generateSignature(secretKey string, payload YayaWebhook) string {
	// Concatenate all values
	signedPayload := fmt.Sprintf("%s%s%s%d%d%s%s%s%s",
		payload.ID,
		payload.Amount,
		payload.Currency,
		payload.CreatedAtTime,
		payload.TimeStamp,
		payload.Cause,
		payload.FullName,
		payload.AccountName,
		payload.InvoiceURL,
	)

	// Debug logging
	y.log.InfoContext(context.Background(), "Generating signature",
		"concatenated_string", signedPayload,
		"secret_key_length", len(secretKey))

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(signedPayload))
	result := hex.EncodeToString(h.Sum(nil))

	// Debug logging
	y.log.InfoContext(context.Background(), "Signature generated",
		"result", result)

	return result
}

func (y *yaya) validateTimestamp(timestamp int64, tolerance time.Duration) bool {
	now := time.Now().Unix()
	diff := now - timestamp
	y.log.InfoContext(context.Background(), "Validating timestamp",
		"timestamp", timestamp,
		"now", now,
		"tolerance", tolerance.Seconds(),
		"diff", diff)
	return diff >= 0 && diff <= int64(tolerance.Seconds())
}

func (y *yaya) saveWebhookToDatabase(webhook YayaWebhook) error {
	// Use SQLite file database for persistence
	dbPath := viper.GetString("database.file")
	if dbPath == "" {
		dbPath = "./yaya_webhooks.db" // default fallback
	}
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create the webhooks table if it doesn't exist
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			amount TEXT,
			currency TEXT,
			created_at_time INTEGER,
			timestamp INTEGER,
			cause TEXT,
			full_name TEXT,
			account_name TEXT,
			invoice_url TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Insert webhook data
	query := `
		INSERT INTO webhooks (
			id, amount, currency, created_at_time, timestamp, 
			cause, full_name, account_name, invoice_url, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			amount = excluded.amount,
			currency = excluded.currency,
			created_at_time = excluded.created_at_time,
			timestamp = excluded.timestamp,
			cause = excluded.cause,
			full_name = excluded.full_name,
			account_name = excluded.account_name,
			invoice_url = excluded.invoice_url,
			updated_at = excluded.updated_at
	`

	now := time.Now()
	_, err = db.Exec(query,
		webhook.ID,
		webhook.Amount.String(),
		webhook.Currency,
		webhook.CreatedAtTime,
		webhook.TimeStamp,
		webhook.Cause,
		webhook.FullName,
		webhook.AccountName,
		webhook.InvoiceURL,
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to insert webhook: %w", err)
	}

	return nil
}

// GetWebhookHandler retrieves a webhook by ID from the URL path
func (y *yaya) GetWebhookHandler(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	path := r.URL.Path
	webhookID := path[len("/webhook/"):]
	
	if webhookID == "" {
		json.NewEncoder(w).Encode(Response{
			StatusCode: http.StatusBadRequest,
			Error:      "webhook ID is required",
		})
		return
	}

	// Use SQLite file database for persistence
	dbPath := viper.GetString("database.file")
	if dbPath == "" {
		dbPath = "./yaya_webhooks.db" // default fallback
	}
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		y.log.ErrorContext(r.Context(), "Failed to connect to database", "error", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Create the webhooks table if it doesn't exist
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			amount TEXT,
			currency TEXT,
			created_at_time INTEGER,
			timestamp INTEGER,
			cause TEXT,
			full_name TEXT,
			account_name TEXT,
			invoice_url TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		y.log.ErrorContext(r.Context(), "Failed to create table", "error", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Query webhook by ID
	query := `SELECT id, amount, currency, created_at_time, timestamp, cause, full_name, account_name, invoice_url FROM webhooks WHERE id = ?`
	
	var id, amount, currency, cause, fullName, accountName, invoiceURL string
	var createdAtTime, timestamp int64
	
	err = db.QueryRow(query, webhookID).Scan(
		&id,
		&amount,
		&currency,
		&createdAtTime,
		&timestamp,
		&cause,
		&fullName,
		&accountName,
		&invoiceURL,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			json.NewEncoder(w).Encode(Response{
				StatusCode: http.StatusNotFound,
				Error:      "webhook not found",
			})
			return
		}
		y.log.ErrorContext(r.Context(), "Failed to query webhook", "error", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	webhook := map[string]interface{}{
		"id":              id,
		"amount":          amount,
		"currency":        currency,
		"created_at_time": createdAtTime,
		"timestamp":       timestamp,
		"cause":           cause,
		"full_name":       fullName,
		"account_name":    accountName,
		"invoice_url":     invoiceURL,
	}

	// Return the webhook data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhook)
}

// DebugWebhooksHandler shows all stored webhooks for debugging
func (y *yaya) DebugWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	// Use SQLite file database for persistence
	dbPath := viper.GetString("database.file")
	if dbPath == "" {
		dbPath = "./yaya_webhooks.db" // default fallback
	}
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		y.log.ErrorContext(r.Context(), "Failed to connect to database", "error", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Create the webhooks table if it doesn't exist
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			amount TEXT,
			currency TEXT,
			created_at_time INTEGER,
			timestamp INTEGER,
			cause TEXT,
			full_name TEXT,
			account_name TEXT,
			invoice_url TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		y.log.ErrorContext(r.Context(), "Failed to create table", "error", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Query all webhooks
	rows, err := db.Query("SELECT id, amount, currency, created_at_time, timestamp, cause, full_name, account_name, invoice_url, created_at, updated_at FROM webhooks ORDER BY created_at DESC")
	if err != nil {
		y.log.ErrorContext(r.Context(), "Failed to query webhooks", "error", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var webhooks []map[string]interface{}
	for rows.Next() {
		var id, amount, currency, cause, fullName, accountName, invoiceURL, createdAt, updatedAt string
		var createdAtTime, timestamp int64

		err := rows.Scan(&id, &amount, &currency, &createdAtTime, &timestamp, &cause, &fullName, &accountName, &invoiceURL, &createdAt, &updatedAt)
		if err != nil {
			y.log.ErrorContext(r.Context(), "Failed to scan row", "error", err)
			continue
		}

		webhook := map[string]interface{}{
			"id":              id,
			"amount":          amount,
			"currency":        currency,
			"created_at_time": createdAtTime,
			"timestamp":       timestamp,
			"cause":           cause,
			"full_name":       fullName,
			"account_name":    accountName,
			"invoice_url":     invoiceURL,
			"created_at":      createdAt,
			"updated_at":      updatedAt,
		}
		webhooks = append(webhooks, webhook)
	}

	// Return the data as JSON
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"count":    len(webhooks),
		"webhooks": webhooks,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		y.log.ErrorContext(r.Context(), "Failed to encode response", "error", err)
		http.Error(w, "Encoding error", http.StatusInternalServerError)
		return
	}
}

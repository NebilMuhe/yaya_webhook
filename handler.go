package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

type yaya struct {
	SecretKey string
	log       *slog.Logger
}

type Handler interface {
	HealthCheckHandler(w http.ResponseWriter, r *http.Request)
	YayayWebhookHandler(w http.ResponseWriter, r *http.Request)
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
	signedPayload := y.generateSignature(y.SecretKey, payload)
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

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(signedPayload))
	result := hex.EncodeToString(h.Sum(nil))

	return result
}

func (y *yaya) validateTimestamp(timestamp int64, tolerance time.Duration) bool {
	now := time.Now().Unix()
	diff := now - timestamp
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

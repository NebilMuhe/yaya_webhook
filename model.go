package main

import (
	"github.com/shopspring/decimal"
)

type Currency string

const (
	ETB Currency = "ETB"
)

type YayaWebhook struct {
	ID            string          `json:"id"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      Currency        `json:"currency"`
	CreatedAtTime int64           `json:"created_at_time"`
	TimeStamp     int64           `json:"timestamp"`
	Cause         string          `json:"cause"`
	FullName      string          `json:"full_name"`
	AccountName   string          `json:"account_name"`
	InvoiceURL    string          `json:"invoice_url"`
}

type Response struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
}

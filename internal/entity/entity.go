package entity

import (
	"github.com/dgrijalva/jwt-go/v4"
	"time"
)

type User struct {
	jwt.StandardClaims
	ID       int64  `json:"-"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Order struct {
	ID         string    `json:"number,omitempty"`
	UserID     int64     `json:"-"`
	Status     string    `json:"status"`
	Accrual    float32   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
	// for accrual system
	// OrderID string `json:"order,omitempty"`
}

type Balance struct {
	UserID    int64   `json:"-"`
	Current   float32 `json:"current"`
	Withdrawn float32 `json:"withdrawn"`
}

type Withdrawals struct {
	UserID      int64     `json:"-"`
	Sum         float32   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
	OrderID     string    `json:"order"`
}

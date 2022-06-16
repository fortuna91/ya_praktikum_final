package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/dgrijalva/jwt-go/v4"
	"log"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
)

type UserData struct {
	jwt.StandardClaims
	ID       int64  `json:"-"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type OrderData struct {
	ID         string    `json:"number,omitempty"`
	UserID     int64     `json:"-"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
	// for accrual system
	// OrderID string `json:"order,omitempty"`
}

type BalanceData struct {
	UserID    int64   `json:"-"`
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type WithdrawalsData struct {
	UserID      int64     `json:"-"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
	OrderID     string    `json:"order"`
}

type DBStorage struct {
	dbConnection *sql.DB
}

func New(dbAddress string) *DBStorage {
	dbConn, err := sql.Open("pgx", dbAddress)
	if err != nil {
		panic(err)
	}
	return &DBStorage{
		dbConnection: dbConn,
	}
}

func (db *DBStorage) Create(ctx context.Context) {
	db.CreateUsers(ctx)
	db.CreateBalance(ctx)
	db.CreateOrders(ctx)
	db.CreateWithdrawals(ctx)
}

func (db *DBStorage) Close() {
	db.dbConnection.Close()
}

func (db *DBStorage) GetUser(ctx context.Context, login string) *UserData {
	user := UserData{}

	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Users WHERE login=$1", login).Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		log.Printf("User with login %s doesn't exist. %s\n", login, err)
		return nil
	}
	return &user
}

func (db *DBStorage) AddUser(ctx context.Context, login string, password string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Users (login, password) VALUES ($1, $2);",
		login, password)
	if err != nil {
		return fmt.Errorf("couldn't add user %s into DB: %s", login, err)
	}
	return nil
}

func (db *DBStorage) GetOrder(ctx context.Context, orderID string) *OrderData {
	order := OrderData{}
	var status sql.NullString
	var accrual sql.NullFloat64

	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Orders WHERE id=$1", orderID).
		Scan(&order.ID, &order.UserID, &status, &accrual, &order.UploadedAt)
	if err != nil {
		log.Printf("Order %s doesn't exist. %s\n", orderID, err)
		return nil
	}
	if status.Valid {
		order.Status = status.String
	}
	if accrual.Valid {
		order.Accrual = accrual.Float64
	}
	return &order
}

func (db *DBStorage) AddOrder(ctx context.Context, id string, userID int64, status string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Orders (id, user_id, status) VALUES ($1, $2, $3);",
		id, userID, status)
	if err != nil {
		return fmt.Errorf("couldn't add order %s into DB: %s", id, err)
	}
	fmt.Printf("Add order %s\n", id)
	return nil
}

func (db *DBStorage) UpdateOrder(ctx context.Context, id string, status string, accrual float64) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Orders (id, status, accrual) VALUES ($1, $2, $3, $4)"+
		"ON CONFLICT (id) DO UPDATE SET status = excluded.status, accrual = excluded.accrual;",
		id, status, accrual)
	if err != nil {
		return fmt.Errorf("couldn't update order %s into DB: %s", id, err)
	}
	fmt.Printf("Update order %s\n", id)
	return nil
}

func (db *DBStorage) GetOrders(ctx context.Context, userID int64) []OrderData {
	var orders []OrderData
	var status sql.NullString
	var accrual sql.NullFloat64

	rows, err := db.dbConnection.QueryContext(ctx, "SELECT * FROM Orders WHERE user_id=$1 ORDER BY uploaded_at", userID)
	if err != nil {
		log.Printf("Couldn't read orders for user. %s\n", err)
		return nil
	}

	for rows.Next() {
		order := OrderData{}
		err = rows.Scan(&order.ID, &order.UserID, &status, &accrual, &order.UploadedAt)
		if err != nil {
			log.Printf("Couldn't set order %s from DB: %s\n", order.ID, err)
			return nil
		}
		if status.Valid {
			order.Status = status.String
		}
		if accrual.Valid {
			order.Accrual = accrual.Float64
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil
	}

	return orders
}

func (db *DBStorage) UpdateBalance(ctx context.Context, userID int64, current float64, withdrawn float64) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Balances (user_id, current, withdrawn) VALUES ($1, $2, $3) ON CONFLICT (user_id) DO UPDATE SET current = excluded.current, withdrawn = excluded.withdrawn",
		userID, current, withdrawn)
	if err != nil {
		return err
	}
	log.Printf("Update balance for user %d\n", userID)
	return nil
}

func (db *DBStorage) GetBalance(ctx context.Context, userID int64) *BalanceData {
	balance := BalanceData{}
	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Balances WHERE user_id=$1", userID).
		Scan(&balance.UserID, &balance.Current, &balance.Withdrawn)
	if err != nil {
		log.Printf("There is no balance data for user %d: %v\n", userID, err)
		return nil
	}
	return &balance
}

func (db *DBStorage) AddWithdrawal(ctx context.Context, userID int64, sum float64, orderID string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Withdrawals (user_id, sum, order_id VALUES ($1, $2, $3);",
		userID, sum, orderID)
	if err != nil {
		return err
	}
	log.Printf("Add withdrawal for user %d\n", userID)
	return nil
}

func (db *DBStorage) GetWithdrawals(ctx context.Context, userID int64) []WithdrawalsData {
	var withdrawals []WithdrawalsData

	rows, err := db.dbConnection.QueryContext(ctx, "SELECT * FROM Withdrawals WHERE user_id=$1 ORDER BY processed_at", userID)
	if err != nil {
		log.Printf("Couldn't read withdrawals for user. %s\n", err)
		return nil
	}

	for rows.Next() {
		withdrawal := WithdrawalsData{}
		err = rows.Scan(&withdrawal.UserID, &withdrawal.Sum, &withdrawal.ProcessedAt, &withdrawal.OrderID)
		if err != nil {
			log.Printf("Couldn't set withdrawal from DB: %v\n", err)
			return nil
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	if rows.Err() != nil {
		return nil
	}
	return withdrawals
}

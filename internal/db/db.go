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
	UserId     int64     `json:"-"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
	// for accrual system
	// OrderID string `json:"order,omitempty"`
}

type BalanceData struct {
	UserId    int64   `json:"-"`
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type WithdrawalsData struct {
	UserId      int64     `json:"-"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
	OrderId     string    `json:"order"`
}

type DbStorage struct {
	dbConnection *sql.DB
}

func New(dbAddress string) *DbStorage {
	dbConn, err := sql.Open("pgx", dbAddress)
	if err != nil {
		panic(err)
	}
	return &DbStorage{
		dbConnection: dbConn,
	}
}

func (db *DbStorage) Create(ctx context.Context) {
	db.CreateUsers(ctx)
	db.CreateBalance(ctx)
	db.CreateOrders(ctx)
	db.CreateWithdrawals(ctx)
}

func (db *DbStorage) Close() {
	db.dbConnection.Close()
}

func (db *DbStorage) GetUser(ctx context.Context, login string) *UserData {
	user := UserData{}

	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Users WHERE login=$1", login).Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		log.Printf("User with login %s doesn't exist. %s\n", login, err)
		return nil
	}
	return &user
}

func (db *DbStorage) AddUser(ctx context.Context, login string, password string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Users (login, password) VALUES ($1, $2);",
		login, password)
	if err != nil {
		return fmt.Errorf("couldn't add user %s into DB: %s", login, err)
	}
	return nil
}

func (db *DbStorage) GetOrder(ctx context.Context, orderId string) *OrderData {
	order := OrderData{}
	var status sql.NullString
	var accrual sql.NullFloat64

	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Orders WHERE id=$1", orderId).
		Scan(&order.ID, &order.UserId, &status, &accrual, &order.UploadedAt)
	if err != nil {
		log.Printf("Order %s doesn't exist. %s\n", orderId, err)
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

func (db *DbStorage) AddOrder(ctx context.Context, id string, userId int64, status string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Orders (id, user_id, status) VALUES ($1, $2, $3);",
		id, userId, status)
	if err != nil {
		return fmt.Errorf("couldn't add order %s into DB: %s", id, err)
	}
	fmt.Printf("Add order %s\n", id)
	return nil
}

func (db *DbStorage) UpdateOrder(ctx context.Context, id string, status string, accrual float64) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Orders (id, status, accrual) VALUES ($1, $2, $3, $4)"+
		"ON CONFLICT (id) DO UPDATE SET status = excluded.status, accrual = excluded.accrual;",
		id, status, accrual)
	if err != nil {
		return fmt.Errorf("couldn't update order %s into DB: %s", id, err)
	}
	fmt.Printf("Update order %s\n", id)
	return nil
}

func (db *DbStorage) GetOrders(ctx context.Context, userId int64) []OrderData {
	var orders []OrderData
	var status sql.NullString
	var accrual sql.NullFloat64

	rows, err := db.dbConnection.QueryContext(ctx, "SELECT * FROM Orders WHERE user_id=$1 ORDER BY uploaded_at", userId)
	if err != nil {
		log.Printf("Couldn't read orders for user. %s\n", err)
		return nil
	}

	for rows.Next() {
		order := OrderData{}
		err = rows.Scan(&order.ID, &order.UserId, &status, &accrual, &order.UploadedAt)
		if err != nil {
			log.Printf("Couldn't set order %d from DB: %s\n", order.ID, err)
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

	return orders
}

func (db *DbStorage) UpdateBalance(ctx context.Context, userId int64, current float64, withdrawn float64) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Balances (user_id, current, withdrawn) VALUES ($1, $2, $3) ON CONFLICT (user_id) DO UPDATE SET current = excluded.current, withdrawn = excluded.withdrawn",
		userId, current, withdrawn)
	if err != nil {
		return err
	}
	log.Printf("Update balance for user %d\n", userId)
	return nil
}

func (db *DbStorage) GetBalance(ctx context.Context, userId int64) *BalanceData {
	balance := BalanceData{}
	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Balances WHERE user_id=$1", userId).
		Scan(&balance.UserId, &balance.Current, &balance.Withdrawn)
	if err != nil {
		log.Printf("There is no balance data for user %d: %v\n", userId, err)
		return nil
	}
	return &balance
}

func (db *DbStorage) AddWithdrawal(ctx context.Context, userId int64, sum float64, orderID string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Withdrawals (user_id, sum, order_id VALUES ($1, $2, $3);",
		userId, sum, orderID)
	if err != nil {
		return err
	}
	log.Printf("Add withdrawal for user %d\n", userId)
	return nil
}

func (db *DbStorage) GetWithdrawals(ctx context.Context, userId int64) []WithdrawalsData {
	var withdrawals []WithdrawalsData

	rows, err := db.dbConnection.QueryContext(ctx, "SELECT * FROM Withdrawals WHERE user_id=$1 ORDER BY processed_at", userId)
	if err != nil {
		log.Printf("Couldn't read withdrawals for user. %s\n", err)
		return nil
	}

	for rows.Next() {
		withdrawal := WithdrawalsData{}
		err = rows.Scan(&withdrawal.UserId, &withdrawal.Sum, &withdrawal.ProcessedAt, &withdrawal.OrderId)
		if err != nil {
			log.Printf("Couldn't set withdrawal from DB: %v\n", err)
			return nil
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	return withdrawals
}

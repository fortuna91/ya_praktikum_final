package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/fortuna91/ya_praktikum_final/internal/entity"
	_ "github.com/jackc/pgx/v4/stdlib"

	"github.com/rs/zerolog/log"
)

type DBStorage struct {
	dbConnection *sql.DB
}

type ErrorDB struct {
	Err error
}

func (err *ErrorDB) Error() string {
	return fmt.Sprintf("%v", err.Err)
}

func New(dbAddress string) (*DBStorage, error) {
	dbConn, err := sql.Open("pgx", dbAddress)
	if err != nil {
		return nil, &ErrorDB{Err: err}
	}
	return &DBStorage{
		dbConnection: dbConn,
	}, nil
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

func (db *DBStorage) GetUser(ctx context.Context, login string) *entity.User {
	user := entity.User{}

	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Users WHERE login=$1", login).Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		log.Warn().Msgf("User with login %s doesn't exist. %s\n", login, err)
		return nil
	}
	return &user
}

func (db *DBStorage) AddUser(ctx context.Context, login string, password string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Users (login, password) VALUES ($1, $2);",
		login, password)
	if err != nil {
		return &ErrorDB{Err: fmt.Errorf("couldn't add user %s into DB: %s", login, err)}
	}
	return nil
}

func (db *DBStorage) GetOrder(ctx context.Context, orderID string) *entity.Order {
	order := entity.Order{}
	var status sql.NullString
	var accrual sql.NullFloat64

	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Orders WHERE id=$1", orderID).
		Scan(&order.ID, &order.UserID, &status, &accrual, &order.UploadedAt)
	if err != nil {
		log.Warn().Msgf("Order %s doesn't exist. %s\n", orderID, err)
		return nil
	}
	if status.Valid {
		order.Status = status.String
	}
	if accrual.Valid {
		order.Accrual = float32(accrual.Float64)
	}
	return &order
}

func (db *DBStorage) AddOrder(ctx context.Context, id string, userID int64, status string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Orders (id, user_id, status) VALUES ($1, $2, $3);",
		id, userID, status)
	if err != nil {
		return &ErrorDB{Err: fmt.Errorf("couldn't add order %s into DB: %s", id, err)}
	}
	log.Info().Msgf("Add order %s\n", id)
	return nil
}

func (db *DBStorage) UpdateOrder(ctx context.Context, id string, userID int64, status string, accrual float32) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Orders (id, user_id, status, accrual) VALUES ($1, $2, $3, $4)"+
		"ON CONFLICT (id) DO UPDATE SET status = excluded.status, accrual = excluded.accrual;",
		id, userID, status, accrual)
	if err != nil {
		return &ErrorDB{Err: fmt.Errorf("couldn't update order %s into DB: %s", id, err)}
	}
	log.Info().Msgf("Update order %s\n", id)
	return nil
}

func (db *DBStorage) GetOrders(ctx context.Context, userID int64) ([]entity.Order, error) {
	var orders []entity.Order
	var status sql.NullString
	var accrual sql.NullFloat64

	rows, err := db.dbConnection.QueryContext(ctx, "SELECT * FROM Orders WHERE user_id=$1 ORDER BY uploaded_at", userID)
	if err != nil {
		log.Error().Msgf("Couldn't read orders for user. %s\n", err)
		return nil, err
	}

	for rows.Next() {
		order := entity.Order{}
		err = rows.Scan(&order.ID, &order.UserID, &status, &accrual, &order.UploadedAt)
		if err != nil {
			log.Error().Msgf("Couldn't set order %s from DB: %s\n", order.ID, err)
			return nil, &ErrorDB{Err: err}
		}
		if status.Valid {
			order.Status = status.String
		}
		if accrual.Valid {
			order.Accrual = float32(accrual.Float64)
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil, &ErrorDB{Err: err}
	}

	return orders, nil
}

func (db *DBStorage) AddBalance(ctx context.Context, userID int64) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Balances (user_id) VALUES ($1);", userID)
	if err != nil {
		return &ErrorDB{Err: err}
	}
	log.Info().Msgf("Add balance for user %d\n", userID)
	return nil
}

func (db *DBStorage) UpdateBalance(ctx context.Context, userID int64, accrual float32) error {
	_, err := db.dbConnection.ExecContext(ctx, "UPDATE Balances SET current = current + $1 WHERE user_id=$2",
		accrual, userID)
	if err != nil {
		return &ErrorDB{Err: err}
	}
	log.Info().Msgf("Update balance for user %d. Add to current = %f\n", userID, accrual)
	return nil
}

func (db *DBStorage) Withdraw(ctx context.Context, userID int64, sum float32) error {
	_, err := db.dbConnection.ExecContext(ctx, "UPDATE Balances SET current = current - $1, withdrawn = withdrawn + $1 WHERE user_id=$2",
		sum, userID)
	if err != nil {
		return &ErrorDB{Err: err}
	}
	log.Info().Msgf("Update balance for user %d. Sum = %f\n", userID, sum)
	return nil
}

func (db *DBStorage) GetBalance(ctx context.Context, userID int64) *entity.Balance {
	balance := entity.Balance{}
	err := db.dbConnection.QueryRowContext(ctx, "SELECT * FROM Balances WHERE user_id=$1", userID).
		Scan(&balance.UserID, &balance.Current, &balance.Withdrawn)
	if err != nil {
		log.Warn().Msgf("There is no balance data for user %d: %v\n", userID, err)
		return nil
	}
	log.Info().Msgf("Get balance for user %d: %v\n", userID, balance)
	return &balance
}

func (db *DBStorage) AddWithdrawal(ctx context.Context, userID int64, sum float32, orderID string) error {
	_, err := db.dbConnection.ExecContext(ctx, "INSERT INTO Withdrawals (user_id, sum, order_id) VALUES ($1, $2, $3);",
		userID, sum, orderID)
	if err != nil {
		return &ErrorDB{Err: err}
	}
	log.Info().Msgf("Add withdrawal for user %d\n", userID)
	return nil
}

func (db *DBStorage) GetWithdrawals(ctx context.Context, userID int64) ([]entity.Withdrawals, error) {
	var withdrawals []entity.Withdrawals

	rows, err := db.dbConnection.QueryContext(ctx, "SELECT * FROM Withdrawals WHERE user_id=$1 ORDER BY processed_at", userID)
	if err != nil {
		log.Error().Msgf("No withdrawals for user. %s\n", err)
		return nil, err
	}

	for rows.Next() {
		withdrawal := entity.Withdrawals{}
		err = rows.Scan(&withdrawal.UserID, &withdrawal.Sum, &withdrawal.ProcessedAt, &withdrawal.OrderID)
		if err != nil {
			log.Error().Msgf("Couldn't set withdrawal from DB: %v\n", err)
			return nil, &ErrorDB{Err: err}
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	if rows.Err() != nil {
		log.Error().Msgf("There is error while reading rows: %v\n", rows.Err())
		return nil, &ErrorDB{Err: rows.Err()}
	}
	return withdrawals, nil
}

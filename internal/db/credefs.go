package db

import "context"

func (db *DBStorage) CreateUsers(ctx context.Context) {
	query := "CREATE TABLE IF NOT EXISTS Users (" +
		"id SERIAL PRIMARY KEY," +
		"login varchar(50) NOT NULL," +
		"password varchar(250) NOT NULL," +
		"CONSTRAINT unique_login UNIQUE(login));"
	_, err := db.dbConnection.ExecContext(ctx, query)
	if err != nil {
		panic(err)
	}
}

func (db *DBStorage) CreateOrders(ctx context.Context) {
	query := "CREATE TABLE IF NOT EXISTS Orders (" +
		"id varchar(50) NOT NULL," +
		"user_id bigint NOT NULL," +
		"status varchar(50)," +
		"accrual real," +
		"uploaded_at timestamp DEFAULT current_timestamp," +
		"PRIMARY KEY(id)," +
		"CONSTRAINT fk_user " +
		"FOREIGN KEY(user_id) " +
		"REFERENCES Users(id) ON DELETE CASCADE);"
	_, err := db.dbConnection.ExecContext(ctx, query)
	if err != nil {
		panic(err)
	}
}

func (db *DBStorage) CreateBalance(ctx context.Context) {
	query := "CREATE TABLE IF NOT EXISTS Balances (" +
		"user_id bigint NOT NULL," +
		"current real DEFAULT 0," +
		"withdrawn real DEFAULT 0," +
		"PRIMARY KEY(user_id));"
	_, err := db.dbConnection.ExecContext(ctx, query)
	if err != nil {
		panic(err)
	}
}

func (db *DBStorage) CreateWithdrawals(ctx context.Context) {
	query := "CREATE TABLE IF NOT EXISTS Withdrawals (" +
		"user_id bigint NOT NULL," +
		"sum real," +
		"processed_at timestamp DEFAULT current_timestamp," +
		"order_id varchar(50) NOT NULL," +
		"CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES Users(id) ON DELETE CASCADE);"
	_, err := db.dbConnection.ExecContext(ctx, query)
	if err != nil {
		panic(err)
	}
}

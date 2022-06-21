package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/theplant/luhn"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/fortuna91/ya_praktikum_final/internal/accrual"
	"github.com/fortuna91/ya_praktikum_final/internal/auth"
	"github.com/fortuna91/ya_praktikum_final/internal/body"
	"github.com/fortuna91/ya_praktikum_final/internal/db"
	"github.com/fortuna91/ya_praktikum_final/internal/entity"
)

var HashKey string

var dbStorage *db.DBStorage
var ContextCancelTimeout time.Duration

const NewStatus = "NEW"

func PrepareDB(dbAddress string) error {
	ctx, cancel := context.WithTimeout(context.Background(), ContextCancelTimeout)
	defer cancel()
	if dbStorage != nil {
		return fmt.Errorf("db has already been initialized")
	}

	var err error
	dbStorage, err = db.New(dbAddress)
	if err != nil {
		return err
	}
	// error
	dbStorage.Create(ctx)
	return nil
}

func GetDB() *db.DBStorage {
	return dbStorage
}

func Register(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ContextCancelTimeout)
	defer cancel()
	r.Header.Add("Content-Type", "application/json; charset=utf-8")
	respBody := body.GetBody(r.Body)
	if respBody == nil {
		http.Error(w, "Couldn't read body", http.StatusInternalServerError)
		return
	}
	userRequest := entity.User{}
	if errJSON := json.Unmarshal(*respBody, &userRequest); errJSON != nil {
		http.Error(w, "Wrong request", http.StatusBadRequest)
		return
	}
	userDB := dbStorage.GetUser(ctx, userRequest.Login)
	if userDB != nil {
		http.Error(w, "Login exists", http.StatusConflict)
		return
	}
	if errAdd := dbStorage.AddUser(ctx, userRequest.Login, auth.CalcHash("someKey", userRequest.Password)); errAdd != nil {
		http.Error(w, errAdd.Error(), http.StatusInternalServerError)
		return
	}
	newUser := dbStorage.GetUser(ctx, userRequest.Login)
	if err := dbStorage.AddBalance(ctx, newUser.ID); err != nil {
		log.Error().Msgf("Couldn't add balance: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	signedToken, err := auth.SetToken(newUser)
	if err != nil {
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Authorization", "Bearer "+signedToken)
	w.WriteHeader(http.StatusOK)
}

func Login(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ContextCancelTimeout)
	defer cancel()
	r.Header.Add("Content-Type", "application/json; charset=utf-8")

	respBody := body.GetBody(r.Body)
	if respBody == nil {
		http.Error(w, "Couldn't read body", http.StatusInternalServerError)
		return
	}
	userRequest := entity.User{}
	if errJSON := json.Unmarshal(*respBody, &userRequest); errJSON != nil {
		http.Error(w, "Wrong request", http.StatusBadRequest)
		return
	}
	userDB := dbStorage.GetUser(ctx, userRequest.Login)
	if userDB == nil {
		http.Error(w, "Login doesn't exist", http.StatusUnauthorized)
		return
	}
	if auth.CalcHash(HashKey, userRequest.Password) != userDB.Password {
		http.Error(w, "Wrong password", http.StatusUnauthorized)
		return
	}

	signedToken, err := auth.SetToken(&userRequest)
	if err != nil {
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Authorization", "Bearer "+signedToken)
	w.WriteHeader(http.StatusOK)
}

func UploadOrder(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ContextCancelTimeout)
	defer cancel()
	r.Header.Add("Content-Type", "application/json; charset=utf-8")

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := dbStorage.GetUser(ctx, login)
	orderID := string(*body.GetBody(r.Body))
	intOrderID, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		http.Error(w, "wrong order format", http.StatusUnprocessableEntity)
		return
	}
	if !luhn.Valid(int(intOrderID)) {
		http.Error(w, "wrong order format, luhn", http.StatusUnprocessableEntity)
		return
	}
	orderDB := dbStorage.GetOrder(ctx, orderID)
	if orderDB != nil {
		if orderDB.UserID != user.ID {
			http.Error(w, "order belongs to another user", http.StatusConflict)
		} else {
			log.Warn().Msgf("Order %s exists\n", orderID)
			w.WriteHeader(http.StatusOK)
		}
		return
	}
	if errAdd := dbStorage.AddOrder(ctx, orderID, user.ID, NewStatus); errAdd != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	accrual.QueueCh <- *dbStorage.GetOrder(ctx, orderID)
	w.WriteHeader(http.StatusAccepted)
}

func GetOrders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ContextCancelTimeout)
	defer cancel()

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := dbStorage.GetUser(ctx, login)
	ordersDB := dbStorage.GetOrders(ctx, user.ID)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if ordersDB == nil {
		log.Warn().Msg("No orders for user")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	bodyResp, err := json.Marshal(ordersDB)
	if err != nil {
		log.Error().Msgf("Cannot convert Orders to JSON: %v", err)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, errBody := w.Write(bodyResp)
	if errBody != nil {
		log.Error().Msgf("Error sending the response: %v\n", errBody)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
}

func GetBalance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ContextCancelTimeout)
	defer cancel()

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := dbStorage.GetUser(ctx, login)
	balanceDB := dbStorage.GetBalance(ctx, user.ID)
	if balanceDB == nil {
		log.Warn().Msg("No balance for user")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	bodyResp, err := json.Marshal(balanceDB)
	if err != nil {
		log.Error().Msgf("Cannot convert Balance to JSON: %v", err)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, errBody := w.Write(bodyResp)
	if errBody != nil {
		log.Error().Msgf("Error sending the response: %v\n", errBody)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
}

func Withdraw(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ContextCancelTimeout)
	defer cancel()
	r.Header.Add("Content-Type", "application/json; charset=utf-8")

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := dbStorage.GetUser(ctx, login)

	respBody := body.GetBody(r.Body)
	if respBody == nil {
		http.Error(w, "Couldn't read body", http.StatusInternalServerError)
		return
	}
	withdrawalRequest := entity.Withdrawals{}
	if errJSON := json.Unmarshal(*respBody, &withdrawalRequest); errJSON != nil {
		http.Error(w, errJSON.Error(), http.StatusBadRequest) //"wrong request",
		return
	}
	currBalance := dbStorage.GetBalance(ctx, user.ID)
	if currBalance == nil {
		log.Warn().Msg("Couldn't find balance")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if currBalance.Current < withdrawalRequest.Sum {
		log.Error().Msgf("Not enough balance: %f < %f\n", currBalance.Current, withdrawalRequest.Sum)
		http.Error(w, "not enough balance", http.StatusPaymentRequired)
		return
	}
	intOrderID, err := strconv.Atoi(withdrawalRequest.OrderID)
	if err != nil {
		http.Error(w, "wrong order format", http.StatusUnprocessableEntity)
		return
	}
	if !luhn.Valid(intOrderID) {
		http.Error(w, "wrong order format", http.StatusUnprocessableEntity)
		return
	}
	if err := dbStorage.AddWithdrawal(ctx, user.ID, withdrawalRequest.Sum, withdrawalRequest.OrderID); err != nil {
		log.Error().Msgf("Couldn't add withdrawal %v\n", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := dbStorage.Withdraw(ctx, user.ID, withdrawalRequest.Sum); err != nil {
		log.Error().Msgf("Couldn't update balance %v\n", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ContextCancelTimeout)
	defer cancel()
	r.Header.Add("Content-Type", "application/json; charset=utf-8")

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := dbStorage.GetUser(ctx, login)
	withdrawalsDB := dbStorage.GetWithdrawals(ctx, user.ID)
	if withdrawalsDB == nil {
		log.Warn().Msg("No withdrawals for user")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	bodyResp, err := json.Marshal(withdrawalsDB)
	if err != nil {
		log.Error().Msgf("Cannot convert withdrawals to JSON: %v", err)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, errBody := w.Write(bodyResp)
	if errBody != nil {
		log.Error().Msgf("Error sending the response: %v\n", errBody)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
}

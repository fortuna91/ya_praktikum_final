package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fortuna91/ya_praktikum_final/internal/accrual"
	"github.com/fortuna91/ya_praktikum_final/internal/auth"
	"github.com/fortuna91/ya_praktikum_final/internal/db"
	"github.com/fortuna91/ya_praktikum_final/internal/utils"
	"github.com/theplant/luhn"
	"log"
	"net/http"
	"strconv"
	"time"
)

var DB *db.DbStorage
var Queue = accrual.QueueAccrualSystem{}

const NewStatus = "NEW"

func Register(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	respBody := utils.GetBody(r.Body)
	if respBody == nil {
		http.Error(w, "Couldn't read body", http.StatusInternalServerError)
		return
	}
	userRequest := db.UserData{}
	if errJson := json.Unmarshal(*respBody, &userRequest); errJson != nil {
		http.Error(w, "Wrong request", http.StatusBadRequest)
		return
	}
	userDB := DB.GetUser(ctx, userRequest.Login)
	if userDB != nil {
		http.Error(w, "Login exists", http.StatusConflict)
		return
	}
	if errAdd := DB.AddUser(ctx, userRequest.Login, auth.CalcHash("someKey", userRequest.Password)); errAdd != nil {
		http.Error(w, errAdd.Error(), http.StatusInternalServerError)
		return
	}
	newUser := DB.GetUser(ctx, userRequest.Login)
	if err := DB.UpdateBalance(ctx, newUser.ID, 0, 0); err != nil {
		log.Printf("Couldn't add balance: %v\n", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func Login(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	respBody := utils.GetBody(r.Body)
	if respBody == nil {
		http.Error(w, "Couldn't read body", http.StatusInternalServerError)
		return
	}
	userRequest := db.UserData{}
	if errJson := json.Unmarshal(*respBody, &userRequest); errJson != nil {
		http.Error(w, "Wrong request", http.StatusBadRequest)
		return
	}
	userDB := DB.GetUser(ctx, userRequest.Login)
	if userDB == nil {
		http.Error(w, "Login doesn't exist", http.StatusUnauthorized)
		return
	}
	// TODO decode password
	if auth.CalcHash("someKey", userRequest.Password) != userDB.Password {
		http.Error(w, "Wrong password", http.StatusUnauthorized)
		return
	}

	signedToken, err := auth.SetToken(&userRequest)
	if err != nil {
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	if _, errBody := w.Write([]byte(signedToken)); errBody != nil {
		log.Printf("Error sending the response: %v\n", errBody)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
	}
}

func UploadOrder(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := DB.GetUser(ctx, login)
	orderId := string(*utils.GetBody(r.Body))
	intOrderId, err := strconv.ParseInt(orderId, 10, 64)
	if err != nil {
		http.Error(w, "wrong order format", http.StatusUnprocessableEntity)
		return
	}
	if !luhn.Valid(int(intOrderId)) {
		http.Error(w, "wrong order format, luhn", http.StatusUnprocessableEntity)
		return
	}
	orderDB := DB.GetOrder(ctx, orderId)
	if orderDB != nil {
		if orderDB.UserId != user.ID {
			http.Error(w, "order belongs to another user", http.StatusConflict)
		} else {
			fmt.Printf("Order %s exists\n", orderId)
			w.WriteHeader(http.StatusOK)
		}
		return
	}
	if errAdd := DB.AddOrder(ctx, orderId, user.ID, NewStatus); errAdd != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	Queue.Append(*DB.GetOrder(ctx, orderId))
	w.WriteHeader(http.StatusAccepted)
}
func GetOrders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := DB.GetUser(ctx, login)
	ordersDB := DB.GetOrders(ctx, user.ID)
	if ordersDB == nil {
		log.Println("No orders for user")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	bodyResp, err := json.Marshal(ordersDB)
	if err != nil {
		log.Printf("Cannot convert Orders to JSON: %v", err)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, errBody := w.Write(bodyResp)
	if errBody != nil {
		log.Printf("Error sending the response: %v\n", errBody)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
}

func GetBalance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := DB.GetUser(ctx, login)
	balanceDB := DB.GetBalance(ctx, user.ID)
	if balanceDB == nil {
		log.Println("No balance for user")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	bodyResp, err := json.Marshal(balanceDB)
	if err != nil {
		log.Printf("Cannot convert Balance to JSON: %v", err)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, errBody := w.Write(bodyResp)
	if errBody != nil {
		log.Printf("Error sending the response: %v\n", errBody)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
}
func Withdraw(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := DB.GetUser(ctx, login)

	respBody := utils.GetBody(r.Body)
	if respBody == nil {
		http.Error(w, "Couldn't read body", http.StatusInternalServerError)
		return
	}
	withdrawalRequest := db.WithdrawalsData{}
	if errJson := json.Unmarshal(*respBody, &withdrawalRequest); errJson != nil {
		http.Error(w, errJson.Error(), http.StatusBadRequest) //"wrong request",
		return
	}
	currBalance := DB.GetBalance(ctx, user.ID)
	if currBalance == nil {
		log.Println("Couldn't find balance")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if currBalance.Current < withdrawalRequest.Sum {
		log.Printf("Not enough balance: %f < %f\n", currBalance.Current, withdrawalRequest.Sum)
		http.Error(w, "not enough balance", http.StatusPaymentRequired)
		return
	}
	intOrderId, err := strconv.Atoi(withdrawalRequest.OrderId)
	if err != nil {
		http.Error(w, "wrong order format", http.StatusUnprocessableEntity)
		return
	}
	if !luhn.Valid(intOrderId) {
		http.Error(w, "wrong order format", http.StatusUnprocessableEntity)
		return
	}
	if err := DB.AddWithdrawal(ctx, user.ID, withdrawalRequest.Sum, withdrawalRequest.OrderId); err != nil {
		log.Printf("Couldn't add withdrawal %v\n", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	newBalance := currBalance.Current - withdrawalRequest.Sum
	newWithdrawn := currBalance.Withdrawn + withdrawalRequest.Sum
	if err := DB.UpdateBalance(ctx, user.ID, newBalance, newWithdrawn); err != nil {
		log.Printf("Couldn't update balance %v\n", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	token, _ := auth.GetTokenFromHeader(r)
	login, _ := auth.ParseToken(token)
	user := DB.GetUser(ctx, login)
	withdrawalsDB := DB.GetWithdrawals(ctx, user.ID)
	if withdrawalsDB == nil {
		log.Println("No withdrawals for user")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	bodyResp, err := json.Marshal(withdrawalsDB)
	if err != nil {
		log.Printf("Cannot convert withdrawals to JSON: %v", err)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, errBody := w.Write(bodyResp)
	if errBody != nil {
		log.Printf("Error sending the response: %v\n", errBody)
		http.Error(w, "Error sending the response", http.StatusInternalServerError)
		return
	}
}
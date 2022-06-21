package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fortuna91/ya_praktikum_final/internal/body"
	"github.com/fortuna91/ya_praktikum_final/internal/db"
	"github.com/fortuna91/ya_praktikum_final/internal/entity"
	"log"
	"net/http"
	"strconv"
	"time"
)

const (
	REGISTERED string = "REGISTERED"
	PROCESSING string = "PROCESSING"
	// INVALID           = "INVALID"
	// PROCESSED         = "PROCESSED"
)

var ContextCancelTimeout time.Duration
var AccrualChannelPool int
var QueueCh = make(chan entity.Order, AccrualChannelPool)
var AccrualSystemAddress string
var retryAfter = 0

func sendRequest(client *http.Client, request *http.Request) *http.Response {
	request.Header.Set("Content-Length", "0")
	response, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return response
}

func getAccrual(accrualSystemAddress string, orderID string) (*entity.Order, int) {
	client := http.Client{}
	request, _ := http.NewRequest(http.MethodGet, accrualSystemAddress+"/api/orders/"+orderID, nil)
	response := sendRequest(&client, request)
	if response == nil {
		log.Println("Error in getting accrual")
		return nil, 0
	}
	if response.StatusCode == http.StatusTooManyRequests {
		retryAfter := response.Header.Get("Retry-After")
		retryAfterInt, _ := strconv.Atoi(retryAfter)
		return nil, retryAfterInt
	} else if response.StatusCode == http.StatusNoContent {
		fmt.Printf("No Content. Response code %d", response.StatusCode)
		return nil, -1
	} else if response.StatusCode != http.StatusOK {
		fmt.Printf("Error in getting accrual. Response code %d", response.StatusCode)
		return nil, 0
	}
	orderResponse := entity.Order{}
	defer response.Body.Close()
	respBody := body.GetBody(response.Body)
	if errJSON := json.Unmarshal(*respBody, &orderResponse); errJSON != nil {
		log.Println(errJSON.Error())
		return nil, 0
	}
	return &orderResponse, 0
}

func updateOrder(db *db.DBStorage, accrualSystemAddress string, orderID string, userID int64) (*entity.Order, int) {
	ctx, cancel := context.WithTimeout(context.Background(), ContextCancelTimeout)
	defer cancel()
	order, retryAfter := getAccrual(accrualSystemAddress, orderID)
	fmt.Printf("Get order from accrual system %v\n", order)
	if order == nil {
		return nil, retryAfter
	}
	status := order.Status
	/*if order.Status == REGISTERED {
		status = PROCESSING
	}*/ // no status REGISTERED in technical task
	if err := db.UpdateOrder(ctx, orderID, userID, status, order.Accrual); err != nil {
		fmt.Println(err.Error())
		return nil, 0
	}
	if err := db.UpdateBalance(ctx, userID, order.Accrual); err != nil {
		fmt.Println(err.Error())
		return nil, 0
	}
	return order, 0
}

func UpdateOrders(db *db.DBStorage) {
	for {
		order := <-QueueCh
		accrualOrder, retryAfterNew := updateOrder(db, AccrualSystemAddress, order.ID, order.UserID)
		if retryAfterNew > 0 {
			retryAfter = retryAfterNew
			QueueCh <- order
		} else if retryAfter == 0 {
			if accrualOrder == nil || accrualOrder.Status == REGISTERED || accrualOrder.Status == PROCESSING {
				QueueCh <- order
			}
		}
		time.Sleep(time.Duration(retryAfter) * time.Second)
		retryAfter = 0
	}
}

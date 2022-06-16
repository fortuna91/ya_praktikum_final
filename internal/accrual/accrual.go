package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fortuna91/ya_praktikum_final/internal/db"
	"github.com/fortuna91/ya_praktikum_final/internal/utils"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	REGISTERED string = "REGISTERED"
	INVALID           = "INVALID"
	PROCESSING        = "PROCESSING"
	PROCESSED         = "PROCESSED"
)

type QueueAccrualSystem struct {
	Queue                []db.OrderData
	AccrualSystemAddress string
	RetryAfter           int
	mtx                  sync.RWMutex
}

func sendRequest(client *http.Client, request *http.Request) *http.Response {
	request.Header.Set("Content-Length", "0")
	response, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return response
}

func getAccrual(accrualSystemAddress string, orderId string) (*db.OrderData, int) {
	client := http.Client{}
	request, _ := http.NewRequest(http.MethodGet, "http://"+accrualSystemAddress+"/api/orders/"+orderId, nil)
	response := sendRequest(&client, request)
	if response == nil {
		log.Println("Error in getting accrual")
		return nil, 0
	}
	if response.StatusCode == 429 {
		retryAfter := response.Header.Get("Retry-After")
		retryAfterInt, _ := strconv.Atoi(retryAfter)
		return nil, retryAfterInt
	} else if response.StatusCode == 204 {
		fmt.Printf("No Content. Response code %d", response.StatusCode)
		return nil, -1
	} else if response.StatusCode != 200 {
		fmt.Printf("Error in getting accrual. Response code %d", response.StatusCode)
		return nil, 0
	}
	orderResponse := db.OrderData{}
	defer response.Body.Close()
	respBody := utils.GetBody(response.Body)
	if errJson := json.Unmarshal(*respBody, &orderResponse); errJson != nil {
		log.Println(errJson.Error())
		return nil, 0
	}
	return &orderResponse, 0
}

func updateOrder(db *db.DbStorage, accrualSystemAddress string, orderId string, userId int64) (*db.OrderData, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	order, retryAfter := getAccrual(accrualSystemAddress, orderId)
	if order == nil {
		return nil, retryAfter
	}
	status := order.Status
	if order.Status == REGISTERED {
		status = PROCESSING
	}
	if err := db.UpdateOrder(ctx, orderId, status, order.Accrual); err != nil {
		fmt.Println(err.Error())
		return nil, 0
	}
	balance := db.GetBalance(ctx, userId)
	newBalance := balance.Current + order.Accrual
	if err := db.UpdateBalance(ctx, userId, newBalance, balance.Withdrawn); err != nil {
		fmt.Println(err.Error())
		return nil, 0
	}
	return order, 0
}

func (queue *QueueAccrualSystem) Append(order db.OrderData) {
	queue.mtx.Lock()
	defer queue.mtx.Unlock()
	queue.Queue = append(queue.Queue, order)
}

func (queue *QueueAccrualSystem) Pop() *db.OrderData {
	queue.mtx.Lock()
	defer queue.mtx.Unlock()
	if len(queue.Queue) <= 0 {
		return nil
	}
	order := queue.Queue[0]
	queue.Queue = queue.Queue[1:len(queue.Queue)]
	return &order
}

func (queue *QueueAccrualSystem) UpdateOrders(db *db.DbStorage) {
	for true {
		order := queue.Pop()
		if order != nil {
			fmt.Printf("Get order from accrual system %v\n", order)
			accrualOrder, retryAfter := updateOrder(db, queue.AccrualSystemAddress, order.ID, order.UserId)
			if retryAfter > 0 {
				queue.RetryAfter = retryAfter
				queue.Append(*order)
			} else if retryAfter == 0 {
				if accrualOrder == nil || accrualOrder.Status == REGISTERED || accrualOrder.Status == PROCESSING {
					queue.Append(*order)
				}
			}
			time.Sleep(time.Duration(retryAfter) * time.Second)
			queue.RetryAfter = 0
		}
	}
}

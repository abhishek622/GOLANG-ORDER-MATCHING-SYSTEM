package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage/mysql"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
)

type MatchingEngine struct {
	orderChan  chan *types.Order
	cancelChan chan int64
	orderBook  map[string]*OrderBook
	storage    *mysql.Mysql
	mu         sync.RWMutex
}

func NewMatchingEngine(m *mysql.Mysql) *MatchingEngine {
	engine := &MatchingEngine{
		orderChan:  make(chan *types.Order),
		cancelChan: make(chan int64),
		orderBook:  make(map[string]*OrderBook),
		storage:    m,
		mu:         sync.RWMutex{},
	}
	go engine.run()
	return engine
}

func (e *MatchingEngine) run() {
	for {
		select {
		case order := <-e.orderChan:
			e.processOrder(order)
		case orderID := <-e.cancelChan:
			e.cancelOrder(orderID)
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func (e *MatchingEngine) SubmitOrder(order *types.Order) {
	order.Status = "open"
	order.Remaining = order.Quantity

	if _, err := e.storage.PlaceOrder(order); err != nil {
		fmt.Println("Failed to place order", err)
		return
	}

	e.orderChan <- order
}

func (e *MatchingEngine) CancelOrder(orderID int64) {
	e.cancelChan <- orderID
}

func (e *MatchingEngine) processOrder(order *types.Order) {
	e.mu.Lock()
	book, exists := e.orderBook[order.Symbol]
	if !exists {
		book = NewOrderBook()
		e.orderBook[order.Symbol] = book
	}
	e.mu.Unlock()

	book.Match(order, func(trade *types.Trade) {
		if _, err := e.storage.CreateTrade(trade); err != nil {
			fmt.Println("Failed to create trade", err)
			return
		}
	})

	if order.Remaining == 0 {
		order.Status = types.FILLED
		_ = e.storage.MarkOrderFilled(order.OrderID)
	} else if order.OrderType == types.LIMIT {
		order.Status = types.PARTIAL
		book.AddOrder(order)
	}
}

func (e *MatchingEngine) cancelOrder(orderID int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, book := range e.orderBooks {
		if book.RemoveOrder(orderID) {
			fmt.Printf("Order %d cancelled\n", orderID)
			_ = e.storage.MarkOrderCancelled(orderID)
			break
		}
	}
}

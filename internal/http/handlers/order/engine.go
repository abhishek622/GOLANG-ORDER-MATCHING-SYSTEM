package order

import (
	"log/slog"
	"sort"
	"strconv"
	"sync"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage/mysql"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
)

type OrderBook struct {
	Bids []*types.Order
	Asks []*types.Order
	mu   sync.RWMutex
}

// NewOrderBook creates a new order book
func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids: []*types.Order{},
		Asks: []*types.Order{},
	}
}

// AddOrder adds an order to the order book
func (ob *OrderBook) AddOrder(order *types.Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if order.Side == types.BUY {
		ob.Bids = append(ob.Bids, order)
		// Sort bids by price (highest first)
		sort.SliceStable(ob.Bids, func(i, j int) bool {
			if ob.Bids[i].Price == nil || ob.Bids[j].Price == nil {
				return false
			}
			return *ob.Bids[i].Price > *ob.Bids[j].Price
		})
	} else {
		ob.Asks = append(ob.Asks, order)
		// Sort asks by price (lowest first)
		sort.SliceStable(ob.Asks, func(i, j int) bool {
			if ob.Asks[i].Price == nil || ob.Asks[j].Price == nil {
				return false
			}
			return *ob.Asks[i].Price < *ob.Asks[j].Price
		})
	}
}

// RemoveOrder removes an order by ID from the order book
func (ob *OrderBook) RemoveOrder(orderID string) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	for i, order := range ob.Bids {
		if order.OrderID == orderID {
			ob.Bids = append(ob.Bids[:i], ob.Bids[i+1:]...)
			return true
		}
	}
	for i, order := range ob.Asks {
		if order.OrderID == orderID {
			ob.Asks = append(ob.Asks[:i], ob.Asks[i+1:]...)
			return true
		}
	}
	return false
}

// Match tries to match the incoming order with the order book
func (ob *OrderBook) Match(order *types.Order, onTrade func(trade *types.Trade)) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if order.Side == types.BUY {
		// Try to match with lowest ask
		for len(ob.Asks) > 0 && order.Remaining > 0 {
			bestAsk := ob.Asks[0]
			if order.OrderType == types.LIMIT && order.Price != nil && bestAsk.Price != nil && *order.Price < *bestAsk.Price {
				break
			}
			tradeQty := min(order.Remaining, bestAsk.Remaining)
			trade := &types.Trade{
				Symbol:      order.Symbol,
				BuyOrderID:  order.OrderID,
				SellOrderID: bestAsk.OrderID,
				Quantity:    tradeQty,
				Price:       *bestAsk.Price,
			}
			onTrade(trade)
			order.Remaining -= tradeQty
			bestAsk.Remaining -= tradeQty
			if bestAsk.Remaining == 0 {
				ob.Asks = ob.Asks[1:]
			}
		}
	} else {
		// Try to match with highest bid
		for len(ob.Bids) > 0 && order.Remaining > 0 {
			bestBid := ob.Bids[0]
			if order.OrderType == types.LIMIT && order.Price != nil && bestBid.Price != nil && *order.Price > *bestBid.Price {
				break
			}
			tradeQty := min(order.Remaining, bestBid.Remaining)
			trade := &types.Trade{
				Symbol:      order.Symbol,
				BuyOrderID:  bestBid.OrderID,
				SellOrderID: order.OrderID,
				Quantity:    tradeQty,
				Price:       *bestBid.Price,
			}
			onTrade(trade)
			order.Remaining -= tradeQty
			bestBid.Remaining -= tradeQty
			if bestBid.Remaining == 0 {
				ob.Bids = ob.Bids[1:]
			}
		}
	}
}

// GetSnapshot returns a snapshot of the order book
func (ob *OrderBook) GetSnapshot() map[string][]*types.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return map[string][]*types.Order{
		"bids": ob.Bids,
		"asks": ob.Asks,
	}
}

// NewOrderHandler creates a new order handler with built-in matching engine
func NewOrderHandler(storage *mysql.Mysql) *OrderHandler {
	return &OrderHandler{
		Storage:    storage,
		OrderBooks: make(map[string]*OrderBook),
	}
}

func (h *OrderHandler) getOrCreateOrderBook(symbol string) *OrderBook {
	h.mu.Lock()
	defer h.mu.Unlock()

	if book, exists := h.OrderBooks[symbol]; exists {
		return book
	}

	book := NewOrderBook()
	h.OrderBooks[symbol] = book
	return book
}

// processOrder processes an order through the matching engine
func (h *OrderHandler) processOrder(order *types.Order) []*types.Trade {
	book := h.getOrCreateOrderBook(order.Symbol)
	var trades []*types.Trade

	book.Match(order, func(trade *types.Trade) {
		trades = append(trades, trade)
		// Store trade in database
		if _, err := h.Storage.CreateTrade(*trade); err != nil {
			slog.Error("Failed to create trade", slog.String("error", err.Error()))
		}
	})

	// Update order status
	if order.Remaining == 0 {
		order.Status = types.FILLED
		orderIDInt, _ := strconv.ParseInt(order.OrderID, 10, 64)
		_ = h.Storage.MarkOrderFilled(orderIDInt)
	} else if order.OrderType == types.LIMIT {
		order.Status = types.PARTIAL
		book.AddOrder(order)
	}

	return trades
}

// Helper function
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

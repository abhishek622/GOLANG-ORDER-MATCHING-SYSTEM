package order

import (
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage"
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
func (ob *OrderBook) RemoveOrder(orderID int64) bool {
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
		// For buy orders, match with lowest ask prices first
		for len(ob.Asks) > 0 && order.Remaining > 0 {
			bestAsk := ob.Asks[0]

			// For limit orders, stop if the best ask is higher than our bid
			if order.OrderType == types.LIMIT && order.Price != nil && bestAsk.Price != nil && *order.Price < *bestAsk.Price {
				break
			}

			tradeQty := min(order.Remaining, bestAsk.Remaining)
			// Determine trade price (resting order's price has priority)
			var tradePrice float64
			if bestAsk.Price != nil {
				tradePrice = *bestAsk.Price
			} else if order.Price != nil {
				tradePrice = *order.Price
			}

			trade := &types.Trade{
				Symbol:      order.Symbol,
				BuyOrderID:  order.OrderID,
				SellOrderID: bestAsk.OrderID,
				Quantity:    tradeQty,
				Price:       tradePrice,
				CreatedAt:   time.Now(),
			}

			onTrade(trade)

			// Update quantities
			order.Remaining -= tradeQty
			bestAsk.Remaining -= tradeQty

			// Remove fully filled orders from the book
			if bestAsk.Remaining == 0 {
				ob.Asks = ob.Asks[1:]
			}
		}
	} else {
		// For sell orders, match with highest bid prices first
		for len(ob.Bids) > 0 && order.Remaining > 0 {
			bestBid := ob.Bids[0]

			// For limit orders, stop if the best bid is lower than our ask
			if order.OrderType == types.LIMIT && order.Price != nil && bestBid.Price != nil && *order.Price > *bestBid.Price {
				break
			}

			tradeQty := min(order.Remaining, bestBid.Remaining)
			// Determine trade price (resting order's price has priority)
			var tradePrice float64
			if bestBid.Price != nil {
				tradePrice = *bestBid.Price
			} else if order.Price != nil {
				tradePrice = *order.Price
			}

			trade := &types.Trade{
				Symbol:      order.Symbol,
				BuyOrderID:  bestBid.OrderID,
				SellOrderID: order.OrderID,
				Quantity:    tradeQty,
				Price:       tradePrice,
				CreatedAt:   time.Now(),
			}

			onTrade(trade)

			// Update quantities
			order.Remaining -= tradeQty
			bestBid.Remaining -= tradeQty

			// Remove fully filled orders from the book
			if bestBid.Remaining == 0 {
				ob.Bids = ob.Bids[1:]
			}
		}
	}
}

// OrderBookEntry represents a single entry in the order book
type OrderBookEntry struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
}

// GetSnapshot returns a snapshot of the order book
func (ob *OrderBook) GetSnapshot() map[string]interface{} {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]OrderBookEntry, 0, len(ob.Bids))
	for _, bid := range ob.Bids {
		if bid.Price != nil {
			bids = append(bids, OrderBookEntry{
				Price:    *bid.Price,
				Quantity: bid.Quantity,
			})
		}
	}

	asks := make([]OrderBookEntry, 0, len(ob.Asks))
	for _, ask := range ob.Asks {
		if ask.Price != nil {
			asks = append(asks, OrderBookEntry{
				Price:    *ask.Price,
				Quantity: ask.Quantity,
			})
		}
	}

	return map[string]interface{}{
		"bids": bids,
		"asks": asks,
	}
}

// OrderHandler handles HTTP requests for order operations
type OrderHandler struct {
	Storage    storage.Storage
	OrderBooks map[string]*OrderBook
	mu         sync.RWMutex
}

// NewOrderHandler creates a new order handler with built-in matching engine
func NewOrderHandler(storage storage.Storage) *OrderHandler {
	return &OrderHandler{
		Storage:    storage,
		OrderBooks: make(map[string]*OrderBook),
	}
}

func (h *OrderHandler) getOrCreateOrderBook(symbol string) *OrderBook {
	h.mu.RLock()
	book, exists := h.OrderBooks[symbol]
	h.mu.RUnlock()

	if !exists {
		h.mu.Lock()
		// Check again in case another goroutine created it while we were waiting for the lock
		book, exists = h.OrderBooks[symbol]
		if !exists {
			book = NewOrderBook()
			h.OrderBooks[symbol] = book
			slog.Info("Created new order book for symbol", "symbol", symbol)
		}
		h.mu.Unlock()
	}

	return book
}

// processOrder processes an order through the matching engine
func (h *OrderHandler) processOrder(order *types.Order) []*types.Trade {
	book := h.getOrCreateOrderBook(order.Symbol)
	var trades []*types.Trade

	// For market orders, we'll try to match with the best available price
	if order.OrderType == types.MARKET {
		order.Price = nil // Clear price for market orders
	}

	// Try to match the order
	book.Match(order, func(trade *types.Trade) {
		trades = append(trades, trade)
		// Store trade in database
		if _, err := h.Storage.CreateTrade(*trade); err != nil {
			slog.Error("Failed to create trade",
				slog.String("error", err.Error()),
				slog.Int64("buy_order", trade.BuyOrderID),
				slog.Int64("sell_order", trade.SellOrderID))
		}
	})

	// Update order status based on remaining quantity
	switch {
	case order.Remaining == 0:
		// Fully filled
		order.Status = types.FILLED
		if err := h.Storage.MarkOrderFilled(order.OrderID); err != nil {
			slog.Error("Failed to mark order as filled",
				slog.String("error", err.Error()),
				slog.Int64("order_id", order.OrderID))
		}

	case order.OrderType == types.MARKET:
		// Market order with remaining quantity gets cancelled
		order.Status = types.CANCELLED
		if err := h.Storage.MarkOrderCancelled(order.OrderID); err != nil {
			slog.Error("Failed to cancel unfilled market order",
				slog.String("error", err.Error()),
				slog.Int64("order_id", order.OrderID))
		}

	case order.OrderType == types.LIMIT:
		// Limit order with remaining quantity goes to the book
		order.Status = types.OPEN
		if order.Remaining < order.Quantity {
			order.Status = types.PARTIAL
		}
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

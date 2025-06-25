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

// new order book
func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids: []*types.Order{},
		Asks: []*types.Order{},
	}
}

func (ob *OrderBook) AddOrder(order *types.Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	// Set the current time if not already set
	if order.CreatedAt.IsZero() {
		order.CreatedAt = time.Now()
	}

	order.Remaining = order.Quantity

	if order.Side == types.BUY {
		ob.Bids = append(ob.Bids, order)
		// Sort bids by price (highest first) and then by time (earliest first)
		sort.SliceStable(ob.Bids, func(i, j int) bool {
			// Handle nil prices (shouldn't happen for limit orders, but just in case)
			if ob.Bids[i].Price == nil || ob.Bids[j].Price == nil {
				return false
			}
			// If prices are equal, sort by time (earlier order first)
			if *ob.Bids[i].Price == *ob.Bids[j].Price {
				return ob.Bids[i].CreatedAt.Before(ob.Bids[j].CreatedAt)
			}
			// Otherwise sort by price (higher first)
			return *ob.Bids[i].Price > *ob.Bids[j].Price
		})

	} else {
		ob.Asks = append(ob.Asks, order)
		// Sort asks by price (lowest first) and then by time (earliest first)
		sort.SliceStable(ob.Asks, func(i, j int) bool {
			// If prices are the same, sort by creation time (earliest first)
			if ob.Asks[i].Price != nil && ob.Asks[j].Price != nil && *ob.Asks[i].Price == *ob.Asks[j].Price {
				return ob.Asks[i].CreatedAt.Before(ob.Asks[j].CreatedAt)
			}
			// Handle nil prices (shouldn't happen for limit orders)
			if ob.Asks[i].Price == nil || ob.Asks[j].Price == nil {
				return false
			}
			// Otherwise sort by price (lower first)
			return *ob.Asks[i].Price < *ob.Asks[j].Price
		})

	}
}

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

func (ob *OrderBook) Match(order *types.Order, onTrade func(trade *types.Trade)) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if order.Side == types.BUY {
		// For buy orders, match with lowest ask prices first
		for len(ob.Asks) > 0 && order.Remaining > 0 {
			bestAsk := ob.Asks[0]

			// For limit orders, stop if the best ask is higher than our bid
			if order.OrderType == types.LIMIT {
				if order.Price != nil && bestAsk.Price != nil && *order.Price < *bestAsk.Price {
					break
				}
			}

			tradeQty := min(bestAsk.Remaining, order.Remaining)
			// Determine trade price (resting order's price has priority)
			var tradePrice int64
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
			if order.OrderType == types.LIMIT {
				if order.Price != nil && bestBid.Price != nil && *order.Price > *bestBid.Price {
					break
				}
			}

			tradeQty := min(bestBid.Remaining, order.Remaining)
			// Determine trade price (resting order's price has priority)
			var tradePrice int64
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

type OrderBookEntry struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

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

// NewOrderHandler creates a new order handler with matching engine
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

func (h *OrderHandler) processOrder(order *types.Order) []*types.Trade {
	book := h.getOrCreateOrderBook(order.Symbol)
	var trades []*types.Trade

	if order.CreatedAt.IsZero() {
		order.CreatedAt = time.Now()
	}

	// For market orders match with the best available price
	if order.OrderType == types.MARKET {
		order.Price = nil
	}

	book.Match(order, func(trade *types.Trade) {
		tradeID, err := h.Storage.CreateTrade(*trade)
		if err != nil {
			slog.Error("Failed to create trade", slog.String("error", err.Error()))
			return
		}
		trade.TradeID = tradeID
		trades = append(trades, trade)
	})

	// Update order status based on remaining quantity
	switch {
	case order.Remaining == 0:
		order.Status = types.FILLED // Fully filled
		if err := h.Storage.MarkOrderFilled(order.OrderID); err != nil {
			slog.Error("Failed to mark order as filled", slog.String("error", err.Error()))
		}

	case order.OrderType == types.MARKET:
		// For market orders, if there's remaining quantity, mark as partially filled
		if order.Remaining > 0 {
			if order.Remaining < order.Quantity {
				order.Status = types.PARTIAL
			} else {
				order.Status = types.CANCELLED // No matches found at all
			}
		}

		switch order.Status {
		case types.CANCELLED:
			if err := h.Storage.MarkOrderCancelled(order.OrderID); err != nil {
				slog.Error("Failed to update market order status", slog.String("error", err.Error()))
			}
		case types.PARTIAL:
			if err := h.Storage.UpdateOrderStatus(order.OrderID, types.PARTIAL, order.Remaining); err != nil {
				slog.Error("Failed to update market order status", slog.String("error", err.Error()))
			}
		}

	case order.OrderType == types.LIMIT:
		// Limit order with remaining quantity goes to the book
		if order.Remaining > 0 {
			order.Status = types.OPEN
			if order.Remaining < order.Quantity {
				order.Status = types.PARTIAL
			}
			book.AddOrder(order)
		}
	}

	return trades
}

package order

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/utils/response"
)

func (h *OrderHandler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJson(w, http.StatusMethodNotAllowed, response.GeneralErrorString("method not allowed"))
		return
	}

	var req types.PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("invalid request body"))
		return
	}

	// Basic validation
	if req.Quantity <= 0 {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("quantity must be positive"))
		return
	}

	// Validate order type specific requirements
	switch req.Type {
	case types.LIMIT:
		if req.Price == nil || *req.Price <= 0 {
			response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("limit orders require a positive price"))
			return
		}
	case types.MARKET:
		// Market orders don't need a price
		if req.Price != nil {
			// Clear price for market orders to avoid confusion
			req.Price = nil
		}
	default:
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("invalid order type"))
		return
	}

	// Validate order side
	if req.Side != types.BUY && req.Side != types.SELL {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("invalid order side"))
		return
	}

	// Create order struct
	order := &types.Order{
		Symbol:    req.Symbol,
		Side:      req.Side,
		OrderType: req.Type,
		Price:     req.Price,
		Quantity:  req.Quantity,
		Remaining: req.Quantity,
		Status:    types.OPEN,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store order in database and get the order ID
	orderId, err := h.Storage.PlaceOrder(*order)
	if err != nil {
		slog.Error("Failed to place order in database", 
			slog.String("error", err.Error()),
			slog.String("symbol", order.Symbol),
			slog.String("side", string(order.Side)))
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to place order"))
		return
	}

	// Set the order ID from database
	order.OrderID = orderId

	slog.Info("Processing order",
		slog.Int64("order_id", orderId),
		slog.String("symbol", order.Symbol),
		slog.String("side", string(order.Side)),
		slog.String("type", string(order.OrderType)))

	// Process order through matching engine
	trades := h.processOrder(order)

	// Log the result
	if len(trades) > 0 {
		slog.Info("Order matched with trades",
			slog.Int64("order_id", orderId),
			slog.Int("trade_count", len(trades)),
			slog.String("status", string(order.Status)))
	} else {
		slog.Info("Order added to order book",
			slog.Int64("order_id", orderId),
			slog.String("status", string(order.Status)))
	}

	response.WriteJson(w, http.StatusOK, map[string]interface{}{
		"message": "order placed successfully",
		"data": map[string]interface{}{
			"order_id": orderId,
			"status":   string(order.Status),
			"trades":   trades,
		},
	})
}

func (h *OrderHandler) GetOrderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("orderId")
	orderID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralError(fmt.Errorf("invalid order id")))
		return
	}

	order, err := h.Storage.GetOrderStatus(orderID)
	if err != nil {
		response.WriteJson(w, http.StatusNotFound, response.GeneralError(err))
		return
	}

	response.WriteJson(w, http.StatusOK, map[string]interface{}{
		"message": "order status fetched successfully",
		"data":    order,
	})
}

func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.WriteJson(w, http.StatusMethodNotAllowed, response.GeneralErrorString("method not allowed"))
		return
	}

	orderIDStr := r.PathValue("orderId")
	if orderIDStr == "" {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("order id is required"))
		return
	}

	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil || orderID <= 0 {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("invalid order id"))
		return
	}

	slog.Info("Cancelling order", slog.Int64("order_id", orderID))

	// Get order from storage
	order, err := h.Storage.GetOrderStatus(orderID)
	if err != nil {
		slog.Error("Failed to fetch order for cancellation", 
			slog.String("error", err.Error()),
			slog.Int64("order_id", orderID))
		response.WriteJson(w, http.StatusNotFound, response.GeneralErrorString("order not found"))
		return
	}

	switch order.Status {
	case types.FILLED:
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("cannot cancel a filled order"))
		return
	case types.CANCELLED:
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("order is already cancelled"))
		return
	}

	// Remove from order book if it's still open
	if order.Status == types.OPEN || order.Status == types.PARTIAL {
		h.mu.Lock()
		if book, exists := h.OrderBooks[order.Symbol]; exists {
			if !book.RemoveOrder(orderID) {
				slog.Warn("Order not found in order book", slog.Int64("order_id", orderID))
			}
		} else {
			slog.Warn("Order book not found for symbol", slog.String("symbol", order.Symbol))
		}
		h.mu.Unlock()
	}

	// Update order status in database
	err = h.Storage.MarkOrderCancelled(orderID)
	if err != nil {
		slog.Error("Failed to cancel order in database",
			slog.String("error", err.Error()),
			slog.Int64("order_id", orderID))
		response.WriteJson(w, http.StatusInternalServerError, 
			response.GeneralErrorString("failed to cancel order"))
		return
	}

	slog.Info("Order cancelled successfully", slog.Int64("order_id", orderID))

	response.WriteJson(w, http.StatusOK, map[string]interface{}{
		"message": "order cancelled successfully",
		"data": map[string]interface{}{
			"order_id": orderID,
			"status":   types.CANCELLED,
		},
	})
}

// OrderBookPriceLevel represents a single price level in the order book
type OrderBookPriceLevel struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
}

// OrderBookSnapshot represents the current state of the order book
type OrderBookSnapshot struct {
	Symbol string                `json:"symbol"`
	Bids   []OrderBookPriceLevel `json:"bids"`
	Asks   []OrderBookPriceLevel `json:"asks"`
}

// GetOrderBook returns the current state of the order book for a given symbol
func (h *OrderHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.WriteJson(w, http.StatusMethodNotAllowed, response.GeneralErrorString("method not allowed"))
		return
	}

	// Extract symbol from query parameters
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("symbol query parameter is required"))
		return
	}

	h.mu.RLock()
	book, exists := h.OrderBooks[symbol]
	h.mu.RUnlock()

	snapshot := OrderBookSnapshot{
		Symbol: symbol,
		Bids:   []OrderBookPriceLevel{},
		Asks:   []OrderBookPriceLevel{},
	}

	if !exists {
		response.WriteJson(w, http.StatusOK, map[string]interface{}{
			"message": "order book is empty",
			"data":    snapshot,
		})
		return
	}

	// Get the raw order book data
	book.mu.RLock()
	defer book.mu.RUnlock()

	// Process bids
	bidLevels := make(map[float64]int64)
	for _, bid := range book.Bids {
		if bid.Price != nil {
			bidLevels[*bid.Price] += bid.Quantity
		}
	}

	// Convert bid levels to price levels
	for price, quantity := range bidLevels {
		snapshot.Bids = append(snapshot.Bids, OrderBookPriceLevel{
			Price:    price,
			Quantity: quantity,
		})
	}

	// Process asks
	askLevels := make(map[float64]int64)
	for _, ask := range book.Asks {
		if ask.Price != nil {
			askLevels[*ask.Price] += ask.Quantity
		}
	}

	// Convert ask levels to price levels
	for price, quantity := range askLevels {
		snapshot.Asks = append(snapshot.Asks, OrderBookPriceLevel{
			Price:    price,
			Quantity: quantity,
		})
	}

	// Sort bids in descending order (highest bid first)
	sort.Slice(snapshot.Bids, func(i, j int) bool {
		return snapshot.Bids[i].Price > snapshot.Bids[j].Price
	})

	// Sort asks in ascending order (lowest ask first)
	sort.Slice(snapshot.Asks, func(i, j int) bool {
		return snapshot.Asks[i].Price < snapshot.Asks[j].Price
	})

	// Limit the number of levels to return (e.g., top 10)
	const maxLevels = 10
	if len(snapshot.Bids) > maxLevels {
		snapshot.Bids = snapshot.Bids[:maxLevels]
	}
	if len(snapshot.Asks) > maxLevels {
		snapshot.Asks = snapshot.Asks[:maxLevels]
	}

	response.WriteJson(w, http.StatusOK, map[string]interface{}{
		"message": "order book retrieved successfully",
		"data":    snapshot,
	})
}

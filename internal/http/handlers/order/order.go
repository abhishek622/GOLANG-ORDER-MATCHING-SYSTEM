package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/utils/response"
	"github.com/go-playground/validator/v10"
)

func (h *OrderHandler) PlaceOrder(w http.ResponseWriter, r *http.Request) {

	var orderBody types.PlaceOrderRequest
	err := json.NewDecoder(r.Body).Decode(&orderBody)
	if errors.Is(err, io.EOF) {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralError(fmt.Errorf("empty body")))
		return
	}
	if err != nil {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("invalid request body"))
		return
	}

	if err := validator.New().Struct(orderBody); err != nil {
		validateErrors := err.(validator.ValidationErrors)
		response.WriteJson(w, http.StatusBadRequest, response.ValidationError(validateErrors))
		return
	}

	order := &types.Order{
		Symbol:    orderBody.Symbol,
		Side:      orderBody.Side,
		OrderType: orderBody.Type,
		Price:     orderBody.Price,
		Quantity:  orderBody.Quantity,
		Remaining: orderBody.Quantity,
		Status:    types.OPEN,
	}

	orderId, err := h.Storage.PlaceOrder(*order)
	if err != nil {
		slog.Error("Failed to place order in database", slog.String("error", err.Error()))
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to place order"))
		return
	}

	order.OrderID = orderId
	slog.Info("Processing order", slog.Int64("order_id", orderId))
	trades := h.processOrder(order)

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "order placed successfully",
		"data": map[string]any{
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
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralError(fmt.Errorf("failed to get order status: %w", err)))
		return
	}

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "order status fetched successfully",
		"data":    order,
	})
}

func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.WriteJson(w, http.StatusMethodNotAllowed, response.GeneralErrorString("method not allowed"))
		return
	}

	id := r.PathValue("orderId")
	orderID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || orderID <= 0 {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("invalid order id"))
		return
	}

	slog.Info("Cancelling order", slog.Int64("order_id", orderID))

	order, err := h.Storage.GetOrderStatus(orderID)
	if err != nil {
		slog.Error("Failed to fetch order for cancellation", slog.String("error", err.Error()))
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
			slog.Warn("Order book not found for", slog.String("symbol", order.Symbol))
		}
		h.mu.Unlock()
	}

	err = h.Storage.MarkOrderCancelled(orderID)
	if err != nil {
		slog.Error("Failed to cancel order in database", slog.String("error", err.Error()))
		response.WriteJson(w, http.StatusInternalServerError,
			response.GeneralErrorString("failed to cancel order"))
		return
	}

	slog.Info("Order cancelled successfully", slog.Int64("order_id", orderID))

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "order cancelled successfully",
		"data": map[string]any{
			"order_id": orderID,
			"status":   types.CANCELLED,
		},
	})
}

func (h *OrderHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("symbol query parameter is required"))
		return
	}

	h.mu.RLock()
	book, exists := h.OrderBooks[symbol]
	h.mu.RUnlock()

	snapshot := types.OrderBookSnapshot{
		Symbol: symbol,
		Bids:   []types.OrderBookPriceLevel{},
		Asks:   []types.OrderBookPriceLevel{},
	}

	if !exists {
		response.WriteJson(w, http.StatusOK, map[string]any{
			"message": "order book is empty",
			"data":    snapshot,
		})
		return
	}

	// Get the raw order book data
	book.mu.RLock()
	defer book.mu.RUnlock()

	// Process bids
	bidLevels := make(map[int64]int64)
	for _, bid := range book.Bids {
		if bid.Price != nil {
			bidLevels[*bid.Price] += bid.Quantity
		}
	}

	// Convert bid levels to price levels
	for price, quantity := range bidLevels {
		snapshot.Bids = append(snapshot.Bids, types.OrderBookPriceLevel{
			Price:    price,
			Quantity: quantity,
		})
	}

	// Process asks
	askLevels := make(map[int64]int64)
	for _, ask := range book.Asks {
		if ask.Price != nil {
			askLevels[*ask.Price] += ask.Quantity
		}
	}

	// Convert ask levels to price levels
	for price, quantity := range askLevels {
		snapshot.Asks = append(snapshot.Asks, types.OrderBookPriceLevel{
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

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "order book retrieved successfully",
		"data":    snapshot,
	})
}

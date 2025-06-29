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

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/utils/response"
	"github.com/go-playground/validator/v10"
)

type OrderHandler struct {
	Storage storage.Storage
}

func NewOrderHandler(storage storage.Storage) *OrderHandler {
	return &OrderHandler{
		Storage: storage,
	}
}

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

	tx, err := h.Storage.Begin()
	if err != nil {
		slog.Error("Failed to begin transaction", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to start transaction"))
		return
	}

	// defer rollback in case of error
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // re-throw panic after rollback
		} else if err != nil {
			tx.Rollback()
		}
	}()

	// create new order
	order := &types.Order{
		Symbol:    orderBody.Symbol,
		Side:      orderBody.Side,
		OrderType: orderBody.Type,
		Price:     orderBody.Price,
		Quantity:  orderBody.Quantity,
		Remaining: orderBody.Quantity,
		Status:    types.OPEN,
	}

	orderID, err := h.Storage.PlaceOrder(tx, *order)
	if err != nil {
		slog.Error("Failed to place order in database", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to place order"))
		return
	}

	order.OrderID = orderID
	slog.Info("Processing order", "order_id", orderID)

	// process order for matching
	trades, err := h.processOrder(tx, order)
	if err != nil {
		slog.Error("Failed to process order", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to process order"))
		return
	}

	// commit the transaction if everything succeeded
	if err = tx.Commit(); err != nil {
		slog.Error("Failed to commit transaction", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to complete order processing"))
		return
	}

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "order placed successfully",
		"data": map[string]any{
			"order_id": orderID,
			"status":   order.Status,
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
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("orderId")
	orderID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralError(fmt.Errorf("invalid order id")))
		return
	}

	slog.Info("Cancelling order", "order_id", orderID)

	tx, err := h.Storage.Begin()
	if err != nil {
		slog.Error("Failed to begin transaction", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to start transaction"))
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		} else if err != nil {
			tx.Rollback()
		}
	}()

	order, err := h.Storage.GetOrderStatus(orderID)
	if err != nil {
		slog.Error("Failed to fetch order", "error", err)
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
	case types.OPEN, types.PARTIAL:
		// proceed with cancellation
	default:
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("invalid order status"))
		return
	}

	err = h.Storage.MarkOrderCancelled(tx, orderID)
	if err != nil {
		slog.Error("Failed to cancel order", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to cancel order"))
		return
	}

	if err = tx.Commit(); err != nil {
		slog.Error("Failed to commit transaction", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to complete cancellation"))
		return
	}

	slog.Info("Order cancelled successfully", "order_id", orderID)

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "order cancelled successfully",
		"data": map[string]any{
			"order_id": orderID,
			"status":   "cancelled",
		},
	})
}

func (h *OrderHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("symbol is required"))
		return
	}

	orders, err := h.Storage.GetMatchingOrders(symbol, nil)
	if err != nil {
		slog.Error("failed to get orders from database", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to get order book"))
		return
	}

	snapshot := types.OrderBookSnapshot{
		Symbol: symbol,
		Bids:   []types.OrderBookEntry{},
		Asks:   []types.OrderBookEntry{},
	}

	bidLevels := make(map[int64]int64)
	askLevels := make(map[int64]int64)

	for _, order := range orders {
		if order.Symbol != symbol || order.Status == types.CANCELLED {
			continue
		}

		if order.Price == nil {
			// skip market orders as they don't have a price
			continue
		}

		switch order.Side {
		case types.BUY:
			bidLevels[*order.Price] += order.Remaining
		case types.SELL:
			askLevels[*order.Price] += order.Remaining
		}
	}

	// convert bid levels to order book entries
	for price, quantity := range bidLevels {
		snapshot.Bids = append(snapshot.Bids, types.OrderBookEntry{
			Price:    price,
			Quantity: quantity,
		})
	}

	// convert ask levels to order book entries
	for price, quantity := range askLevels {
		snapshot.Asks = append(snapshot.Asks, types.OrderBookEntry{
			Price:    price,
			Quantity: quantity,
		})
	}

	// sort bids, highest bid first
	sort.Slice(snapshot.Bids, func(i, j int) bool {
		return snapshot.Bids[i].Price > snapshot.Bids[j].Price
	})

	// sort asks, lowest ask first
	sort.Slice(snapshot.Asks, func(i, j int) bool {
		return snapshot.Asks[i].Price < snapshot.Asks[j].Price
	})

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "order book retrieved successfully",
		"data":    snapshot,
	})
}

func (h *OrderHandler) GetAllOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.Storage.GetAllOrders()
	if err != nil {
		slog.Error("failed to get orders from database", "error", err)
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to get orders"))
		return
	}

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "orders retrieved successfully",
		"data":    orders,
	})
}

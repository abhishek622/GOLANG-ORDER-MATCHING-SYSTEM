package order

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage/mysql"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/utils/response"
	"github.com/go-playground/validator/v10"
)

type OrderHandler struct {
	Storage    *mysql.Mysql
	OrderBooks map[string]*OrderBook
	mu         sync.RWMutex
}

func (h *OrderHandler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req types.PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralError(err))
		return
	}

	if err := validator.New().Struct(req); err != nil {
		validatorErrors := err.(validator.ValidationErrors)
		response.WriteJson(w, http.StatusBadRequest, response.ValidationError(validatorErrors))
		return
	}

	order := &types.Order{
		Symbol:    req.Symbol,
		Side:      req.Side,
		OrderType: req.Type,
		Price:     req.Price,
		Quantity:  req.Quantity,
		Remaining: req.Quantity,
		Status:    types.OPEN,
	}

	// Store order in database
	_, err := h.Storage.PlaceOrder(*order)
	if err != nil {
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralError(err))
		return
	}

	// Process order through matching engine
	trades := h.processOrder(order)

	slog.Info("Order placed", slog.String("order_id", order.OrderID))

	response.WriteJson(w, http.StatusOK, map[string]interface{}{
		"message": "order placed successfully",
		"order": map[string]interface{}{
			"order_id": order.OrderID,
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
		"message": "order status retrieved successfully",
		"order":   order,
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

	// Remove from order book
	orderIDStr := strconv.FormatInt(orderID, 10)
	h.mu.RLock()
	for _, book := range h.OrderBooks {
		if book.RemoveOrder(orderIDStr) {
			break
		}
	}
	h.mu.RUnlock()

	// Mark as cancelled in database
	err = h.Storage.MarkOrderCancelled(orderID)
	if err != nil {
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralError(err))
		return
	}

	response.WriteJson(w, http.StatusOK, map[string]string{"message": "order cancelled successfully"})
}

func (h *OrderHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("symbol query param required"))
		return
	}

	book := h.getOrCreateOrderBook(symbol)
	snapshot := book.GetSnapshot()

	response.WriteJson(w, http.StatusOK, map[string]interface{}{
		"message": "order book snapshot retrieved successfully",
		"orderbook": map[string]interface{}{
			"symbol": symbol,
			"bids":   snapshot["bids"],
			"asks":   snapshot["asks"],
		},
	})
}

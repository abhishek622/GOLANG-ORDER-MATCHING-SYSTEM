package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/engine"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/utils/response"
	"github.com/go-playground/validator/v10"
)

type OrderHandler struct {
	engine  *engine.MatchingEngine
	storage *storage.Storage
}

func PlaceOrder(api *OrderHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Placing order")
		var order types.Order
		err := json.NewDecoder(r.Body).Decode(&order)
		if errors.Is(err, io.EOF) {
			response.WriteJson(w, http.StatusBadRequest, response.GeneralError(fmt.Errorf("empty body")))
			return
		}
		if err != nil {
			response.WriteJson(w, http.StatusBadRequest, response.GeneralError(err))
			return
		}

		if err := validator.New().Struct(order); err != nil {
			validatorErrors := err.(validator.ValidationErrors)
			response.WriteJson(w, http.StatusBadRequest, response.ValidationError(validatorErrors))
			return
		}

		orderID, err := storage.PlaceOrder(order.Symbol, order.Side, order.Type, order.Price, order.InitialQuantity, order.RemainingQuantity)
		if err != nil {
			response.WriteJson(w, http.StatusInternalServerError, response.GeneralError(err))
			return
		}

		slog.Info("Order placed successfully", slog.String("order_id", orderID))
		response.WriteJson(w, http.StatusOK, map[string]string{"message": "order placed successfully", "order_id": orderID})
	}
}

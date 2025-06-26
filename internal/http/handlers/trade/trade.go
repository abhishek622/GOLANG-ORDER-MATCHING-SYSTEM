package trade

import (
	"log/slog"
	"net/http"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/utils/response"
)

type TradeHandler struct {
	Storage storage.Storage
}

func NewTradeHandler(storage storage.Storage) *TradeHandler {
	return &TradeHandler{
		Storage: storage,
	}
}

func (h *TradeHandler) ListTrades(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("symbol is required"))
		return
	}

	slog.Info("Fetching trades for symbol", slog.String("symbol", symbol))

	trades, err := h.Storage.ListTrades(symbol)
	if err != nil {
		slog.Error("Failed to fetch trades", slog.String("error", err.Error()))
		response.WriteJson(w, http.StatusInternalServerError, response.GeneralErrorString("failed to fetch trades"))
		return
	}

	response.WriteJson(w, http.StatusOK, map[string]any{
		"message": "trades fetched successfully",
		"data":    trades,
	})
}

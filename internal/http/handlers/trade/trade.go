package trade

import (
	"log/slog"
	"net/http"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/utils/response"
)

func ListTrades(storage storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			response.WriteJson(w, http.StatusBadRequest, response.GeneralErrorString("symbol query param required"))
			return
		}

		slog.Info("Fetching trades for symbol", slog.String("symbol", symbol))

		trades, err := storage.ListTrades(symbol)
		if err != nil {
			response.WriteJson(w, http.StatusInternalServerError, response.GeneralError(err))
			return
		}

		response.WriteJson(w, http.StatusOK, map[string]interface{}{"message": "Trades fetched successfully", "trades": trades})
	}
}

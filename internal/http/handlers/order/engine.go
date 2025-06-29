package order

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
)

// processOrder and returns the executed trades
func (h *OrderHandler) processOrder(tx storage.Tx, newOrder *types.Order) ([]types.Trade, error) {
	var trades []types.Trade

	// determine opposite side for matching
	oppositeSide := types.SELL
	if newOrder.Side == types.SELL {
		oppositeSide = types.BUY
	}

	// get matching orders from opposite side
	matchingOrders, err := h.Storage.GetMatchingOrders(newOrder.Symbol, &oppositeSide)
	if err != nil {
		slog.Error("Failed to get matching orders", "error", err)
		return nil, fmt.Errorf("failed to get matching orders: %w", err)
	}

	// process matching
	for _, matchingOrder := range matchingOrders {
		// check if orders can match based on price
		if !h.canMatch(newOrder, matchingOrder) {
			// for limit orders, skip to next order if not matched
			// for market orders, will match with any opposite side order
			if newOrder.OrderType == types.LIMIT {
				continue
			}
		}

		// calculate trade, minimum of remaining quantities
		tradeQuantity := min(newOrder.Remaining, matchingOrder.Remaining)
		if tradeQuantity <= 0 {
			continue
		}

		// determine trade price
		var tradePrice int64
		if newOrder.OrderType == types.MARKET {
			// market orders always execute at the counterparty's price
			if matchingOrder.Price == nil {
				slog.Error("Matching order has no price", "orderID", matchingOrder.OrderID)
				continue
			}
			tradePrice = *matchingOrder.Price
		} else if matchingOrder.OrderType == types.MARKET {
			// if a limit order matching against a market order, use new order price
			if newOrder.Price == nil {
				slog.Error("New limit order has no price", "orderID", newOrder.OrderID)
				continue
			}
			tradePrice = *newOrder.Price
		} else {
			// both are limit orders, use the existing order's price
			if matchingOrder.Price == nil {
				slog.Error("Matching limit order has no price", "orderID", matchingOrder.OrderID)
				continue
			}
			tradePrice = *matchingOrder.Price
		}

		// create trade
		trade := types.Trade{
			Symbol:    newOrder.Symbol,
			Price:     tradePrice,
			Quantity:  tradeQuantity,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if newOrder.Side == types.BUY {
			trade.BuyOrderID = newOrder.OrderID
			trade.SellOrderID = matchingOrder.OrderID
		} else {
			trade.BuyOrderID = matchingOrder.OrderID
			trade.SellOrderID = newOrder.OrderID
		}

		_, err = h.Storage.CreateTrade(tx, trade)
		if err != nil {
			slog.Error("Failed to create trade", "error", err)
			return nil, fmt.Errorf("failed to create trade: %w", err)
		}

		newOrder.Remaining -= tradeQuantity
		matchingOrder.Remaining -= tradeQuantity

		h.updateOrderStatus(newOrder)
		h.updateOrderStatus(matchingOrder)

		// update orders in db
		if err := h.Storage.UpdateOrder(tx, newOrder.OrderID, newOrder.Remaining, newOrder.Status); err != nil {
			slog.Error("Failed to update new order", "error", err)
			return nil, fmt.Errorf("failed to update order %d: %w", newOrder.OrderID, err)
		}

		if err := h.Storage.UpdateOrder(tx, matchingOrder.OrderID, matchingOrder.Remaining, matchingOrder.Status); err != nil {
			slog.Error("Failed to update matching order", "error", err)
			return nil, fmt.Errorf("failed to update matching order %d: %w", matchingOrder.OrderID, err)
		}

		trades = append(trades, trade)

		slog.Info("Trade executed",
			"symbol", trade.Symbol,
			"price", trade.Price,
			"quantity", trade.Quantity,
			"buy_order", trade.BuyOrderID,
			"sell_order", trade.SellOrderID,
			"new_order_type", newOrder.OrderType,
			"matching_order_type", matchingOrder.OrderType)

		// if new order is fully filled, break
		if newOrder.Remaining <= 0 {
			break
		}
	}

	// for market orders that couldn't be fully filled, mark them as cancelled
	if newOrder.OrderType == types.MARKET && newOrder.Remaining > 0 {
		slog.Warn("Market order partially filled - remaining quantity will be cancelled",
			"orderID", newOrder.OrderID,
			"remaining", newOrder.Remaining,
			"original", newOrder.Quantity)

		newOrder.Status = types.CANCELLED
		if err := h.Storage.UpdateOrder(tx, newOrder.OrderID, newOrder.Remaining, newOrder.Status); err != nil {
			slog.Error("Failed to update market order status", "error", err)
			return nil, fmt.Errorf("failed to update market order %d: %w", newOrder.OrderID, err)
		}
	}

	return trades, nil
}

// check if two orders can be matched based on price
func (h *OrderHandler) canMatch(newOrder *types.Order, existingOrder *types.Order) bool {
	// Market orders can always match with limit orders
	if newOrder.OrderType == types.MARKET && (existingOrder.OrderType == types.LIMIT || existingOrder.OrderType == types.MARKET) {
		return true
	}

	// Both market orders can match
	if newOrder.OrderType == types.MARKET && existingOrder.OrderType == types.MARKET {
		return true
	}

	// For limit orders, compare prices
	if newOrder.Price == nil || existingOrder.Price == nil {
		return false
	}

	switch {
	case newOrder.Side == types.BUY && existingOrder.Side == types.SELL:
		// Buy order can match sell order if buy price >= sell price
		return *newOrder.Price >= *existingOrder.Price
	case newOrder.Side == types.SELL && existingOrder.Side == types.BUY:
		// Sell order can match buy order if sell price <= buy price
		return *newOrder.Price <= *existingOrder.Price
	}

	return false
}

func (h *OrderHandler) updateOrderStatus(order *types.Order) {
	switch {
	case order.Remaining == 0:
		order.Status = types.FILLED
	case order.Remaining < order.Quantity:
		order.Status = types.PARTIAL
	default:
		order.Status = types.OPEN
	}
}

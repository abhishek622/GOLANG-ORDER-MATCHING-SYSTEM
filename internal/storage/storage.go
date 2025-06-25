package storage

import "github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"

type Storage interface {
	PlaceOrder(order types.Order) (int64, error)
	MarkOrderFilled(order_id int64) error
	MarkOrderCancelled(order_id int64) error
	UpdateOrderStatus(orderID int64, status types.OrderStatus, remaining int64) error
	CreateTrade(trade types.Trade) (int64, error)
	ListTrades(symbol string) ([]types.Trade, error)
	GetOrderStatus(order_id int64) (*types.Order, error)
}

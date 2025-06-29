package storage

import "github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"

// Tx represents a database transaction
type Tx interface {
	Commit() error
	Rollback() error
}

type Storage interface {
	// Transaction management
	Begin() (Tx, error)

	PlaceOrder(tx Tx, order types.Order) (int64, error)
	UpdateOrder(tx Tx, orderID int64, remaining int64, status types.OrderStatus) error
	MarkOrderCancelled(tx Tx, orderID int64) error
	GetOrderStatus(orderID int64) (*types.Order, error)
	GetMatchingOrders(symbol string, side *types.OrderSide) ([]*types.Order, error)
	GetAllOrders() ([]*types.Order, error)

	CreateTrade(tx Tx, trade types.Trade) (int64, error)
	ListTrades(symbol string) ([]types.Trade, error)
}

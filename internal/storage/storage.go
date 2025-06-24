package storage

type Storage interface {
	PlaceOrder(symbol string, side string, order_type string, price float64, initial_quantity int, remaining_quantity int) (string, error)
}

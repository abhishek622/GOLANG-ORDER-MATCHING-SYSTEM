package types

import "time"

type OrderSide string
type OrderType string
type OrderStatus string

const (
	BUY  OrderSide = "buy"
	SELL OrderSide = "sell"
)

const (
	LIMIT  OrderType = "limit"
	MARKET OrderType = "market"
)

const (
	OPEN      OrderStatus = "open"
	FILLED    OrderStatus = "filled"
	PARTIAL   OrderStatus = "partial"
	CANCELLED OrderStatus = "cancelled"
)

type Order struct {
	OrderID   int64       `json:"order_id"`
	Symbol    string      `json:"symbol"`
	Side      OrderSide   `json:"side"`
	OrderType OrderType   `json:"type"`
	Price     *int64      `json:"price,omitempty"`
	Quantity  int64       `json:"quantity"`
	Remaining int64       `json:"remaining"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type Trade struct {
	TradeID     int64     `json:"trade_id"`
	Symbol      string    `json:"symbol"`
	BuyOrderID  int64     `json:"buy_order_id"`
	SellOrderID int64     `json:"sell_order_id"`
	Quantity    int64     `json:"quantity"`
	Price       int64     `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PlaceOrderRequest struct {
	Symbol   string    `json:"symbol" validate:"required"`
	Side     OrderSide `json:"side" validate:"required,oneof=buy sell"`
	Type     OrderType `json:"type" validate:"required,oneof=limit market"`
	Price    *int64    `json:"price,omitempty" validate:"required_if=Type limit,omitempty,gt=0"`
	Quantity int64     `json:"quantity" validate:"required,gt=0"`
}

type OrderBookPriceLevel struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

type OrderBookSnapshot struct {
	Symbol string                `json:"symbol"`
	Bids   []OrderBookPriceLevel `json:"bids"`
	Asks   []OrderBookPriceLevel `json:"asks"`
}

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
	OrderID   string      `json:"order_id"`
	Symbol    string      `json:"symbol"`
	Side      OrderSide   `json:"side"`
	OrderType OrderType   `json:"type"`
	Price     *float64    `json:"price,omitempty"`
	Quantity  int64       `json:"quantity"`
	Remaining int64       `json:"remaining"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type Trade struct {
	TradeID     string    `json:"trade_id"`
	Symbol      string    `json:"symbol"`
	BuyOrderID  string    `json:"buy_order_id"`
	SellOrderID string    `json:"sell_order_id"`
	Quantity    int64     `json:"quantity"`
	Price       float64   `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PlaceOrderRequest struct {
	Symbol   string    `json:"symbol" validate:"required"`
	Side     OrderSide `json:"side" validate:"required, eq=buy|eq=sell"`
	Type     OrderType `json:"type" validate:"required, eq=limit|eq=market"`
	Price    *float64  `json:"price,omitempty" validate:"required_if=Type eq=limit,gt=0"`
	Quantity int64     `json:"quantity" validate:"required,gt=0"`
}

type OrderLevel struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	Orders   int     `json:"orders"`
}
type OrderResponse struct {
	Symbol string       `json:"symbol"`
	Bids   []OrderLevel `json:"bids"`
	Asks   []OrderLevel `json:"asks"`
}

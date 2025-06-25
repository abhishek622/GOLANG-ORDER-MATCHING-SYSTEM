package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/config"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
)

type Mysql struct {
	DB *sql.DB
}

// new initializes a new MySQL database connection
func New(cfg *config.Config) (*Mysql, error) {
	db, err := sql.Open("mysql", cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// close idle connections after 1 minutes
	db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	// ping the database to verify the connection is alive
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}

	// --- creating initial tables ---
	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS orders (
            order_id BIGINT PRIMARY KEY AUTO_INCREMENT,
            symbol VARCHAR(20) NOT NULL,
            side ENUM('buy', 'sell') NOT NULL,
            type ENUM('limit', 'market') NOT NULL,
            price INT,
            quantity BIGINT NOT NULL,
            remaining BIGINT NOT NULL,
            status ENUM('open', 'filled', 'cancelled', 'partial') NOT NULL,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
        )`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'orders' table: %w", err)
	}

	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS trades (
            trade_id BIGINT PRIMARY KEY AUTO_INCREMENT,
            buy_order_id BIGINT NOT NULL,
            sell_order_id BIGINT NOT NULL,
			price INT NOT NULL,
            quantity BIGINT NOT NULL,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            FOREIGN KEY (buy_order_id) REFERENCES orders(order_id),
            FOREIGN KEY (sell_order_id) REFERENCES orders(order_id)
        )`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'trades' table: %w", err)
	}

	return &Mysql{DB: db}, nil
}

// implement the storage.Storage interface
func (m *Mysql) PlaceOrder(order types.Order) (int64, error) {
	stmt, err := m.DB.Prepare(`INSERT INTO orders (symbol, side, type, price, quantity, remaining, status, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`)
	if err != nil {
		return 0, err
	}

	defer stmt.Close()

	result, err := stmt.Exec(order.Symbol, order.Side, order.OrderType, order.Price, order.Quantity, order.Remaining, order.Status)
	if err != nil {
		return 0, err
	}

	orderId, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return orderId, nil
}

func (m *Mysql) MarkOrderFilled(order_id int64) error {
	stmt, err := m.DB.Prepare(`UPDATE orders SET quantity = 0, status = 'filled' WHERE order_id = ?`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(order_id)
	if err != nil {
		return err
	}

	return nil
}

func (m *Mysql) MarkOrderCancelled(order_id int64) error {
	stmt, err := m.DB.Prepare(`UPDATE orders SET status = 'cancelled' WHERE order_id = ? AND status IN ('open', 'partial')`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(order_id)
	if err != nil {
		return err
	}

	return nil
}

func (m *Mysql) CreateTrade(trade types.Trade) (int64, error) {
	stmt, err := m.DB.Prepare(`INSERT INTO trades (buy_order_id, sell_order_id, price, quantity, created_at, updated_at) 
		VALUES (?, ?, ?, ?, NOW(), NOW())`)
	if err != nil {
		return 0, err
	}

	defer stmt.Close()

	result, err := stmt.Exec(trade.BuyOrderID, trade.SellOrderID, trade.Price, trade.Quantity)
	if err != nil {
		return 0, err
	}

	tradeId, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return tradeId, nil
}

func (m *Mysql) ListTrades(symbol string) ([]types.Trade, error) {
	query := `SELECT t.trade_id, t.buy_order_id, t.sell_order_id, t.price, t.quantity, t.created_at, t.updated_at, o.symbol
		FROM trades t 
		JOIN orders o ON t.buy_order_id = o.order_id 
		WHERE o.symbol = ?
		ORDER BY t.created_at DESC
	`

	stmt, err := m.DB.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []types.Trade
	for rows.Next() {
		var trade types.Trade
		err := rows.Scan(
			&trade.TradeID,
			&trade.BuyOrderID,
			&trade.SellOrderID,
			&trade.Price,
			&trade.Quantity,
			&trade.CreatedAt,
			&trade.UpdatedAt,
			&trade.Symbol,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return trades, nil
}

func (m *Mysql) UpdateOrderStatus(orderID int64, status types.OrderStatus, remaining int64) error {
	stmt, err := m.DB.Prepare(`UPDATE orders SET status = ?, remaining = ?, updated_at = NOW() WHERE order_id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	result, err := stmt.Exec(status, remaining, orderID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("order not found")
	}

	return nil
}

func (m *Mysql) GetOrderStatus(order_id int64) (*types.Order, error) {
	stmt, err := m.DB.Prepare(`SELECT order_id, symbol, side, type, price, quantity, remaining, status, created_at, updated_at 
		FROM orders WHERE order_id = ?`)
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	var order types.Order
	err = stmt.QueryRow(order_id).Scan(
		&order.OrderID,
		&order.Symbol,
		&order.Side,
		&order.OrderType,
		&order.Price,
		&order.Quantity,
		&order.Remaining,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("order not found")
	}

	if err != nil {
		return nil, err
	}

	return &order, nil
}

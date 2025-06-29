package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/config"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/types"
)

// mysqlTx implements the storage.Tx interface
type mysqlTx struct {
	tx *sql.Tx
}

func (m *mysqlTx) Commit() error {
	return m.tx.Commit()
}

func (m *mysqlTx) Rollback() error {
	return m.tx.Rollback()
}

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
            symbol VARCHAR(20) NOT NULL,
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
func (m *Mysql) GetAllOrders() ([]*types.Order, error) {
	rows, err := m.DB.Query(`SELECT
	order_id, symbol, side, type, price, quantity, remaining, status, created_at, updated_at
	FROM orders ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*types.Order
	for rows.Next() {
		var order types.Order
		err := rows.Scan(&order.OrderID, &order.Symbol, &order.Side, &order.OrderType, &order.Price, &order.Quantity, &order.Remaining, &order.Status, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// Begin starts a new transaction
func (m *Mysql) Begin() (storage.Tx, error) {
	tx, err := m.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &mysqlTx{tx: tx}, nil
}

func (m *Mysql) PlaceOrder(tx storage.Tx, order types.Order) (int64, error) {
	var stmt *sql.Stmt
	var err error

	if tx != nil {
		txImpl := tx.(*mysqlTx)
		stmt, err = txImpl.tx.Prepare(`INSERT INTO orders (symbol, side, type, price, quantity, remaining, status, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`)
	} else {
		return 0, fmt.Errorf("transaction is nil")
	}

	if err != nil {
		return 0, err
	}

	defer stmt.Close()

	result, err := stmt.Exec(order.Symbol, order.Side, order.OrderType, order.Price, order.Quantity, order.Remaining, order.Status)
	if err != nil {
		return 0, err
	}

	orderID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return orderID, nil
}

func (m *Mysql) GetMatchingOrders(symbol string, side *types.OrderSide) ([]*types.Order, error) {
	var (
		query  string
		params []interface{}
	)

	// Base query
	query = `SELECT order_id, symbol, side, type, price, quantity, remaining, status, created_at, updated_at
			 FROM orders 
			 WHERE symbol = ? AND status IN ('open', 'partial')`
	params = append(params, symbol)

	// Add side condition if provided
	if side != nil {
		query += " AND side = ?"
		params = append(params, *side)

		// Set order by based on side
		switch *side {
		case types.BUY:
			query += " ORDER BY price DESC, created_at ASC"
		case types.SELL:
			query += " ORDER BY price ASC, created_at ASC"
		}
	} else {
		// Default for orderbook
		query += " ORDER BY created_at ASC"
	}

	rows, err := m.DB.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*types.Order
	for rows.Next() {
		var order types.Order
		err := rows.Scan(&order.OrderID, &order.Symbol, &order.Side, &order.OrderType, &order.Price, &order.Quantity, &order.Remaining, &order.Status, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (m *Mysql) UpdateOrder(tx storage.Tx, orderID int64, remaining int64, status types.OrderStatus) error {
	var stmt *sql.Stmt
	var err error

	if tx != nil {
		txImpl := tx.(*mysqlTx)
		stmt, err = txImpl.tx.Prepare(`UPDATE orders SET remaining = ?, status = ?, updated_at = NOW() WHERE order_id = ?`)
	} else {
		stmt, err = m.DB.Prepare(`UPDATE orders SET remaining = ?, status = ?, updated_at = NOW() WHERE order_id = ?`)
	}

	if err != nil {
		return err
	}

	defer stmt.Close()

	result, err := stmt.Exec(remaining, status, orderID)
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

func (m *Mysql) MarkOrderCancelled(tx storage.Tx, orderID int64) error {
	var stmt *sql.Stmt
	var err error

	if tx != nil {
		txImpl := tx.(*mysqlTx)
		stmt, err = txImpl.tx.Prepare(`UPDATE orders SET status = 'cancelled' WHERE order_id = ? AND status IN ('open', 'partial')`)
	} else {
		return fmt.Errorf("transaction is nil")
	}

	if err != nil {
		return err
	}

	defer stmt.Close()

	result, err := stmt.Exec(orderID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("order not found or already cancelled/filled")
	}

	return nil
}

func (m *Mysql) GetOrderStatus(order_id int64) (*types.Order, error) {
	var order types.Order
	err := m.DB.QueryRow(
		`SELECT order_id, symbol, side, type, price, quantity, remaining, status, created_at, updated_at 
		FROM orders WHERE order_id = ?`, order_id).
		Scan(&order.OrderID, &order.Symbol, &order.Side, &order.OrderType, &order.Price, &order.Quantity, &order.Remaining, &order.Status, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found")
		}
		return nil, err
	}

	return &order, nil
}

func (m *Mysql) CreateTrade(tx storage.Tx, trade types.Trade) (int64, error) {
	var stmt *sql.Stmt
	var err error

	if tx != nil {
		txImpl := tx.(*mysqlTx)
		stmt, err = txImpl.tx.Prepare(`INSERT INTO trades (symbol, buy_order_id, sell_order_id, price, quantity, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NOW(), NOW())`)
	} else {
		return 0, fmt.Errorf("transaction is nil")
	}

	if err != nil {
		return 0, err
	}

	defer stmt.Close()

	result, err := stmt.Exec(trade.Symbol, trade.BuyOrderID, trade.SellOrderID, trade.Price, trade.Quantity)
	if err != nil {
		return 0, err
	}

	tradeID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return tradeID, nil
}

func (m *Mysql) ListTrades(symbol string) ([]types.Trade, error) {
	query := `
        SELECT t.trade_id, t.symbol, t.buy_order_id, t.sell_order_id, t.price, t.quantity, t.created_at, t.updated_at
        FROM trades t
        WHERE t.symbol = ?
        ORDER BY t.created_at DESC
    `

	rows, err := m.DB.Query(query, symbol)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var trades []types.Trade
	for rows.Next() {
		var trade types.Trade
		err := rows.Scan(
			&trade.TradeID,
			&trade.Symbol,
			&trade.BuyOrderID,
			&trade.SellOrderID,
			&trade.Price,
			&trade.Quantity,
			&trade.CreatedAt,
			&trade.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		trades = append(trades, trade)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return trades, nil
}

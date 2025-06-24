package mysql

import (
	_ "github.com/go-sql-driver/mysql"
	"database/sql"
	"fmt"
	"time"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/config"
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
	defer db.Close()

	// close idle connections after 1 minutes
	db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	// ping the database to verify the connection is alive
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}

	// --- creating initial tables ---
	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS orders (
            order_id VARCHAR(36) PRIMARY KEY,
            symbol VARCHAR(10) NOT NULL,
            side ENUM('buy', 'sell') NOT NULL,
            type ENUM('limit', 'market') NOT NULL,
            price DECIMAL(10, 2) NOT NULL,
            initial_quantity INT NOT NULL,
            remaining_quantity INT NOT NULL,
            status ENUM('open', 'filled', 'partial', 'cancelled') NOT NULL,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            INDEX idx_symbol_side_price_created (symbol, side, price, created_at),
            INDEX idx_status (status)
        )`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'orders' table: %w", err)
	}

	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS trades (
            trade_id VARCHAR(36) PRIMARY KEY,
            symbol VARCHAR(10) NOT NULL,
            buy_order_id VARCHAR(36) NOT NULL,
            sell_order_id VARCHAR(36) NOT NULL,
            quantity BIGINT NOT NULL,
            price DECIMAL(10, 2) NOT NULL,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            INDEX idx_symbol_created (symbol, created_at),
            FOREIGN KEY (buy_order_id) REFERENCES orders(order_id),
            FOREIGN KEY (sell_order_id) REFERENCES orders(order_id)
        )`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'trades' table: %w", err)
	}

	return &Mysql{DB: db}, nil
}

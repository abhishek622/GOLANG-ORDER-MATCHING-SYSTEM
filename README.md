# GOLANG-ORDER-MATCHING-SYSTEM

An efficient Order Matching System built with Go and TiDB.

## Features

- Real-time order matching
- Support for order types (Market and Limit)
- Order book management
- Trade execution and reporting
- RESTful API interface

## Prerequisites

- Go 1.24 or higher
- tiDB
- Git

## Setup Instructions

### 1. Install Dependencies

```bash
go mod tidy
```

### 2. Database Setup (Using Docker)

1. Start a local TiDB cluster:

```bash
tiup playground
```

2. Using mysql CLI or GUI tools like DBeaver

3. Create the database

```bash
CREATE DATABASE IF NOT EXISTS order_matching;
```

4. Use the database

```bash
USE order_matching;
```

### 3. Run the Application

```bash
go run cmd/stock-api/main.go -config config/local.yaml
```

The application will be available at http://localhost:8082

### 4. API Documentation

### Process Limit Order

```bash
# 1. Place a sell limit order
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"sell","type":"limit","price":55,"quantity":100}'

# 2. Place a buy limit order (first)
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"buy","type":"limit","price":120,"quantity":7}'

# 3. Place another buy limit order (second)
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"buy","type":"limit","price":120,"quantity":3}'
```

### 📊 Limit Order Execution

| Side | Price | Quantity | Matches      | Orders Filled    | Partial Left           | Quantity Left |
| ---- | ----- | -------- | ------------ | ---------------- | ---------------------- | ------------- |
| Sell | 55    | 100      | None         | None             | Added to asks at 55    | 100           |
| Buy  | 120   | 7        | 100@1, 110@5 | 2 orders matched | Buy 120@1 (1 unit)     | 1             |
| Buy  | 120   | 3        | 100@1, 110@2 | 2 orders matched | Sell 110@3 (remaining) | 0             |

### Process Market Order

```bash
# 1. Place a sell market order
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"AAPL","side":"sell","type":"market","quantity":6}'

# 2. Place a buy market order
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"AAPL","side":"buy","type":"market","quantity":10}'
```

### 📈 Market Order Execution

| Side | Quantity | Matches      | Orders Filled    | Partial Left    | Quantity Left |
| ---- | -------- | ------------ | ---------------- | --------------- | ------------- |
| Sell | 6        | 90@5, 80@1   | 2 orders matched | 80@1 (bid side) | 0             |
| Buy  | 10       | 100@1, 110@5 | 2 orders matched | none            | 4 (unfilled)  |

### Cancel Order

```bash
# 1. Place a market order
curl -s -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d {"symbol":"BTC-USD","side":"sell","type":"market","quantity":100}

# Replace {order_id} with actual order ID
curl -X DELETE http://localhost:8082/api/orders/{order_id}
```

### ❌ Cancel Order Behavior

| Side | Original Price | Original Quantity | Effect on Book         |
| ---- | -------------- | ----------------- | ---------------------- |
| Sell | 100            | 1                 | Removed from asks list |

### Get Order Status

```bash
# Get status of a specific order
curl -X GET http://localhost:8082/api/orders/{order_id}
```

### Get Order Book

```bash
# Get current order book for a symbol
curl -X GET "http://localhost:8082/api/orderbook?symbol=BTC-USD"
```

### Get Trades

```bash
# Get all trades for a symbol
curl -X GET "http://localhost:8082/api/trades?symbol=BTC-USD"
```

## Design Decisions

1. **Order Matching Engine**:

   - Implemented as a separate service
   - Uses in-memory order book for performance
   - Supports multiple order types
   - Implements FIFO matching algorithm

2. **API Design**:
   - RESTful architecture
   - JSON-based communication
   - Error handling with descriptive messages

## Assumptions

1. All prices and quantities are positive numbers
2. Orders are processed in FIFO order
3. Market orders are matched immediately
4. Limit orders wait for matching price

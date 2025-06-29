# GOLANG-ORDER-MATCHING-SYSTEM

An efficient Order Matching System built with Go and TiDB.

## Features

- Order matching
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

Note: _price @ quantity_

### 📊 Limit Order Matching

```bash
# 1. Place a sell limit order
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"sell","type":"limit","price":55,"quantity":20}'

# 2. Place a buy limit order (first)
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"buy","type":"limit","price":120,"quantity":7}'

# 3. Place another buy limit order (second)
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"buy","type":"limit","price":120,"quantity":3}'
```

| Side | Price | Quantity | Matches With | Filled | Remaining | Action                     |
| ---- | ----- | -------- | ------------ | ------ | --------- | -------------------------- |
| Sell | 55    | 20       | -            | 0      | 20        | Added to asks              |
| Buy  | 120   | 7        | 55@20        | 7      | 0         | Filled against ask@55 → 13 |
| Buy  | 120   | 3        | 55@13        | 3      | 0         | Filled against ask@55 → 10 |

### 📈 Market Order Matching

```bash
# 1. Place a buy market order
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"buy","type":"market","quantity":10}'

curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"buy","type":"limit","price":120,"quantity":7}'

# 2. Place a sell market order
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USD","side":"sell","type":"market","quantity":6}'
```

### 🧾 Order Execution Summary

| Side | Type   | Price | Quantity | Matches With | Filled | Unfilled | Action                            |
| ---- | ------ | ----- | -------- | ------------ | ------ | -------- | --------------------------------- |
| Buy  | Market | -     | 10       | 55@10        | 10     | 0        | Fully filled from asks            |
| Buy  | Limit  | 120   | 7        | None         | 0      | 7        | Added to bids                     |
| Sell | Market | -     | 6        | 120@7        | 6      | 0        | Filled from limit buy (1 remains) |

### Cancel Order

```bash
# Replace {order_id} with actual order ID
curl -X DELETE http://localhost:8082/api/orders/{order_id}
```

### ❌ Cancel Order Behavior

_I have cancelled the last buy limit order_

| Side | Original Price | Original Quantity | Effect on Book         |
| ---- | -------------- | ----------------- | ---------------------- |
| Buy  | 120            | 7                 | Removed from asks list |

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

### Get All active orders

```bash
# Get current order book for a symbol
curl -X GET "http://localhost:8082/api/orders"
```

### Get Trades

```bash
# Get all trades for a symbol
curl -X GET "http://localhost:8082/api/trades?symbol=BTC-USD"
```

## Design Decisions

1. **Order Matching Engine**:

   - Implemented as a separate service
   - Uses transactions for order consistency
   - Supports multiple order types
   - Implements FIFO matching algorithm

2. **API Design**:
   - RESTful architecture
   - JSON-based communication
   - Error handling with descriptive messages
   - logs are used for debugging and monitoring

## Assumptions

1. All prices and quantities are positive numbers
2. Orders are processed in FIFO order
3. Market orders are matched immediately
4. Limit orders wait for matching price
5. For simplicity used price as int64

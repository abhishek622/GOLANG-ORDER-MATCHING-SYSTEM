# GOLANG-ORDER-MATCHING-SYSTEM

An efficient Order Matching System built with Go and TiDB, designed to handle high-frequency trading operations.

## Features

- Real-time order matching
- Support for various order types (Market, Limit, Stop)
- Order book management
- Trade execution and reporting
- RESTful API interface

## Prerequisites

- Go 1.20 or higher
- Docker and Docker Compose
- Git

## Setup Instructions

### 1. Install Dependencies

```bash
go mod download
```

### 2. Database Setup (Using Docker)

1. Start the database containers:

```bash
docker-compose up -d
```

The TiDB cluster will be available with:

- PD (Placement Driver) at port 2379
- TiKV (Distributed Key-Value Store) at port 20160
- TiDB (MySQL-compatible Database) at port 3307

### 3. Run the Application

```bash
go run cmd/stock-api/main.go -config config/local.yaml
```

The application will be available at http://localhost:8082

### 4. API Documentation

#### Submit New Order

```bash
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "AAPL",
    "side": "BUY",
    "type": "LIMIT",
    "quantity": 100,
    "price": 150.00
  }'
```

#### Get Order Status

```bash
curl http://localhost:8082/api/orders/{orderId}
```

#### Get Order Book

```bash
curl http://localhost:8082/api/orderbook?symbol={symbol}
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
5. Stop orders trigger when price condition is met

## Maintenance

### Restart Application

```bash
docker-compose restart
```

### Reset Database

```bash
docker-compose down -v
```

### View Logs

```bash
docker-compose logs -f
```

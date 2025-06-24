# GOLANG-ORDER-MATCHING-SYSTEM

Order Matching Engine

## Setup Instructions

### Using Docker (Recommended for Production)

1. Prerequisites:

   - Docker and Docker Compose installed
   - Go 1.20 or higher

2. Clone the repository:
   ```bash
   git clone [repository-url]
   cd GOLANG-ORDER-MATCHING-SYSTEM
   ```

````

3. Start the database:
   ```bash
docker-compose up -d
docker-compose logs -f db  # Check database logs
docker-compose down # stop the database
````

4. Build and run the application:
   ```bash
   go build -o matcher-api cmd/matcher-api/main.go
   ./matcher-api
   ```

````

5. The application will be available at http://localhost:8082

### Local Development Setup

1. Prerequisites:
   - MySQL 8.0+ installed locally
   - Go 1.20 or higher

2. Create the database:
   ```bash
   mysql -u root -e "CREATE DATABASE IF NOT EXISTS order_matching;"
````

3. Clone the repository:
   ```bash
   git clone [repository-url]
   cd GOLANG-ORDER-MATCHING-SYSTEM
   ```

````

4. Build and run the application:
   ```bash
go build -o matcher-api cmd/matcher-api/main.go
./matcher-api
````

5. The application will be available at http://localhost:8082

## Database Configuration

### Local Development

- Host: `127.0.0.1`
- Port: `3306`
- User: `root`
- Password: `""` (empty)
- Database: `order_matching`

### Docker Setup

The database is configured to run in Docker with these settings:

- Host: `db`
- Port: `3306`
- User: `order_matching`
- Password: `order_matching`
- Database: `order_matching`

The database data is persisted in a Docker volume named `mysql_data`, so your data will persist even if you restart the container.

## Development

### Using Docker

To stop the services:

```bash
docker-compose down
```

To rebuild the database:

```bash
docker-compose down -v  # This will remove the mysql_data volume
docker-compose up -d
```

### Local Development

To reset the database:

```bash
mysql -u root -e "DROP DATABASE order_matching; CREATE DATABASE order_matching;"
```

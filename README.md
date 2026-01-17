# Crypto Alert System

A comprehensive cryptocurrency alert system that monitors DeFi protocols and price feeds, sending alerts via email when configured thresholds are met.

## Features

- 📊 **Price Monitoring**: Monitor cryptocurrency prices using Pyth Network oracle
- 🏦 **DeFi Protocol Monitoring**: Track TVL, utilization, and liquidity across multiple protocols:
  - Aave v3
  - Morpho v1/v2 (Markets & Vaults)
  - Kamino v2 (Solana)
- 📧 **Email Alerts**: Send formatted email alerts via Resend API
- 📝 **Date-based Logging**: Automatic log rotation by date (yyyyMMdd.log format)
- 🖥️ **Web Dashboard**: React-based log viewer frontend

## Project Structure

```
crypto-alert/
├── cmd/
│   ├── main.go          # Main application
│   └── api/
│       └── main.go       # Log API server
├── internal/
│   ├── config/          # Configuration management
│   ├── core/            # Decision engine
│   ├── defi/            # DeFi protocol clients
│   ├── logger/          # Date-based logging
│   ├── message/         # Email sending
│   └── price/           # Price oracle client
├── frontend/            # React log viewer
├── logs/               # Log files (created automatically)
└── alert-rules.json    # Alert configuration
```

## Prerequisites

- Go 1.21+
- Node.js 18+ (for frontend)
- Environment variables configured (see Configuration)

## Installation

### Backend

1. Clone the repository
2. Install dependencies:
```bash
make deps
```

3. Build the application:
```bash
make build
```

### Frontend

1. Navigate to frontend directory:
```bash
cd frontend
```

2. Install dependencies:
```bash
npm install
```

## Configuration

Create a `.env` file in the root directory:

```env
# Pyth Oracle
PYTH_API_URL=https://hermes.pyth.network
PYTH_API_KEY=your_pyth_api_key

# Resend Email
RESEND_API_KEY=your_resend_api_key
RESEND_FROM_EMAIL=noreply@yourdomain.com

# Application
ALERT_RULES_FILE=alert-rules.json
CHECK_INTERVAL=60
LOG_DIR=logs

# API Server (optional)
API_PORT=8181
```

## Usage

### Running the Alert System

```bash
make run
# or
./bin/crypto-alert
```

### Running the Log API Server

```bash
make run-api
# or
./bin/log-api
```

The API server will run on port 8181 by default (configurable via `API_PORT`).

### Running the Frontend

#### Option 1: Local Development

```bash
make frontend-dev
# or
cd frontend && npm run dev
```

The frontend will be available at `http://localhost:3000`

#### Option 2: Docker (Production)

```bash
# Build and run with Docker Compose
docker-compose up -d

# Or build manually
cd frontend
docker build -t crypto-alert-frontend .
docker run -p 3000:3000 --add-host=host.docker.internal:host-gateway crypto-alert-frontend
```

The frontend will be available at `http://localhost:3000`

#### Option 3: Docker (Development with Hot Reload)

```bash
docker-compose -f docker-compose.dev.yml up
```

**Note**: When running the frontend in Docker, make sure the backend API server is running on `localhost:8181` on your host machine. The Docker container uses `host.docker.internal` to access the host's localhost.

## Alert Rules Configuration

Alert rules are configured in `alert-rules.json`. See `alert-rules.example.json` for examples.

### Price Alert Example

```json
{
  "symbol": "SOL/USD",
  "price_feed_id": "0x...",
  "threshold": 150.0,
  "direction": ">=",
  "enabled": true,
  "frequency": {
    "number": 3,
    "unit": "HOUR"
  },
  "recipient_email": "your@email.com"
}
```

### DeFi Alert Example

```json
{
  "protocol": "morpho",
  "category": "market",
  "version": "v1",
  "chain_id": "8453",
  "market_id": "0x...",
  "market_token_pair": "weETH/USDC",
  "borrow_token_contract": "0x...",
  "collateral_token_contract": "0x...",
  "field": "TVL",
  "direction": "<=",
  "threshold": 5000,
  "enabled": true,
  "frequency": {
    "unit": "ONCE"
  },
  "recipient_email": "your@email.com"
}
```

## Logging

Logs are automatically written to the `logs/` directory (or directory specified by `LOG_DIR`):
- Format: `yyyyMMdd.log` (e.g., `20260107.log`)
- New file created each day
- Logs are written to both stdout and log files

## API Endpoints

The log API server provides the following endpoints:

- `GET /api/logs/dates` - Returns array of available log dates
- `GET /api/logs/:date` - Returns log content for a specific date (format: yyyyMMdd)

## Development

### Format Code
```bash
make fmt
```

### Run Tests
```bash
make test
```

### Lint Code
```bash
make lint
```

## License

MIT

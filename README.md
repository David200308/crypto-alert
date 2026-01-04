# Crypto Alert System

A Go-based Cryptocurrency price / DeFi protocol data alert system that monitors prices from Oracle / protocol contract and sends alerts when thresholds are met.

## Project Structure

```
crypto-alert/
├── alert-rules.json
├── cmd
│   └── main.go
├── go.mod
├── go.sum
├── internal
│   ├── config
│   │   └── config.go
│   ├── core
│   │   ├── decision_test.go
│   │   └── decision.go
│   ├── defi
│   │   ├── aave
│   │   │   ├── abi
│   │   │   │   ├── erc20.json
│   │   │   │   └── pool.json
│   │   │   └── v3.go
│   │   └── morpho
│   │       ├── abi
│   │       │   ├── erc20.json
│   │       │   └── market.json
│   │       ├── market_v1.go
│   │       └── vault_v1.go
│   ├── message
│   │   ├── email_template.go
│   │   └── sender.go
│   └── price
│       └── pyth.go
├── Makefile
└── README.md
```

## Token Price Oracle Integration

- Pyth

## DeFi Integration

| DeFi Protocol | Market / Vault | Version | Chain          | TVL  | APY  | UTILIZATION | LIQUIDITY |
| ------------- | -------------- | ------- | -------------- | ---- | ---- | ----------- | --------- |
| AAVE          | Market         | V3      | ETH, Base, ARB | ✔️ | ✔️ | ✔️        | ✔️      |
| Morpho        | Market         | V1      | ETH, Base, ARB | ✔️ | ❌   | ✔️        | ✔️      |
| Morpho        | Vault          | V1      | ETH, Base, ARB | ✔️ | ✔️ | ✔️        | ✔️      |

## Installation

1. **Clone the repository**:

```bash
git clone <repository-url>
cd crypto-alert
```

2. **Install dependencies**:

```bash
go mod download
```

3. **Set up environment variables**:

```bash
cp .env.example .env ## remember to edit the .env file & alert-rules.json
```

## Usage

### Basic Usage

Run the application:

```bash
go run cmd/main.go
```

### Building

Build the binary:

```bash
go build -o crypto-alert cmd/main.go
```

Run the binary:

```bash
./crypto-alert
```

## Setting Up Alert Rules

Alert rules are configured in a JSON file (`alert-rules.json` by default). You can specify a custom path using the `ALERT_RULES_FILE` environment variable.

### JSON Format

Create an `alert-rules.json` file in the project root:

```json
[
  {
    "symbol": "BTC/USD",
    "threshold": 100000.0,
    "direction": ">=",
    "enabled": true,
    "recipient_email": "alerts@example.com"
  },
  {
    "symbol": "ETH/USD",
    "threshold": 5000.0,
    "direction": "<=",
    "enabled": true,
    "recipient_email": "alerts@example.com"
  },
  {
    "symbol": "SOL/USD",
    "threshold": 150.0,
    "direction": ">",
    "enabled": false,
    "recipient_email": "alerts@example.com"
  },
  {
    "symbol": "USDC/USD",
    "threshold": 1.0,
    "direction": "=",
    "enabled": true,
    "recipient_email": "alerts@example.com"
  }
]
```

### Rule Fields

- **symbol**: The cryptocurrency pair (e.g., "BTC/USD", "ETH/USD")
- **threshold**: The price threshold to monitor
- **direction**: Comparison operator - one of: `">="`, `">"`, `"="`, `"<="`, `"<"`
  - `">="`: Triggers when price is greater than or equal to threshold
  - `">"`: Triggers when price is greater than threshold
  - `"="`: Triggers when price equals threshold (within 0.01 tolerance)
  - `"<="`: Triggers when price is less than or equal to threshold
  - `"<"`: Triggers when price is less than threshold
- **enabled**: `true` to enable the rule, `false` to disable it
- **recipient_email**: Email address to send alerts to (required for each rule)

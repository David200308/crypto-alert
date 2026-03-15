# Crypto Alert System

A Go-based Cryptocurrency price / DeFi protocol data alert system that monitors prices from Oracle / protocol contract and sends alerts when thresholds are met.

## Tech Stack

- Backend Service: Golang
- Frontend: React
- Database: MySQL & Elastic Search
- Container: Docker
- Message Queue: Kafka
- Email Service: Resend

## Project Structure

```
crypto-alert/
├── cmd
│   ├── api
│   │   └── main.go
│   ├── main.go
│   └── notification-service
│       └── main.go
├── docker-compose.yml
├── Dockerfile
├── frontend
│   ├── dist
│   │   ├── assets
│   │   │   ├── index-C8hVq9ua.css
│   │   │   └── index-ECa1PYB9.js
│   │   ├── index.html
│   │   └── vite.svg
│   ├── Dockerfile
│   ├── index.html
│   ├── package-lock.json
│   ├── package.json
│   ├── postcss.config.js
│   ├── public
│   │   └── vite.svg
│   ├── README.md
│   ├── src
│   │   ├── App.jsx
│   │   ├── index.css
│   │   └── main.jsx
│   ├── tailwind.config.js
│   └── vite.config.js
├── go.mod
├── go.sum
├── internal
│   ├── config
│   │   └── config.go
│   ├── core
│   │   └── decision.go
│   ├── data
│   │   ├── defi
│   │   │   ├── aave
│   │   │   │   ├── abi
│   │   │   │   │   ├── erc20.json
│   │   │   │   │   └── pool.json
│   │   │   │   └── v3.go
│   │   │   ├── defi.go
│   │   │   ├── hyperliquid
│   │   │   │   └── vault_v1.go
│   │   │   ├── kamino
│   │   │   │   └── vault_v2.go
│   │   │   ├── morpho
│   │   │   │   ├── abi
│   │   │   │   │   ├── erc20.json
│   │   │   │   │   └── market.json
│   │   │   │   ├── market_v1.go
│   │   │   │   ├── vault_v1.go
│   │   │   │   └── vault_v2.go
│   │   │   └── pendle
│   │   │       └── market_v2.go
│   │   ├── prediction
│   │   │   └── polymarket
│   │   │       └── polymarket.go
│   │   └── price
│   │       └── pyth.go
│   ├── logger
│   │   ├── elasticsearch.go
│   │   └── logger.go
│   ├── message
│   │   ├── email_template.go
│   │   ├── email.go
│   │   ├── events.go
│   │   ├── kafka_publisher.go
│   │   └── telegram.go
│   ├── store
│   │   ├── elasticsearch.go
│   │   ├── logfile.go
│   │   └── mysql.go
│   └── utils
│       └── rpcutil.go
├── Makefile
├── README.md
└── sql
    └── alert_rules_schema.sql
```

## Web3 Data Integration


| Type              | Oracle | Protocol / DApp   | Market / Vault | Version | Chain          | Price | TVL  | APY  | UTILIZATION | LIQUIDITY |
| ----------------- | ------ | ----------------- | -------------- | ------- | -------------- | ----- | ---- | ---- | ----------- | --------- |
| Token             | Pyth   |                   |                |         |                | ✔️     |      |      |             |           |
| DeFi              |        | AAVE              | Market         | V3      | ETH, Base, ARB |       | ✔️    | ✔️    | ✔️           | ✔️         |
| DeFi              |        | Morpho            | Market         | V1      | ETH, Base, ARB |       | ✔️    |      | ✔️           | ✔️         |
| DeFi              |        | Morpho            | Vault          | V1      | ETH, Base, ARB |       | ✔️    | ✔️    | ✔️           | ✔️         |
| DeFi              |        | Morpho            | Vault          | V2      | ETH, Base, ARB |       | ✔️    | ✔️    | ✔️           | ✔️         |
| DeFi              |        | Kamino            | Vault          | V2      | Solana         |       | ✔️    | ✔️    | ✔️           | ✔️         |
| DeFi              |        | Pendle            | PT Market      | V2      |                |       | ✔️    | ✔️    |             |           |
| DeFi              |        | Hyperliquid Vault | Vault          |         | Hyperliquid L1 |       | ✔️    | ✔️    |             |           |
| Prediction Market |        | Polymarket        |                |         |                | ✔️     |      |      |             |           |


## Message Channel Integration


| Type  | Provider |
| ----- | -------- |
| Email | Resend   |
| Bot   | Telegram |


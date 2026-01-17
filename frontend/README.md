# Crypto Alert Log Viewer

A React-based frontend application for viewing crypto-alert logs in real-time.

## Features

- 📅 Date-based log file selection
- 🔍 Real-time log filtering and search
- 🎨 Color-coded log levels (Alert, Error, Warning, Success, Info)
- 🔄 Auto-refresh capability
- 📱 Responsive design
- 🎨 Built with Tailwind CSS for modern, responsive styling

## Prerequisites

- Node.js 24+ and npm/yarn/pnpm

## Installation

```bash
npm install
```

## Development

```bash
npm run dev
```

The app will be available at `http://localhost:3030`

## Building for Production

```bash
npm run build
```

## Docker

### Production Build

```bash
# Build Docker image
docker build -t crypto-alert-frontend .

# Run container
docker run -p 3030:3030 \
  --add-host=host.docker.internal:host-gateway \
  crypto-alert-frontend
```

### Using Docker Compose

```bash
# Production
docker-compose up -d

# Development (with hot reload)
docker-compose -f docker-compose.dev.yml up
```

**Note**: When running in Docker, the backend API server should be running on `localhost:8181` on your host machine. The container uses `host.docker.internal` to access the host's localhost.

## Backend API

This frontend expects a backend API server running on `http://localhost:8181` (or `http://host.docker.internal:8181` when in Docker) with the following endpoints:

- `GET /api/logs/dates` - Returns array of available log dates (format: ["20260107", "20260106", ...])
- `GET /api/logs/:date` - Returns log file content for a specific date (format: yyyyMMdd)

Example response for `/api/logs/20260107`:

```json
{
  "logs": [
    "2026/01/07 12:35:46   - morpho market v1 on Base (8453) (weETH/USDC): TVL",
    "2026/01/07 12:35:48 💰 morpho market v1 on Base - TVL (weETH/USDC): 6688.893693"
  ]
}
```

## Environment Variables

- `BACKEND_URL` - Backend API URL (default: `http://host.docker.internal:8181` for Docker, `http://localhost:8181` for local)

## Notes

- The backend API server needs to be running separately
- Log files are expected to be in the `logs/` directory with format `yyyyMMdd.log`
- Auto-refresh checks for new logs every 30 seconds when enabled

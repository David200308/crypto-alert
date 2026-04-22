#!/bin/sh
set -e

_secret() {
  local f="/run/secrets/$1"
  [ -f "$f" ] && cat "$f" || true
}

# Load secrets as environment variables
v=$(_secret mysql_root_password); [ -n "$v" ] && export MYSQL_ROOT_PASSWORD="$v"
v=$(_secret mysql_password);      [ -n "$v" ] && export MYSQL_PASSWORD="$v"
v=$(_secret pyth_api_key);        [ -n "$v" ] && export PYTH_API_KEY="$v"
v=$(_secret resend_api_key);      [ -n "$v" ] && export RESEND_API_KEY="$v"
v=$(_secret telegram_bot_token);  [ -n "$v" ] && export TELEGRAM_BOT_TOKEN="$v"
v=$(_secret eth_rpc_url);         [ -n "$v" ] && export ETH_RPC_URL="$v"
v=$(_secret base_rpc_url);        [ -n "$v" ] && export BASE_RPC_URL="$v"
v=$(_secret arb_rpc_url);         [ -n "$v" ] && export ARB_RPC_URL="$v"
v=$(_secret solana_rpc_url);      [ -n "$v" ] && export SOLANA_RPC_URL="$v"

# Construct MYSQL_DSN if not already provided
if [ -z "${MYSQL_DSN:-}" ] && [ -n "${MYSQL_USER:-}" ] && [ -n "${MYSQL_PASSWORD:-}" ]; then
  export MYSQL_DSN="${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(mysql:3306)/${MYSQL_DB:-web3}?parseTime=true"
fi

exec "$@"

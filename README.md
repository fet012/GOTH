# Go ETH Wallet Monitor

A zero-dependency Ethereum wallet watcher written in pure Go. Monitors one or more addresses for balance changes and incoming/outgoing transactions in real time.

## What It Teaches

| Concept | Where |
|---|---|
| JSON-RPC over HTTP | `EthClient.call()` |
| `*big.Int` for crypto math | `hexToBigInt`, `weiToEth` |
| Polling with `time.Ticker` | `WalletMonitor.Start()` |
| Struct embedding / separation of concerns | `EthClient` vs `WalletMonitor` |
| Environment-based config | `getEnv()`, `ETH_RPC_URL`, `ETH_WATCH` |
| Go error wrapping (`%w`) | Throughout |

## Run It

```bash
# Default — watches Vitalik's address on a free public RPC
go run main.go

# Watch your own address
ETH_WATCH=0xYourAddress go run main.go

# Use your own RPC (Infura, Alchemy, QuickNode, etc.)
ETH_RPC_URL=https://mainnet.infura.io/v3/YOUR_KEY ETH_WATCH=0xYourAddr go run main.go
```

## Free Public RPC Endpoints

- `https://eth.llamarpc.com` (default)
- `https://rpc.ankr.com/eth`
- `https://ethereum.publicnode.com`

## How It Works

1. On startup, fetches and prints the current balance of each watched address.
2. Every ~12 seconds (one Ethereum block), it:
   - Fetches any new blocks since the last poll.
   - Scans every transaction in those blocks for your watched addresses.
   - Prints details (direction, value, counterparty) for any match.
   - Compares current balance against previous — prints a delta if changed.

## Extending This

- Add a `--notify` flag that POSTs to a webhook (Slack, Telegram bot, etc.)
- Persist seen transactions to a SQLite/MongoDB store
- Add ERC-20 token transfer detection via `eth_getLogs` + Transfer event topic
- Build a REST API wrapper around it (perfect Go + Web3 portfolio piece)

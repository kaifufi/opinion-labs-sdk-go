# Opinion CLOB SDK - Go

[![Go Reference](https://pkg.go.dev/badge/github.com/kaifufi/opinion-labs-sdk-go.svg)](https://pkg.go.dev/github.com/kaifufi/opinion-labs-sdk-go)
Go SDK for Opinion Prediction Market CLOB API

## Installation

```bash
go get github.com/kaifufi/opinion-labs-sdk-go
```

## Requirements

- Go 1.21 or higher

## Quick Start

```go
package main

import (
    "context"
    "time"

    "github.com/kaifufi/opinion-labs-sdk-go"
)

func main() {
    config := opinionclob.ClientConfig{
        Host:          "https://api.opinionlabs.com",
        APIKey:        "your-api-key",
        ChainID:       opinionclob.ChainIDBNBMainnet,
        RPCURL:        "https://bsc-dataseed1.binance.org",
        PrivateKey:    "your-private-key",
        MultiSigAddr:  "your-multisig-address",
    }

    client, err := opinionclob.NewClient(config)
    if err != nil {
        panic(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Get markets
    markets, err := client.GetMarkets(
        opinionclob.TopicTypeAll,
        1,
        20,
        nil,
        nil,
    )

    // Place an order
    orderData := opinionclob.PlaceOrderDataInput{
        MarketID:              1,
        TokenID:               "1",
        MakerAmountInQuoteToken: stringPtr("100"),
        Price:                 "0.5",
        Side:                  opinionclob.OrderSideBuy,
        OrderType:             opinionclob.OrderTypeLimit,
    }

    result, err := client.PlaceOrder(ctx, orderData, true)
}
```

## Features

- **Market Operations**: Get markets, market details, price history, orderbooks
- **Trading Operations**: Place orders, cancel orders, batch operations
- **Position Management**: Split, merge, and redeem positions
- **User Data**: Get orders, positions, balances, trade history
- **Caching**: Built-in caching for quote tokens and market data
- **Blockchain Integration**: Full support for on-chain operations via Safe wallets

## API Reference

### Client Methods

#### Market Operations

- `GetMarkets()` - Get markets with pagination and filters
- `GetMarket()` - Get detailed market information
- `GetCategoricalMarket()` - Get categorical market details
- `GetPriceHistory()` - Get price/candlestick data
- `GetOrderbook()` - Get orderbook for a token
- `GetLatestPrice()` - Get latest token price

#### Trading Operations

- `PlaceOrder()` - Place a limit or market order
- `CancelOrder()` - Cancel an existing order
- `GetMyOrders()` - Get user's orders
- `GetOrderByID()` - Get order details

#### Position Management

- `Split()` - Split collateral into outcome tokens
- `Merge()` - Merge outcome tokens back to collateral
- `Redeem()` - Redeem winning positions after resolution
- `EnableTrading()` - Approve tokens for trading

#### User Data

- `GetMyPositions()` - Get user's positions
- `GetMyBalances()` - Get user's token balances
- `GetMyTrades()` - Get trade history
- `GetUserAuth()` - Get authenticated user info

## Configuration

The SDK supports the following configuration options:

- `Host` - API host URL
- `APIKey` - API authentication key
- `ChainID` - Blockchain chain ID (56 for BNB Chain)
- `RPCURL` - Ethereum RPC endpoint
- `PrivateKey` - Private key for signing transactions
- `MultiSigAddr` - Multi-signature wallet address
- `ConditionalTokensAddr` - Conditional tokens contract (optional, uses default)
- `MultisendAddr` - Multisend contract (optional, uses default)
- `FeeManagerAddr` - Fee manager contract (optional, uses default)
- `EnableTradingCheckInterval` - Cache interval for enable_trading checks (default: 1 hour)
- `QuoteTokensCacheTTL` - Cache TTL for quote tokens (default: 1 hour)
- `MarketCacheTTL` - Cache TTL for market data (default: 5 minutes)

## Error Handling

The SDK uses custom error types:

- `InvalidParamError` - Invalid parameter provided
- `OpenAPIError` - API request failed
- `BalanceNotEnough` - Insufficient balance
- `NoPositionsToRedeem` - No positions to redeem
- `InsufficientGasBalance` - Insufficient gas for transaction

## Examples

See the `example/` directory for complete usage examples.

## Migration from Python SDK

This Go SDK is designed to be a direct port of the Python SDK. The API surface is very similar:

**Python:**

```python
from opinion_clob_sdk import Client

client = Client(
    host="https://api.opinionlabs.com",
    apikey="your-key",
    chain_id=56,
    rpc_url="https://...",
    private_key="0x...",
    multi_sig_addr="0x...",
)

markets = client.get_markets()
```

**Go:**

```go
import "github.com/kaifufi/opinion-labs-sdk-go"

config := opinionclob.ClientConfig{
    Host: "https://api.opinionlabs.com",
    APIKey: "your-key",
    ChainID: opinionclob.ChainIDBNBMainnet,
    RPCURL: "https://...",
    PrivateKey: "0x...",
    MultiSigAddr: "0x...",
}

client, _ := opinionclob.NewClient(config)
markets, _ := client.GetMarkets(...)
```

## License

MIT License

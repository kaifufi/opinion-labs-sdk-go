// Example usage of the Opinion CLOB SDK Go
package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	opinionclob "github.com/kaifufi/opinion-labs-sdk-go"
)

func main() {
	// Initialize the SDK client
	config := opinionclob.ClientConfig{
		Host:                       "https://api.opinionlabs.com", // Replace with actual API host
		APIKey:                     "your-api-key-here",
		ChainID:                    opinionclob.ChainIDBNBMainnet,
		RPCURL:                     "https://bsc-dataseed1.binance.org", // Replace with actual RPC URL
		PrivateKey:                 "your-private-key-here",             // Replace with actual private key
		MultiSigAddr:               "your-multisig-address-here",
		ConditionalTokensAddr:      "", // Will use default
		MultisendAddr:              "", // Will use default
		FeeManagerAddr:             "", // Will use default
		EnableTradingCheckInterval: 1 * time.Hour,
		QuoteTokensCacheTTL:        1 * time.Hour,
		MarketCacheTTL:             5 * time.Minute,
	}

	client, err := opinionclob.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example: Get quote tokens
	fmt.Println("Fetching quote tokens...")
	quoteTokens, err := client.GetQuoteTokens(true)
	if err != nil {
		log.Printf("Failed to get quote tokens: %v", err)
	} else {
		fmt.Printf("Quote tokens: %+v\n", quoteTokens)
	}

	// Example: Get markets
	fmt.Println("\nFetching markets...")
	markets, err := client.GetMarkets(
		opinionclob.TopicTypeAll,
		1,
		20,
		nil,
		nil,
	)
	if err != nil {
		log.Printf("Failed to get markets: %v", err)
	} else {
		fmt.Printf("Markets: %+v\n", markets)
	}

	// Example: Get a specific market
	fmt.Println("\nFetching market details...")
	marketID := 1 // Replace with actual market ID
	market, err := client.GetMarket(marketID, true)
	if err != nil {
		log.Printf("Failed to get market: %v", err)
	} else {
		fmt.Printf("Market: %+v\n", market)
	}

	// Example: Enable trading (approve tokens)
	fmt.Println("\nEnabling trading...")
	txResult, err := client.EnableTrading(ctx)
	if err != nil {
		log.Printf("Failed to enable trading: %v", err)
	} else {
		fmt.Printf("Transaction result: %+v\n", txResult)
	}

	// Example: Place an order
	fmt.Println("\nPlacing order...")
	orderData := opinionclob.PlaceOrderDataInput{
		MarketID:                marketID,
		TokenID:                 "1",              // Replace with actual token ID
		MakerAmountInQuoteToken: stringPtr("100"), // 100 USDC
		Price:                   "0.5",
		Side:                    opinionclob.OrderSideBuy,
		OrderType:               opinionclob.OrderTypeLimit,
	}

	orderResult, err := client.PlaceOrder(ctx, orderData, true)
	if err != nil {
		log.Printf("Failed to place order: %v", err)
	} else {
		fmt.Printf("Order result: %+v\n", orderResult)
	}

	// Example: Get user's orders
	fmt.Println("\nFetching user orders...")
	orders, err := client.GetMyOrders(0, "", 10, 1)
	if err != nil {
		log.Printf("Failed to get orders: %v", err)
	} else {
		fmt.Printf("Orders: %+v\n", orders)
	}

	// Example: Split collateral
	fmt.Println("\nSplitting collateral...")
	splitAmount := big.NewInt(1000000000000000000) // 1 token (18 decimals)
	splitResult, err := client.Split(ctx, marketID, splitAmount, true)
	if err != nil {
		log.Printf("Failed to split: %v", err)
	} else {
		fmt.Printf("Split result: %+v\n", splitResult)
	}

	// Example: Get user's positions
	fmt.Println("\nFetching user positions...")
	positions, err := client.GetMyPositions(0, 1, 10)
	if err != nil {
		log.Printf("Failed to get positions: %v", err)
	} else {
		fmt.Printf("Positions: %+v\n", positions)
	}

	// Example: Get user's balances
	fmt.Println("\nFetching user balances...")
	balances, err := client.GetMyBalances()
	if err != nil {
		log.Printf("Failed to get balances: %v", err)
	} else {
		fmt.Printf("Balances: %+v\n", balances)
	}
}

func stringPtr(s string) *string {
	return &s
}

package opinionclob

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/kaifufi/opinion-clob-sdk-go/chain"
)

// Client is the main SDK client
type Client struct {
	apiClient            *APIClient
	contractCaller       *chain.ContractCaller
	chainID              ChainID
	quoteTokensCache     interface{}
	quoteTokensCacheTime time.Time
	quoteTokensCacheTTL  time.Duration
	marketCache          map[int]cacheEntry
	marketCacheTTL       time.Duration
	cacheMutex           sync.RWMutex
}

type cacheEntry struct {
	data      interface{}
	timestamp time.Time
}

// ClientConfig holds configuration for creating a Client
type ClientConfig struct {
	Host                       string
	APIKey                     string
	ChainID                    ChainID
	RPCURL                     string
	PrivateKey                 string
	MultiSigAddr               string
	ConditionalTokensAddr      string
	MultisendAddr              string
	FeeManagerAddr             string
	EnableTradingCheckInterval time.Duration
	QuoteTokensCacheTTL        time.Duration
	MarketCacheTTL             time.Duration
}

// NewClient creates a new Opinion CLOB SDK client
func NewClient(config ClientConfig) (*Client, error) {
	// Validate chain ID
	isSupported := false
	for _, supportedID := range SupportedChainIDs {
		if config.ChainID == supportedID {
			isSupported = true
			break
		}
	}
	if !isSupported {
		return nil, &InvalidParamError{
			Message: fmt.Sprintf("chain_id must be one of %v", SupportedChainIDs),
		}
	}

	// Use default contract addresses if not provided
	contracts := DefaultContractAddresses[config.ChainID]
	if config.ConditionalTokensAddr == "" {
		config.ConditionalTokensAddr = contracts.ConditionalTokens
	}
	if config.MultisendAddr == "" {
		config.MultisendAddr = contracts.Multisend
	}
	if config.FeeManagerAddr == "" {
		config.FeeManagerAddr = contracts.FeeManager
	}

	// Set default cache TTLs
	if config.QuoteTokensCacheTTL == 0 {
		config.QuoteTokensCacheTTL = 1 * time.Hour
	}
	if config.MarketCacheTTL == 0 {
		config.MarketCacheTTL = 5 * time.Minute
	}
	if config.EnableTradingCheckInterval == 0 {
		config.EnableTradingCheckInterval = 1 * time.Hour
	}

	// Create API client
	apiClient := NewAPIClient(config.Host, config.APIKey, config.ChainID)

	// Create contract caller
	contractCaller, err := chain.NewContractCaller(
		config.RPCURL,
		config.PrivateKey,
		config.MultiSigAddr,
		config.ConditionalTokensAddr,
		config.MultisendAddr,
		config.FeeManagerAddr,
		config.EnableTradingCheckInterval,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract caller: %w", err)
	}

	return &Client{
		apiClient:           apiClient,
		contractCaller:      contractCaller,
		chainID:             config.ChainID,
		quoteTokensCacheTTL: config.QuoteTokensCacheTTL,
		marketCacheTTL:      config.MarketCacheTTL,
		marketCache:         make(map[int]cacheEntry),
	}, nil
}

// Close closes the client and cleans up resources
func (c *Client) Close() {
	if c.contractCaller != nil {
		c.contractCaller.Close()
	}
}

// EnableTrading enables trading by approving necessary tokens
func (c *Client) EnableTrading(ctx context.Context) (*TransactionResult, error) {
	quoteTokenListResponse, err := c.GetQuoteTokens(true)
	if err != nil {
		return nil, err
	}

	// Parse quote token response to extract quote_token_address -> ctf_exchange_address mapping
	supportedQuoteTokens := make(map[string]string)

	for _, quoteToken := range quoteTokenListResponse.Result.List {
		quoteTokenAddress := common.HexToAddress(quoteToken.QuoteTokenAddress).Hex()
		ctfExchangeAddress := common.HexToAddress(quoteToken.CTFExchangeAddress).Hex()
		supportedQuoteTokens[quoteTokenAddress] = ctfExchangeAddress
	}

	fmt.Printf("Supported quote tokens: %v\n", supportedQuoteTokens)

	if len(supportedQuoteTokens) == 0 {
		return nil, &OpenAPIError{Message: "No supported quote tokens found"}
	}

	tx, err := c.contractCaller.EnableTrading(ctx, supportedQuoteTokens)
	if err != nil {
		return nil, err
	}

	if tx == nil {
		// No transaction needed (within check interval)
		return &TransactionResult{
			TxHash:      "0x",
			SafeTxHash:  "0x",
			ReturnValue: "",
		}, nil
	}

	return &TransactionResult{
		TxHash:      tx.Hash().Hex(),
		SafeTxHash:  "", // Would be populated from Safe transaction
		ReturnValue: "",
	}, nil
}

// Split splits collateral into outcome tokens
func (c *Client) Split(ctx context.Context, marketID int, amount *big.Int, checkApproval bool) (*TransactionResult, error) {
	if marketID <= 0 {
		return nil, &InvalidParamError{Message: "market_id must be a positive integer"}
	}
	if amount == nil || amount.Sign() <= 0 {
		return nil, &InvalidParamError{Message: "amount must be a positive integer"}
	}

	if checkApproval {
		if _, err := c.EnableTrading(ctx); err != nil {
			return nil, err
		}
	}

	market, err := c.GetMarket(marketID, true)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("get market for split: %v", err)}
	}

	// Validate chain_id matches
	marketChainID, err := strconv.Atoi(market.ChainID)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("invalid market chain_id: %s", market.ChainID)}
	}
	if ChainID(marketChainID) != c.chainID {
		return nil, &OpenAPIError{Message: "Cannot split on different chain"}
	}

	// Validate market status (must be ACTIVATED, RESOLVED, or RESOLVING)
	status := TopicStatus(market.Status)
	if status != TopicStatusActivated && status != TopicStatusResolved && status != TopicStatusResolving {
		return nil, &OpenAPIError{Message: "Cannot split on non-activated/resolving/resolved market"}
	}

	// Extract collateral (quote_token) and condition_id from market data
	collateral := common.HexToAddress(market.QuoteToken)
	conditionID, err := hex.DecodeString(market.ConditionID)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("invalid condition_id: %s", market.ConditionID)}
	}

	tx, err := c.contractCaller.Split(ctx, collateral, conditionID, amount)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("Failed to split collateral: %v", err)}
	}

	return &TransactionResult{
		TxHash:      tx.Hash().Hex(),
		SafeTxHash:  "",
		ReturnValue: "",
	}, nil
}

// Merge merges outcome tokens back into collateral
func (c *Client) Merge(ctx context.Context, marketID int, amount *big.Int, checkApproval bool) (*TransactionResult, error) {
	if marketID <= 0 {
		return nil, &InvalidParamError{Message: "market_id must be a positive integer"}
	}
	if amount == nil || amount.Sign() <= 0 {
		return nil, &InvalidParamError{Message: "amount must be a positive integer"}
	}

	if checkApproval {
		if _, err := c.EnableTrading(ctx); err != nil {
			return nil, err
		}
	}

	market, err := c.GetMarket(marketID, true)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("get market for merge: %v", err)}
	}

	// Validate chain_id matches
	marketChainID, err := strconv.Atoi(market.ChainID)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("invalid market chain_id: %s", market.ChainID)}
	}
	if ChainID(marketChainID) != c.chainID {
		return nil, &OpenAPIError{Message: "Cannot merge on different chain"}
	}

	// Validate market status (must be ACTIVATED, RESOLVED, or RESOLVING)
	status := TopicStatus(market.Status)
	if status != TopicStatusActivated && status != TopicStatusResolved && status != TopicStatusResolving {
		return nil, &OpenAPIError{Message: "Cannot merge on non-activated/resolving/resolved market"}
	}

	// Extract collateral (quote_token) and condition_id from market data
	collateral := common.HexToAddress(market.QuoteToken)
	conditionID, err := hex.DecodeString(market.ConditionID)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("invalid condition_id: %s", market.ConditionID)}
	}

	tx, err := c.contractCaller.Merge(ctx, collateral, conditionID, amount)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("Failed to merge tokens: %v", err)}
	}

	return &TransactionResult{
		TxHash:      tx.Hash().Hex(),
		SafeTxHash:  "",
		ReturnValue: "",
	}, nil
}

// Redeem redeems winning outcome tokens for collateral
func (c *Client) Redeem(ctx context.Context, marketID int, checkApproval bool) (*TransactionResult, error) {
	if marketID <= 0 {
		return nil, &InvalidParamError{Message: "market_id must be a positive integer"}
	}

	if checkApproval {
		if _, err := c.EnableTrading(ctx); err != nil {
			return nil, err
		}
	}

	market, err := c.GetMarket(marketID, true)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("get market for redeem: %v", err)}
	}

	// Validate chain_id matches
	marketChainID, err := strconv.Atoi(market.ChainID)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("invalid market chain_id: %s", market.ChainID)}
	}
	if ChainID(marketChainID) != c.chainID {
		return nil, &OpenAPIError{Message: "Cannot redeem on different chain"}
	}

	// Validate market status (must be RESOLVED for redemption)
	status := TopicStatus(market.Status)
	if status != TopicStatusResolved {
		return nil, &OpenAPIError{Message: "Cannot redeem on non-resolved market"}
	}

	// Extract collateral (quote_token) and condition_id from market data
	collateral := common.HexToAddress(market.QuoteToken)
	conditionID, err := hex.DecodeString(market.ConditionID)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("invalid condition_id: %s", market.ConditionID)}
	}

	tx, err := c.contractCaller.Redeem(ctx, collateral, conditionID)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("Failed to redeem tokens: %v", err)}
	}

	return &TransactionResult{
		TxHash:      tx.Hash().Hex(),
		SafeTxHash:  "",
		ReturnValue: "",
	}, nil
}

// GetQuoteTokens fetches the list of supported quote tokens
func (c *Client) GetQuoteTokens(useCache bool) (*GetQuoteTokensResponse, error) {
	c.cacheMutex.RLock()
	if useCache && c.quoteTokensCacheTTL > 0 {
		if c.quoteTokensCache != nil {
			cacheAge := time.Since(c.quoteTokensCacheTime)
			if cacheAge < c.quoteTokensCacheTTL {
				c.cacheMutex.RUnlock()
				if cached, ok := c.quoteTokensCache.(*GetQuoteTokensResponse); ok {
					return cached, nil
				}
			}
		}
	}
	c.cacheMutex.RUnlock()

	result, err := c.apiClient.GetQuoteTokens()
	if err != nil {
		return nil, err
	}

	c.cacheMutex.Lock()
	if c.quoteTokensCacheTTL > 0 {
		c.quoteTokensCache = result
		c.quoteTokensCacheTime = time.Now()
	}
	c.cacheMutex.Unlock()

	return result, nil
}

// GetMarkets fetches markets with pagination and filters
func (c *Client) GetMarkets(topicType TopicType, page, limit int, status *TopicStatusFilter, sortBy *TopicSortType) (*GetMarketsResponse, error) {
	if page < 1 {
		return nil, &InvalidParamError{Message: "page must be >= 1"}
	}
	if limit < 1 || limit > 20 {
		return nil, &InvalidParamError{Message: "limit must be between 1 and 20"}
	}

	return c.apiClient.GetMarkets(topicType, page, limit, status, sortBy)
}

// GetMarket fetches detailed information about a specific market
func (c *Client) GetMarket(marketID int, useCache bool) (*Market, error) {
	if marketID <= 0 {
		return nil, &InvalidParamError{Message: "market_id is required"}
	}

	c.cacheMutex.RLock()
	if useCache && c.marketCacheTTL > 0 {
		if entry, ok := c.marketCache[marketID]; ok {
			cacheAge := time.Since(entry.timestamp)
			if cacheAge < c.marketCacheTTL {
				c.cacheMutex.RUnlock()
				if market, ok := entry.data.(*Market); ok {
					return market, nil
				}
			}
		}
	}
	c.cacheMutex.RUnlock()

	result, err := c.apiClient.GetMarket(marketID)
	if err != nil {
		return nil, err
	}

	market := &result.Result.Data

	c.cacheMutex.Lock()
	if c.marketCacheTTL > 0 {
		c.marketCache[marketID] = cacheEntry{
			data:      market,
			timestamp: time.Now(),
		}
	}
	c.cacheMutex.Unlock()

	return market, nil
}

// GetCategoricalMarket fetches detailed information about a categorical market
func (c *Client) GetCategoricalMarket(marketID int) (interface{}, error) {
	if marketID <= 0 {
		return nil, &InvalidParamError{Message: "market_id is required"}
	}

	return c.apiClient.GetCategoricalMarket(marketID)
}

// GetPriceHistory fetches price history for a token
func (c *Client) GetPriceHistory(tokenID string, interval string, startAt, endAt *int64) (interface{}, error) {
	if tokenID == "" {
		return nil, &InvalidParamError{Message: "token_id is required"}
	}
	if interval == "" {
		return nil, &InvalidParamError{Message: "interval is required"}
	}

	return c.apiClient.GetPriceHistory(tokenID, interval, startAt, endAt)
}

// GetOrderbook fetches the orderbook for a token
func (c *Client) GetOrderbook(tokenID string) (interface{}, error) {
	if tokenID == "" {
		return nil, &InvalidParamError{Message: "token_id is required"}
	}

	return c.apiClient.GetOrderbook(tokenID)
}

// GetLatestPrice fetches the latest price for a token
func (c *Client) GetLatestPrice(tokenID string) (interface{}, error) {
	if tokenID == "" {
		return nil, &InvalidParamError{Message: "token_id is required"}
	}

	return c.apiClient.GetLatestPrice(tokenID)
}

// GetFeeRates fetches fee rates from FeeManager contract
func (c *Client) GetFeeRates(ctx context.Context, tokenID int) (*FeeRateSettings, error) {
	if tokenID <= 0 {
		return nil, &InvalidParamError{Message: "token_id is required"}
	}

	result, err := c.contractCaller.GetFeeRateSettings(ctx, big.NewInt(int64(tokenID)))
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("fee rate settings not available")
	}

	// Convert chain.FeeRateSettings to FeeRateSettings
	return &FeeRateSettings{
		MakerMaxFeeRate: result.MakerMaxFeeRate,
		TakerMaxFeeRate: result.TakerMaxFeeRate,
		Enabled:         result.Enabled,
	}, nil
}

// PlaceOrder places an order on the market
func (c *Client) PlaceOrder(ctx context.Context, data PlaceOrderDataInput, checkApproval bool) (interface{}, error) {
	// Enable trading first if requested
	if checkApproval {
		if _, err := c.EnableTrading(ctx); err != nil {
			return nil, err
		}
	}

	// Get quote tokens
	quoteTokenListResponse, err := c.GetQuoteTokens(true)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("failed to get quote tokens: %v", err)}
	}

	// Get market data
	market, err := c.GetMarket(data.MarketID, true)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("failed to get market: %v", err)}
	}

	// Validate chain ID matches
	marketChainID, err := strconv.Atoi(market.ChainID)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("invalid market chain_id: %s", market.ChainID)}
	}
	if ChainID(marketChainID) != c.chainID {
		return nil, &OpenAPIError{Message: "Cannot place order on different chain"}
	}

	// Find matching quote token
	quoteTokenAddr := market.QuoteToken
	var matchedQuoteToken *QuoteToken
	for i := range quoteTokenListResponse.Result.List {
		qt := &quoteTokenListResponse.Result.List[i]
		if strings.EqualFold(qt.QuoteTokenAddress, quoteTokenAddr) {
			matchedQuoteToken = qt
			break
		}
	}
	if matchedQuoteToken == nil {
		return nil, &OpenAPIError{Message: "Quote token not found for this market"}
	}

	exchangeAddr := matchedQuoteToken.CTFExchangeAddress
	currencyDecimal := matchedQuoteToken.Decimal

	// Validate based on order type and side
	// Reject if market buy and makerAmountInBaseToken is provided
	if data.Side == OrderSideBuy && data.OrderType == OrderTypeMarket && data.MakerAmountInBaseToken != nil {
		return nil, &InvalidParamError{Message: "makerAmountInBaseToken is not allowed for market buy"}
	}
	// Reject if market sell and makerAmountInQuoteToken is provided
	if data.Side == OrderSideSell && data.OrderType == OrderTypeMarket && data.MakerAmountInQuoteToken != nil {
		return nil, &InvalidParamError{Message: "makerAmountInQuoteToken is not allowed for market sell"}
	}

	// Validate price for limit orders
	if data.OrderType == OrderTypeLimit {
		priceFloat, err := strconv.ParseFloat(data.Price, 64)
		if err != nil || priceFloat <= 0 {
			return nil, &InvalidParamError{Message: fmt.Sprintf("Price must be positive for limit orders, got: %s", data.Price)}
		}
	}

	// Calculate makerAmount based on side
	var makerAmount float64
	const minimalMakerAmount = 1.0

	if data.Side == OrderSideBuy {
		if data.MakerAmountInBaseToken != nil {
			// BUY with base token amount: makerAmount = baseAmount * price
			baseAmount, err := strconv.ParseFloat(*data.MakerAmountInBaseToken, 64)
			if err != nil {
				return nil, &InvalidParamError{Message: fmt.Sprintf("invalid makerAmountInBaseToken: %s", *data.MakerAmountInBaseToken)}
			}
			if baseAmount < minimalMakerAmount {
				return nil, &InvalidParamError{Message: "makerAmountInBaseToken must be at least 1"}
			}
			priceFloat, _ := strconv.ParseFloat(data.Price, 64)
			makerAmount = baseAmount * priceFloat
		} else if data.MakerAmountInQuoteToken != nil {
			// BUY with quote token amount: use as-is
			quoteAmount, err := strconv.ParseFloat(*data.MakerAmountInQuoteToken, 64)
			if err != nil {
				return nil, &InvalidParamError{Message: fmt.Sprintf("invalid makerAmountInQuoteToken: %s", *data.MakerAmountInQuoteToken)}
			}
			if quoteAmount < minimalMakerAmount {
				return nil, &InvalidParamError{Message: "makerAmountInQuoteToken must be at least 1"}
			}
			makerAmount = quoteAmount
		} else {
			return nil, &InvalidParamError{Message: "Either makerAmountInBaseToken or makerAmountInQuoteToken must be provided for BUY orders"}
		}
	} else { // SELL
		if data.MakerAmountInBaseToken != nil {
			// SELL with base token amount: use as-is
			baseAmount, err := strconv.ParseFloat(*data.MakerAmountInBaseToken, 64)
			if err != nil {
				return nil, &InvalidParamError{Message: fmt.Sprintf("invalid makerAmountInBaseToken: %s", *data.MakerAmountInBaseToken)}
			}
			if baseAmount < minimalMakerAmount {
				return nil, &InvalidParamError{Message: "makerAmountInBaseToken must be at least 1"}
			}
			makerAmount = baseAmount
		} else if data.MakerAmountInQuoteToken != nil {
			// SELL with quote token amount: makerAmount = quoteAmount / price
			quoteAmount, err := strconv.ParseFloat(*data.MakerAmountInQuoteToken, 64)
			if err != nil {
				return nil, &InvalidParamError{Message: fmt.Sprintf("invalid makerAmountInQuoteToken: %s", *data.MakerAmountInQuoteToken)}
			}
			if quoteAmount < minimalMakerAmount {
				return nil, &InvalidParamError{Message: "makerAmountInQuoteToken must be at least 1"}
			}
			priceFloat, _ := strconv.ParseFloat(data.Price, 64)
			if priceFloat == 0 {
				return nil, &InvalidParamError{Message: "Price cannot be zero for SELL orders with makerAmountInQuoteToken"}
			}
			makerAmount = quoteAmount / priceFloat
		} else {
			return nil, &InvalidParamError{Message: "Either makerAmountInBaseToken or makerAmountInQuoteToken must be provided for SELL orders"}
		}
	}

	// Final validation: ensure makerAmount was properly calculated
	if makerAmount <= 0 {
		return nil, &InvalidParamError{Message: fmt.Sprintf("Calculated makerAmount must be positive, got: %f", makerAmount)}
	}

	// Handle market orders: set price to 0 and takerAmount to 0
	price := data.Price
	if data.OrderType == OrderTypeMarket {
		price = "0"
	}

	// Convert makerAmount to wei
	makerAmountWei, err := SafeAmountToWei(makerAmount, currencyDecimal)
	if err != nil {
		return nil, &InvalidParamError{Message: fmt.Sprintf("failed to convert makerAmount to wei: %v", err)}
	}

	// Calculate order amounts for limit orders
	var recalculatedMakerAmount, takerAmount *big.Int
	if data.OrderType == OrderTypeLimit {
		priceFloat, _ := strconv.ParseFloat(price, 64)
		recalculatedMakerAmount, takerAmount, err = CalculateOrderAmounts(
			priceFloat,
			makerAmountWei,
			data.Side,
			currencyDecimal,
		)
		if err != nil {
			return nil, err
		}
	} else {
		recalculatedMakerAmount = makerAmountWei
		takerAmount = big.NewInt(0)
	}

	// Build order data
	orderData := &chain.OrderData{
		Maker:         c.contractCaller.GetMultiSigAddress().Hex(),
		Taker:         ZeroAddress,
		TokenID:       data.TokenID,
		MakerAmount:   recalculatedMakerAmount.String(),
		TakerAmount:   takerAmount.String(),
		FeeRateBps:    "0",
		Side:          convertOrderSide(data.Side),
		SignatureType: chain.SignatureTypePolyGnosisSafe,
		Nonce:         "0",
		Signer:        c.contractCaller.GetSignerAddress().Hex(),
		Expiration:    "0",
	}

	// Build and sign order
	orderBuilder, err := chain.NewOrderBuilder(exchangeAddr, int64(c.chainID), c.contractCaller.GetPrivateKey())
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("failed to create order builder: %v", err)}
	}

	signedOrder, err := orderBuilder.BuildSignedOrder(orderData)
	if err != nil {
		return nil, &OpenAPIError{Message: fmt.Sprintf("failed to build signed order: %v", err)}
	}

	// Create order request
	orderReq := map[string]interface{}{
		"salt":             signedOrder.Order.Salt,
		"topic_id":         data.MarketID,
		"maker":            signedOrder.Order.Maker,
		"signer":           signedOrder.Order.Signer,
		"taker":            signedOrder.Order.Taker,
		"token_id":         signedOrder.Order.TokenID,
		"maker_amount":     signedOrder.Order.MakerAmount,
		"taker_amount":     signedOrder.Order.TakerAmount,
		"expiration":       signedOrder.Order.Expiration,
		"nonce":            signedOrder.Order.Nonce,
		"fee_rate_bps":     signedOrder.Order.FeeRateBps,
		"side":             signedOrder.Order.Side,
		"signature_type":   signedOrder.Order.SignatureType,
		"signature":        signedOrder.Signature,
		"sign":             signedOrder.Signature,
		"contract_address": "",
		"currency_address": quoteTokenAddr,
		"price":            price,
		"trading_method":   int(data.OrderType),
		"timestamp":        time.Now().Unix(),
		"safe_rate":        "0",
		"order_exp_time":   "0",
	}

	return c.apiClient.PlaceOrder(orderReq)
}

func getMakerAmount(data PlaceOrderDataInput) string {
	if data.MakerAmountInBaseToken != nil {
		return *data.MakerAmountInBaseToken
	}
	if data.MakerAmountInQuoteToken != nil {
		return *data.MakerAmountInQuoteToken
	}
	return "0"
}

func convertOrderSide(side OrderSide) chain.OrderSide {
	if side == OrderSideBuy {
		return chain.OrderSideBuy
	}
	return chain.OrderSideSell
}

// CancelOrder cancels an existing order
func (c *Client) CancelOrder(orderID string) (interface{}, error) {
	if orderID == "" {
		return nil, &InvalidParamError{Message: "order_id must be a non-empty string"}
	}

	return c.apiClient.CancelOrder(orderID)
}

// GetMyOrders fetches user's orders with optional filters
func (c *Client) GetMyOrders(marketID int, status string, limit, page int) (interface{}, error) {
	return c.apiClient.GetMyOrders(marketID, status, limit, page)
}

// GetOrderByID fetches detailed information about a specific order
func (c *Client) GetOrderByID(orderID string) (interface{}, error) {
	if orderID == "" {
		return nil, &InvalidParamError{Message: "order_id must be a non-empty string"}
	}

	return c.apiClient.GetOrderByID(orderID)
}

// GetMyPositions fetches user's positions
func (c *Client) GetMyPositions(marketID int, page, limit int) (interface{}, error) {
	return c.apiClient.GetMyPositions(marketID, page, limit)
}

// GetMyBalances fetches user's balances
func (c *Client) GetMyBalances() (interface{}, error) {
	return c.apiClient.GetMyBalances()
}

// GetMyTrades fetches user's trade history
func (c *Client) GetMyTrades(marketID *int, page, limit int) (interface{}, error) {
	return c.apiClient.GetMyTrades(marketID, page, limit)
}

// GetUserAuth fetches authenticated user information
func (c *Client) GetUserAuth() (interface{}, error) {
	return c.apiClient.GetUserAuth()
}

// PlaceOrdersBatch places multiple orders in batch to reduce API calls.
// If checkApproval is true, trading is enabled once for all orders.
func (c *Client) PlaceOrdersBatch(ctx context.Context, orders []PlaceOrderDataInput, checkApproval bool) ([]BatchOrderResult, error) {
	if len(orders) == 0 {
		return nil, &InvalidParamError{Message: "orders list cannot be empty"}
	}

	// Enable trading once for all orders if needed
	if checkApproval {
		if _, err := c.EnableTrading(ctx); err != nil {
			return nil, err
		}
	}

	results := make([]BatchOrderResult, 0, len(orders))

	for i, order := range orders {
		orderCopy := order                             // Create a copy for the pointer
		result, err := c.PlaceOrder(ctx, order, false) // Don't check approval again
		if err != nil {
			results = append(results, BatchOrderResult{
				Index:   i,
				Success: false,
				Error:   err.Error(),
				Order:   &orderCopy,
			})
		} else {
			results = append(results, BatchOrderResult{
				Index:   i,
				Success: true,
				Result:  result,
				Order:   &orderCopy,
			})
		}
	}

	return results, nil
}

// CancelOrdersBatch cancels multiple orders in batch.
func (c *Client) CancelOrdersBatch(orderIDs []string) ([]BatchCancelResult, error) {
	if len(orderIDs) == 0 {
		return nil, &InvalidParamError{Message: "orderIDs list cannot be empty"}
	}

	results := make([]BatchCancelResult, 0, len(orderIDs))

	for i, orderID := range orderIDs {
		result, err := c.CancelOrder(orderID)
		if err != nil {
			results = append(results, BatchCancelResult{
				Index:   i,
				Success: false,
				Error:   err.Error(),
				OrderID: orderID,
			})
		} else {
			results = append(results, BatchCancelResult{
				Index:   i,
				Success: true,
				Result:  result,
				OrderID: orderID,
			})
		}
	}

	return results, nil
}

// CancelAllOrders cancels all open orders, optionally filtered by market and/or side.
// Uses pagination to fetch all orders (max 20 per page).
func (c *Client) CancelAllOrders(marketID *int, side *OrderSide) (*CancelAllOrdersResult, error) {
	const (
		maxPageLimit = 20
		maxPages     = 100 // Safety limit to prevent infinite loops
		openStatus   = "1" // 1 = pending/open orders
	)

	// Collect all open orders using pagination
	var allOrderIDs []string
	page := 1

	for page <= maxPages {
		// Get orders for current page
		market := 0
		if marketID != nil {
			market = *marketID
		}
		pageOrders, err := c.GetMyOrders(market, openStatus, maxPageLimit, page)
		if err != nil {
			return nil, &OpenAPIError{Message: fmt.Sprintf("failed to get open orders page %d: %v", page, err)}
		}

		// Parse response to extract order list
		orders, ok := parseOrdersList(pageOrders)
		if !ok || len(orders) == 0 {
			// No more orders on this page
			break
		}

		// Filter by side if specified and extract order IDs
		for _, order := range orders {
			if side != nil {
				orderSide, sideOk := order["side"]
				if sideOk {
					// Handle both int and float64 JSON parsing
					var orderSideInt int
					switch v := orderSide.(type) {
					case float64:
						orderSideInt = int(v)
					case int:
						orderSideInt = v
					default:
						continue // Skip if side is not a valid type
					}
					if orderSideInt != int(*side) {
						continue // Skip orders that don't match the filter
					}
				}
			}

			if orderID, ok := order["order_id"].(string); ok && orderID != "" {
				allOrderIDs = append(allOrderIDs, orderID)
			}
		}

		// If we got fewer orders than the limit, we've reached the last page
		if len(orders) < maxPageLimit {
			break
		}

		page++
	}

	if len(allOrderIDs) == 0 {
		return &CancelAllOrdersResult{
			TotalOrders: 0,
			Cancelled:   0,
			Failed:      0,
			Results:     []BatchCancelResult{},
		}, nil
	}

	// Cancel all orders in batch
	results, err := c.CancelOrdersBatch(allOrderIDs)
	if err != nil {
		return nil, err
	}

	// Count successes and failures
	cancelled := 0
	failed := 0
	for _, r := range results {
		if r.Success {
			cancelled++
		} else {
			failed++
		}
	}

	return &CancelAllOrdersResult{
		TotalOrders: len(allOrderIDs),
		Cancelled:   cancelled,
		Failed:      failed,
		Results:     results,
	}, nil
}

// parseOrdersList attempts to parse the orders list from a GetMyOrders response
func parseOrdersList(response interface{}) ([]map[string]interface{}, bool) {
	// Try to parse as a map with result.list structure
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return nil, false
	}

	result, ok := respMap["result"].(map[string]interface{})
	if !ok {
		return nil, false
	}

	list, ok := result["list"].([]interface{})
	if !ok {
		return nil, false
	}

	orders := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		if order, ok := item.(map[string]interface{}); ok {
			orders = append(orders, order)
		}
	}

	return orders, true
}

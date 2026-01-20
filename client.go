package opinionclob

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/metfin/opinion-clob-sdk-go/chain"
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

	marketResponse, err := c.GetMarket(marketID, true)
	if err != nil {
		return nil, err
	}

	// TODO: Parse market response to extract condition_id and collateral
	// Parse market data (simplified)
	_ = marketResponse

	collateral := common.HexToAddress("0x0") // TODO: Extract from market data
	conditionID := []byte{}                  // TODO: Extract from market data

	tx, err := c.contractCaller.Merge(ctx, collateral, conditionID, amount)
	if err != nil {
		return nil, err
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

	marketResponse, err := c.GetMarket(marketID, true)
	if err != nil {
		return nil, err
	}

	// TODO: Parse market response to extract condition_id and collateral
	// Parse market data
	_ = marketResponse

	collateral := common.HexToAddress("0x0") // TODO: Extract from market data
	conditionID := []byte{}                  // TODO: Extract from market data

	tx, err := c.contractCaller.Redeem(ctx, collateral, conditionID)
	if err != nil {
		return nil, err
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
	// Get quote tokens
	quoteTokenListResponse, err := c.GetQuoteTokens(true)
	if err != nil {
		return nil, err
	}

	// Get market data
	marketResponse, err := c.GetMarket(data.MarketID, true)
	if err != nil {
		return nil, err
	}

	// TODO: Parse quote token response to extract exchange address and decimals
	// Parse responses (simplified)
	_ = quoteTokenListResponse
	_ = marketResponse

	// TODO: Get private key from client config instead of hardcoded empty string
	// Build order
	privateKey, err := crypto.HexToECDSA("") // Would come from config
	if err != nil {
		return nil, err
	}

	// Calculate amounts
	makerAmountFloat, err := strconv.ParseFloat(getMakerAmount(data), 64)
	if err != nil {
		return nil, err
	}

	// TODO: Get currency decimals from parsed quote token data instead of hardcoded 18
	currencyDecimal := 18 // Would come from quote token data
	makerAmountWei, err := SafeAmountToWei(makerAmountFloat, currencyDecimal)
	if err != nil {
		return nil, err
	}

	var recalculatedMakerAmount, takerAmount *big.Int
	priceFloat, err := strconv.ParseFloat(data.Price, 64)
	if err != nil {
		return nil, err
	}

	if data.OrderType == OrderTypeLimit {
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
		Expiration:    "0",
	}

	// TODO: Extract exchange address from parsed quote token data
	// Build and sign order
	exchangeAddr := "" // Would come from quote token data
	orderBuilder, err := chain.NewOrderBuilder(exchangeAddr, int64(c.chainID), privateKey)
	if err != nil {
		return nil, err
	}

	signedOrder, err := orderBuilder.BuildSignedOrder(orderData)
	if err != nil {
		return nil, err
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
		"currency_address": "", // TODO: Extract from parsed market data
		"price":            data.Price,
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

package opinionclob

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient handles HTTP requests to the Opinion CLOB API
type APIClient struct {
	host    string
	apiKey  string
	chainID ChainID
	client  *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(host, apiKey string, chainID ChainID) *APIClient {
	return &APIClient{
		host:    host,
		apiKey:  apiKey,
		chainID: chainID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request
func (c *APIClient) doRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := fmt.Sprintf("%s%s", c.host, endpoint)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// GetQuoteTokens fetches the list of supported quote tokens
// TODO: Return typed struct instead of interface{} for better type safety
func (c *APIClient) GetQuoteTokens() (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/quote-token?chain_id=%d", c.chainID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetMarkets fetches markets with pagination and filters
// TODO: Return typed struct instead of interface{} for better type safety
func (c *APIClient) GetMarkets(topicType TopicType, page, limit int, status *TopicStatusFilter, sortBy *TopicSortType) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/market?chain_id=%d&page=%d&limit=%d", c.chainID, page, limit)
	
	if topicType != TopicTypeAll {
		endpoint += fmt.Sprintf("&market_type=%d", topicType)
	}
	
	if status != nil && *status != TopicStatusFilterAll {
		endpoint += fmt.Sprintf("&status=%s", *status)
	}
	
	if sortBy != nil {
		endpoint += fmt.Sprintf("&sort_by=%d", *sortBy)
	}

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetMarket fetches detailed information about a specific market
// TODO: Return typed Market struct instead of interface{} for better type safety
func (c *APIClient) GetMarket(marketID int) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/market/%d", marketID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetCategoricalMarket fetches detailed information about a categorical market
func (c *APIClient) GetCategoricalMarket(marketID int) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/market/categorical/%d", marketID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetPriceHistory fetches price history/candlestick data for a token
func (c *APIClient) GetPriceHistory(tokenID string, interval string, startAt, endAt *int64) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/token/price-history?token_id=%s&interval=%s", tokenID, interval)
	if startAt != nil {
		endpoint += fmt.Sprintf("&start_at=%d", *startAt)
	}
	if endAt != nil {
		endpoint += fmt.Sprintf("&end_at=%d", *endAt)
	}

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetOrderbook fetches the orderbook for a specific token
func (c *APIClient) GetOrderbook(tokenID string) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/token/orderbook?token_id=%s", tokenID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetLatestPrice fetches the latest price for a token
func (c *APIClient) GetLatestPrice(tokenID string) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/token/latest-price?token_id=%s", tokenID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// PlaceOrder places an order on the market
func (c *APIClient) PlaceOrder(orderReq interface{}) (interface{}, error) {
	endpoint := "/openapi/order"
	resp, err := c.doRequest("POST", endpoint, orderReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// CancelOrder cancels an existing order
func (c *APIClient) CancelOrder(orderID string) (interface{}, error) {
	endpoint := "/openapi/order/cancel"
	reqBody := map[string]string{"order_id": orderID}
	resp, err := c.doRequest("POST", endpoint, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetMyOrders fetches user's orders with optional filters
func (c *APIClient) GetMyOrders(marketID int, status string, limit, page int) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/order?chain_id=%d&limit=%d&page=%d", c.chainID, limit, page)
	if marketID > 0 {
		endpoint += fmt.Sprintf("&market_id=%d", marketID)
	}
	if status != "" {
		endpoint += fmt.Sprintf("&status=%s", status)
	}

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetOrderByID fetches detailed information about a specific order
func (c *APIClient) GetOrderByID(orderID string) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/order/%s", orderID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetMyPositions fetches user's positions with optional filters
func (c *APIClient) GetMyPositions(marketID int, page, limit int) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/positions?chain_id=%d&page=%d&limit=%d", c.chainID, page, limit)
	if marketID > 0 {
		endpoint += fmt.Sprintf("&market_id=%d", marketID)
	}

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetMyBalances fetches user's balances
func (c *APIClient) GetMyBalances() (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/user/balance?chain_id=%d", c.chainID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetMyTrades fetches user's trade history
func (c *APIClient) GetMyTrades(marketID *int, page, limit int) (interface{}, error) {
	endpoint := fmt.Sprintf("/openapi/trade?chain_id=%d&page=%d&limit=%d", c.chainID, page, limit)
	if marketID != nil {
		endpoint += fmt.Sprintf("&market_id=%d", *marketID)
	}

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetUserAuth fetches authenticated user information
func (c *APIClient) GetUserAuth() (interface{}, error) {
	endpoint := "/openapi/user/auth"
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}


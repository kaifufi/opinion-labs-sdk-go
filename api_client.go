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

// decodeJSONResponse reads the response body, checks HTTP status, and decodes JSON
func (c *APIClient) decodeJSONResponse(resp *http.Response, result interface{}) error {
	// Read body first to check status and handle errors
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code before attempting to decode JSON
	if resp.StatusCode != http.StatusOK {
		bodyStr := string(bodyBytes)
		if bodyStr == "" {
			bodyStr = resp.Status
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodyStr)
	}

	// Decode JSON
	if err := json.Unmarshal(bodyBytes, result); err != nil {
		// If JSON decode fails, include the body in the error for debugging
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return fmt.Errorf("failed to decode JSON response: %w (body: %s)", err, bodyStr)
	}

	return nil
}

// decodeJSONResponseInterface reads the response body, checks HTTP status, and decodes JSON into interface{}
func (c *APIClient) decodeJSONResponseInterface(resp *http.Response) (interface{}, error) {
	// Read body first to check status and handle errors
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code before attempting to decode JSON
	if resp.StatusCode != http.StatusOK {
		bodyStr := string(bodyBytes)
		if bodyStr == "" {
			bodyStr = resp.Status
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodyStr)
	}

	// Decode JSON
	var result interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		// If JSON decode fails, include the body in the error for debugging
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return nil, fmt.Errorf("failed to decode JSON response: %w (body: %s)", err, bodyStr)
	}

	return result, nil
}

// GetQuoteTokens fetches the list of supported quote tokens
func (c *APIClient) GetQuoteTokens() (*GetQuoteTokensResponse, error) {
	// According to OpenAPI spec: /quoteToken with chainId as query parameter
	endpoint := fmt.Sprintf("/quoteToken?chainId=%d", c.chainID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GetQuoteTokensResponse
	if err := c.decodeJSONResponse(resp, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	return &result, nil
}

// GetMarkets fetches markets with pagination and filters
func (c *APIClient) GetMarkets(topicType TopicType, page, limit int, status *TopicStatusFilter, sortBy *TopicSortType) (*GetMarketsResponse, error) {
	endpoint := fmt.Sprintf("/market?chain_id=%d&page=%d&limit=%d", c.chainID, page, limit)

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

	var result GetMarketsResponse
	if err := c.decodeJSONResponse(resp, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	return &result, nil
}

// GetMarket fetches detailed information about a specific market
func (c *APIClient) GetMarket(marketID int) (*GetMarketResponse, error) {
	endpoint := fmt.Sprintf("/market/%d", marketID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GetMarketResponse
	if err := c.decodeJSONResponse(resp, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	return &result, nil
}

// GetCategoricalMarket fetches detailed information about a categorical market
func (c *APIClient) GetCategoricalMarket(marketID int) (interface{}, error) {
	endpoint := fmt.Sprintf("/market/categorical/%d", marketID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// GetPriceHistory fetches price history/candlestick data for a token
func (c *APIClient) GetPriceHistory(tokenID string, interval string, startAt, endAt *int64) (interface{}, error) {
	endpoint := fmt.Sprintf("/token/price-history?token_id=%s&interval=%s", tokenID, interval)
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

	return c.decodeJSONResponseInterface(resp)
}

// GetOrderbook fetches the orderbook for a specific token
func (c *APIClient) GetOrderbook(tokenID string) (interface{}, error) {
	endpoint := fmt.Sprintf("/token/orderbook?token_id=%s", tokenID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// GetLatestPrice fetches the latest price for a token
func (c *APIClient) GetLatestPrice(tokenID string) (interface{}, error) {
	endpoint := fmt.Sprintf("/token/latest-price?token_id=%s", tokenID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// PlaceOrder places an order on the market
func (c *APIClient) PlaceOrder(orderReq interface{}) (interface{}, error) {
	endpoint := "/order"
	
	// Log the request for debugging (remove sensitive fields in production)
	if reqMap, ok := orderReq.(map[string]interface{}); ok {
		// Create a copy for logging (remove signature)
		logReq := make(map[string]interface{})
		for k, v := range reqMap {
			if k == "signature" || k == "sign" {
				logReq[k] = "[REDACTED]"
			} else {
				logReq[k] = v
			}
		}
		// This will help debug what's being sent
		_ = logReq
	}
	
	resp, err := c.doRequest("POST", endpoint, orderReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// CancelOrder cancels an existing order
func (c *APIClient) CancelOrder(orderID string) (interface{}, error) {
	endpoint := "/order/cancel"
	reqBody := map[string]string{"order_id": orderID}
	resp, err := c.doRequest("POST", endpoint, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// GetMyOrders fetches user's orders with optional filters
func (c *APIClient) GetMyOrders(marketID int, status string, limit, page int) (interface{}, error) {
	endpoint := fmt.Sprintf("/order?chain_id=%d&limit=%d&page=%d", c.chainID, limit, page)
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

	return c.decodeJSONResponseInterface(resp)
}

// GetOrderByID fetches detailed information about a specific order
func (c *APIClient) GetOrderByID(orderID string) (interface{}, error) {
	endpoint := fmt.Sprintf("/order/%s", orderID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// GetMyPositions fetches user's positions with optional filters
func (c *APIClient) GetMyPositions(marketID int, page, limit int) (interface{}, error) {
	endpoint := fmt.Sprintf("/positions?chain_id=%d&page=%d&limit=%d", c.chainID, page, limit)
	if marketID > 0 {
		endpoint += fmt.Sprintf("&market_id=%d", marketID)
	}

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// GetMyBalances fetches user's balances
func (c *APIClient) GetMyBalances() (interface{}, error) {
	endpoint := fmt.Sprintf("/user/balance?chain_id=%d", c.chainID)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// GetMyTrades fetches user's trade history
func (c *APIClient) GetMyTrades(marketID *int, page, limit int) (interface{}, error) {
	endpoint := fmt.Sprintf("/trade?chain_id=%d&page=%d&limit=%d", c.chainID, page, limit)
	if marketID != nil {
		endpoint += fmt.Sprintf("&market_id=%d", *marketID)
	}

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.decodeJSONResponseInterface(resp)
}

// GetUserAuth fetches authenticated user information
func (c *APIClient) GetUserAuth() (interface{}, error) {
	endpoint := "/user/auth"
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

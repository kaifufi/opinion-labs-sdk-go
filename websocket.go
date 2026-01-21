package opinionclob

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// WebSocket endpoint
	DefaultWSEndpoint = "wss://ws.opinion.trade"

	// Heartbeat interval
	HeartbeatInterval = 30 * time.Second

	// Reconnect settings
	DefaultReconnectInterval    = 5 * time.Second
	DefaultMaxReconnectAttempts = 10
)

// WebSocket action types
const (
	ActionHeartbeat   = "HEARTBEAT"
	ActionSubscribe   = "SUBSCRIBE"
	ActionUnsubscribe = "UNSUBSCRIBE"
)

// WebSocket channel types
const (
	ChannelOrderUpdate     = "trade.order.update"
	ChannelTradeRecord     = "trade.record.new"
	ChannelMarketDepthDiff = "market.depth.diff"
	ChannelMarketLastPrice = "market.last.price"
	ChannelMarketLastTrade = "market.last.trade"
)

// WSMessage represents a generic WebSocket message
type WSMessage struct {
	Action string `json:"action"`
}

// SubscribeBinaryMessage represents a subscription message for binary markets
type SubscribeBinaryMessage struct {
	Action   string `json:"action"`
	Channel  string `json:"channel"`
	MarketID int    `json:"marketId"`
}

// SubscribeCategoricalMessage represents a subscription message for categorical markets
type SubscribeCategoricalMessage struct {
	Action       string `json:"action"`
	Channel      string `json:"channel"`
	RootMarketID int    `json:"rootMarketId"`
}

// HeartbeatMessage represents a heartbeat message
type HeartbeatMessage struct {
	Action string `json:"action"`
}

// OrderUpdate represents an order update message from WebSocket
type OrderUpdate struct {
	OrderUpdateType string `json:"orderUpdateType"` // e.g., "orderConfirm"
	MarketID        int    `json:"marketId"`
	RootMarketID    int    `json:"rootMarketId"`
	OrderID         string `json:"orderId"`
	Side            int    `json:"side"`
	OutcomeSide     int    `json:"outcomeSide"`
	Price           string `json:"price"`
	Shares          string `json:"shares"`
	Amount          string `json:"amount"`
	Status          int    `json:"status"`
	TradingMethod   int    `json:"tradingMethod"`
	QuoteToken      string `json:"quoteToken"`
	CreatedAt       int64  `json:"createdAt"`
	ExpiresAt       int64  `json:"expiresAt"`
	ChainID         string `json:"chainId"`
	FilledShares    string `json:"filledShares"`
	FilledAmount    string `json:"filledAmount"`
	MsgType         string `json:"msgType"`
}

// TradeRecord represents a trade execution message from WebSocket
type TradeRecord struct {
	OrderID            string `json:"orderId"`
	TradeNo            string `json:"tradeNo"`
	MarketID           int    `json:"marketId"`
	RootMarketID       int    `json:"rootMarketId"`
	TxHash             string `json:"txHash"`
	Side               string `json:"side"` // "Buy" or "Sell"
	OutcomeSide        int    `json:"outcomeSide"`
	Price              string `json:"price"`
	Shares             string `json:"shares"`
	Amount             string `json:"amount"`
	Profit             string `json:"profit"`
	Status             int    `json:"status"`
	QuoteToken         string `json:"quoteToken"`
	QuoteTokenUsdPrice string `json:"quoteTokenUsdPrice"`
	UsdAmount          string `json:"usdAmount"`
	Fee                string `json:"fee"`
	ChainID            string `json:"chainId"`
	CreatedAt          int64  `json:"createdAt"`
	MsgType            string `json:"msgType"`
}

// MarketDepthDiff represents an orderbook change message from WebSocket
type MarketDepthDiff struct {
	MarketID    int    `json:"marketId"`
	TokenID     string `json:"tokenId"`
	OutcomeSide int    `json:"outcomeSide"`
	Side        string `json:"side"` // "bids" or "asks"
	Price       string `json:"price"`
	Size        string `json:"size"`
	MsgType     string `json:"msgType"`
}

// MarketLastPrice represents a market price change message from WebSocket
type MarketLastPrice struct {
	TokenID     string `json:"tokenId"`
	OutcomeSide int    `json:"outcomeSide"`
	Price       string `json:"price"`
	MarketID    int    `json:"marketId"`
	MsgType     string `json:"msgType"`
}

// MarketLastTrade represents a market last trade message from WebSocket
type MarketLastTrade struct {
	TokenID     string `json:"tokenId"`
	Side        string `json:"side"` // "Buy" or "Sell"
	OutcomeSide int    `json:"outcomeSide"`
	Price       string `json:"price"`
	Shares      string `json:"shares"`
	Amount      string `json:"amount"`
	MarketID    int    `json:"marketId"`
	MsgType     string `json:"msgType"`
}

// WSEventHandler is a callback function for handling WebSocket events
type WSEventHandler func(messageType int, data []byte)

// WSErrorHandler is a callback function for handling WebSocket errors
type WSErrorHandler func(err error)

// WSConfig holds configuration for the WebSocket client
type WSConfig struct {
	Endpoint             string
	APIKey               string
	ReconnectInterval    time.Duration
	MaxReconnectAttempts int
	OnMessage            WSEventHandler
	OnError              WSErrorHandler
	OnConnect            func()
	OnDisconnect         func()
}

// WSClient is the WebSocket client for Opinion Labs
type WSClient struct {
	config           WSConfig
	conn             *websocket.Conn
	mu               sync.RWMutex
	isConnected      bool
	subscriptions    map[string]interface{} // Track active subscriptions for reconnection
	subMu            sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	heartbeatTicker  *time.Ticker
	reconnectAttempt int
	done             chan struct{}
}

// NewWSClient creates a new WebSocket client
func NewWSClient(config WSConfig) *WSClient {
	if config.Endpoint == "" {
		config.Endpoint = DefaultWSEndpoint
	}
	if config.ReconnectInterval == 0 {
		config.ReconnectInterval = DefaultReconnectInterval
	}
	if config.MaxReconnectAttempts == 0 {
		config.MaxReconnectAttempts = DefaultMaxReconnectAttempts
	}

	return &WSClient{
		config:        config,
		subscriptions: make(map[string]interface{}),
		done:          make(chan struct{}),
	}
}

// Connect establishes a WebSocket connection
func (ws *WSClient) Connect(ctx context.Context) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.isConnected {
		return nil
	}

	ws.ctx, ws.cancel = context.WithCancel(ctx)

	// Build WebSocket URL with API key
	u, err := url.Parse(ws.config.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse WebSocket endpoint: %w", err)
	}
	q := u.Query()
	q.Set("apikey", ws.config.APIKey)
	u.RawQuery = q.Encode()

	// Establish connection
	conn, _, err := websocket.DefaultDialer.DialContext(ws.ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	ws.conn = conn
	ws.isConnected = true
	ws.reconnectAttempt = 0

	// Start heartbeat
	ws.startHeartbeat()

	// Start message reader
	go ws.readLoop()

	if ws.config.OnConnect != nil {
		go ws.config.OnConnect()
	}

	return nil
}

// Disconnect closes the WebSocket connection
func (ws *WSClient) Disconnect() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	return ws.disconnect()
}

// disconnect is the internal disconnect method (must be called with lock held)
func (ws *WSClient) disconnect() error {
	if !ws.isConnected {
		return nil
	}

	ws.isConnected = false

	if ws.cancel != nil {
		ws.cancel()
	}

	if ws.heartbeatTicker != nil {
		ws.heartbeatTicker.Stop()
	}

	var err error
	if ws.conn != nil {
		err = ws.conn.Close()
		ws.conn = nil
	}

	if ws.config.OnDisconnect != nil {
		go ws.config.OnDisconnect()
	}

	return err
}

// IsConnected returns the current connection status
func (ws *WSClient) IsConnected() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.isConnected
}

// SubscribeBinary subscribes to a binary market channel
func (ws *WSClient) SubscribeBinary(channel string, marketID int) error {
	msg := SubscribeBinaryMessage{
		Action:   ActionSubscribe,
		Channel:  channel,
		MarketID: marketID,
	}

	if err := ws.sendMessage(msg); err != nil {
		return err
	}

	// Track subscription for reconnection
	ws.subMu.Lock()
	key := fmt.Sprintf("binary:%s:%d", channel, marketID)
	ws.subscriptions[key] = msg
	ws.subMu.Unlock()

	return nil
}

// SubscribeCategorical subscribes to a categorical market channel
func (ws *WSClient) SubscribeCategorical(channel string, rootMarketID int) error {
	msg := SubscribeCategoricalMessage{
		Action:       ActionSubscribe,
		Channel:      channel,
		RootMarketID: rootMarketID,
	}

	if err := ws.sendMessage(msg); err != nil {
		return err
	}

	// Track subscription for reconnection
	ws.subMu.Lock()
	key := fmt.Sprintf("categorical:%s:%d", channel, rootMarketID)
	ws.subscriptions[key] = msg
	ws.subMu.Unlock()

	return nil
}

// UnsubscribeBinary unsubscribes from a binary market channel
func (ws *WSClient) UnsubscribeBinary(channel string, marketID int) error {
	msg := SubscribeBinaryMessage{
		Action:   ActionUnsubscribe,
		Channel:  channel,
		MarketID: marketID,
	}

	if err := ws.sendMessage(msg); err != nil {
		return err
	}

	// Remove from subscriptions
	ws.subMu.Lock()
	key := fmt.Sprintf("binary:%s:%d", channel, marketID)
	delete(ws.subscriptions, key)
	ws.subMu.Unlock()

	return nil
}

// UnsubscribeCategorical unsubscribes from a categorical market channel
func (ws *WSClient) UnsubscribeCategorical(channel string, rootMarketID int) error {
	msg := SubscribeCategoricalMessage{
		Action:       ActionUnsubscribe,
		Channel:      channel,
		RootMarketID: rootMarketID,
	}

	if err := ws.sendMessage(msg); err != nil {
		return err
	}

	// Remove from subscriptions
	ws.subMu.Lock()
	key := fmt.Sprintf("categorical:%s:%d", channel, rootMarketID)
	delete(ws.subscriptions, key)
	ws.subMu.Unlock()

	return nil
}

// SubscribeOrderUpdateBinary subscribes to order updates for a binary market
func (ws *WSClient) SubscribeOrderUpdateBinary(marketID int) error {
	return ws.SubscribeBinary(ChannelOrderUpdate, marketID)
}

// SubscribeOrderUpdateCategorical subscribes to order updates for a categorical market
func (ws *WSClient) SubscribeOrderUpdateCategorical(rootMarketID int) error {
	return ws.SubscribeCategorical(ChannelOrderUpdate, rootMarketID)
}

// UnsubscribeOrderUpdateBinary unsubscribes from order updates for a binary market
func (ws *WSClient) UnsubscribeOrderUpdateBinary(marketID int) error {
	return ws.UnsubscribeBinary(ChannelOrderUpdate, marketID)
}

// UnsubscribeOrderUpdateCategorical unsubscribes from order updates for a categorical market
func (ws *WSClient) UnsubscribeOrderUpdateCategorical(rootMarketID int) error {
	return ws.UnsubscribeCategorical(ChannelOrderUpdate, rootMarketID)
}

// SubscribeTradeRecordBinary subscribes to trade records for a binary market
func (ws *WSClient) SubscribeTradeRecordBinary(marketID int) error {
	return ws.SubscribeBinary(ChannelTradeRecord, marketID)
}

// SubscribeTradeRecordCategorical subscribes to trade records for a categorical market
func (ws *WSClient) SubscribeTradeRecordCategorical(rootMarketID int) error {
	return ws.SubscribeCategorical(ChannelTradeRecord, rootMarketID)
}

// UnsubscribeTradeRecordBinary unsubscribes from trade records for a binary market
func (ws *WSClient) UnsubscribeTradeRecordBinary(marketID int) error {
	return ws.UnsubscribeBinary(ChannelTradeRecord, marketID)
}

// UnsubscribeTradeRecordCategorical unsubscribes from trade records for a categorical market
func (ws *WSClient) UnsubscribeTradeRecordCategorical(rootMarketID int) error {
	return ws.UnsubscribeCategorical(ChannelTradeRecord, rootMarketID)
}

// SubscribeMarketDepthDiff subscribes to orderbook changes for a binary market
func (ws *WSClient) SubscribeMarketDepthDiff(marketID int) error {
	return ws.SubscribeBinary(ChannelMarketDepthDiff, marketID)
}

// UnsubscribeMarketDepthDiff unsubscribes from orderbook changes for a binary market
func (ws *WSClient) UnsubscribeMarketDepthDiff(marketID int) error {
	return ws.UnsubscribeBinary(ChannelMarketDepthDiff, marketID)
}

// SubscribeMarketLastPriceBinary subscribes to market price changes for a binary market
func (ws *WSClient) SubscribeMarketLastPriceBinary(marketID int) error {
	return ws.SubscribeBinary(ChannelMarketLastPrice, marketID)
}

// SubscribeMarketLastPriceCategorical subscribes to market price changes for a categorical market
func (ws *WSClient) SubscribeMarketLastPriceCategorical(rootMarketID int) error {
	return ws.SubscribeCategorical(ChannelMarketLastPrice, rootMarketID)
}

// UnsubscribeMarketLastPriceBinary unsubscribes from market price changes for a binary market
func (ws *WSClient) UnsubscribeMarketLastPriceBinary(marketID int) error {
	return ws.UnsubscribeBinary(ChannelMarketLastPrice, marketID)
}

// UnsubscribeMarketLastPriceCategorical unsubscribes from market price changes for a categorical market
func (ws *WSClient) UnsubscribeMarketLastPriceCategorical(rootMarketID int) error {
	return ws.UnsubscribeCategorical(ChannelMarketLastPrice, rootMarketID)
}

// SubscribeMarketLastTradeBinary subscribes to market last trade for a binary market
func (ws *WSClient) SubscribeMarketLastTradeBinary(marketID int) error {
	return ws.SubscribeBinary(ChannelMarketLastTrade, marketID)
}

// SubscribeMarketLastTradeCategorical subscribes to market last trade for a categorical market
func (ws *WSClient) SubscribeMarketLastTradeCategorical(rootMarketID int) error {
	return ws.SubscribeCategorical(ChannelMarketLastTrade, rootMarketID)
}

// UnsubscribeMarketLastTradeBinary unsubscribes from market last trade for a binary market
func (ws *WSClient) UnsubscribeMarketLastTradeBinary(marketID int) error {
	return ws.UnsubscribeBinary(ChannelMarketLastTrade, marketID)
}

// UnsubscribeMarketLastTradeCategorical unsubscribes from market last trade for a categorical market
func (ws *WSClient) UnsubscribeMarketLastTradeCategorical(rootMarketID int) error {
	return ws.UnsubscribeCategorical(ChannelMarketLastTrade, rootMarketID)
}

// sendMessage sends a message over the WebSocket connection
func (ws *WSClient) sendMessage(msg interface{}) error {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if !ws.isConnected || ws.conn == nil {
		return fmt.Errorf("WebSocket not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := ws.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// startHeartbeat starts the heartbeat ticker
func (ws *WSClient) startHeartbeat() {
	ws.heartbeatTicker = time.NewTicker(HeartbeatInterval)

	go func() {
		for {
			select {
			case <-ws.heartbeatTicker.C:
				if err := ws.sendHeartbeat(); err != nil {
					if ws.config.OnError != nil {
						ws.config.OnError(fmt.Errorf("heartbeat failed: %w", err))
					}
				}
			case <-ws.ctx.Done():
				return
			}
		}
	}()
}

// sendHeartbeat sends a heartbeat message
func (ws *WSClient) sendHeartbeat() error {
	return ws.sendMessage(HeartbeatMessage{Action: ActionHeartbeat})
}

// readLoop continuously reads messages from the WebSocket
func (ws *WSClient) readLoop() {
	for {
		select {
		case <-ws.ctx.Done():
			return
		default:
			ws.mu.RLock()
			conn := ws.conn
			ws.mu.RUnlock()

			if conn == nil {
				return
			}

			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					ws.handleDisconnect()
					return
				}
				if ws.config.OnError != nil {
					ws.config.OnError(fmt.Errorf("read error: %w", err))
				}
				ws.handleDisconnect()
				return
			}

			if ws.config.OnMessage != nil {
				ws.config.OnMessage(messageType, data)
			}
		}
	}
}

// handleDisconnect handles disconnection and attempts reconnection
func (ws *WSClient) handleDisconnect() {
	ws.mu.Lock()
	wasConnected := ws.isConnected
	ws.isConnected = false
	if ws.heartbeatTicker != nil {
		ws.heartbeatTicker.Stop()
	}
	ws.mu.Unlock()

	if wasConnected && ws.config.OnDisconnect != nil {
		ws.config.OnDisconnect()
	}

	// Attempt reconnection
	go ws.attemptReconnect()
}

// attemptReconnect attempts to reconnect to the WebSocket
func (ws *WSClient) attemptReconnect() {
	for ws.reconnectAttempt < ws.config.MaxReconnectAttempts {
		ws.reconnectAttempt++

		select {
		case <-ws.ctx.Done():
			return
		case <-time.After(ws.config.ReconnectInterval):
		}

		// Create a new context for reconnection
		ctx := context.Background()
		if err := ws.Connect(ctx); err != nil {
			if ws.config.OnError != nil {
				ws.config.OnError(fmt.Errorf("reconnect attempt %d failed: %w", ws.reconnectAttempt, err))
			}
			continue
		}

		// Resubscribe to all channels
		ws.resubscribe()
		return
	}

	if ws.config.OnError != nil {
		ws.config.OnError(fmt.Errorf("max reconnect attempts (%d) reached", ws.config.MaxReconnectAttempts))
	}
}

// resubscribe resubscribes to all tracked subscriptions
func (ws *WSClient) resubscribe() {
	ws.subMu.RLock()
	defer ws.subMu.RUnlock()

	for _, msg := range ws.subscriptions {
		if err := ws.sendMessage(msg); err != nil {
			if ws.config.OnError != nil {
				ws.config.OnError(fmt.Errorf("resubscribe failed: %w", err))
			}
		}
	}
}

// GetSubscriptions returns a list of current subscriptions
func (ws *WSClient) GetSubscriptions() []string {
	ws.subMu.RLock()
	defer ws.subMu.RUnlock()

	subs := make([]string, 0, len(ws.subscriptions))
	for key := range ws.subscriptions {
		subs = append(subs, key)
	}
	return subs
}

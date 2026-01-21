package opinionclob

// TopicStatus represents the status of a market topic
type TopicStatus int

const (
	TopicStatusCreated TopicStatus = iota + 1
	TopicStatusActivated
	TopicStatusResolving
	TopicStatusResolved
	TopicStatusFailed
	TopicStatusDeleted
)

// TopicType represents the type of market
type TopicType int

const (
	TopicTypeBinary TopicType = iota
	TopicTypeCategorical
	TopicTypeAll
)

// TopicStatusFilter represents filter options for topic status
type TopicStatusFilter string

const (
	TopicStatusFilterAll       TopicStatusFilter = ""
	TopicStatusFilterActivated TopicStatusFilter = "activated"
	TopicStatusFilterResolved  TopicStatusFilter = "resolved"
)

// TopicSortType represents sort options for market queries
type TopicSortType int

const (
	TopicSortTypeNoSort TopicSortType = iota
	TopicSortTypeByTimeDesc
	TopicSortTypeByCutoffTimeAsc
	TopicSortTypeByVolumeDesc
	TopicSortTypeByVolumeAsc
	TopicSortTypeByVolume24HDesc
	TopicSortTypeByVolume24HAsc
	TopicSortTypeByVolume7DDesc
	TopicSortTypeByVolume7DAsc
)

// OrderSide represents the side of an order
type OrderSide int

const (
	OrderSideBuy OrderSide = iota
	OrderSideSell
)

// OrderType represents the type of order
type OrderType int

const (
	OrderTypeMarket OrderType = iota + 1
	OrderTypeLimit
)

// SignatureType represents the signature type for orders
type SignatureType int

const (
	SignatureTypeEOA SignatureType = iota
	SignatureTypePolyGnosisSafe
	SignatureTypePolyProxy
)

// TransactionResult represents the result of a blockchain transaction
type TransactionResult struct {
	TxHash      string
	SafeTxHash  string
	ReturnValue string
}

// PlaceOrderDataInput represents input data for placing an order
type PlaceOrderDataInput struct {
	MarketID                int
	TokenID                 string
	MakerAmountInQuoteToken *string // Optional: amount in quote token (e.g., USDC)
	MakerAmountInBaseToken  *string // Optional: amount in base token (e.g., YES token)
	Price                   string
	Side                    OrderSide
	OrderType               OrderType
}

// OrderData represents the data for building an order
type OrderData struct {
	Maker         string
	Taker         string
	TokenID       string
	MakerAmount   string
	TakerAmount   string
	Side          OrderSide
	FeeRateBps    string
	Nonce         string
	Signer        string
	Expiration    string
	SignatureType SignatureType
}

// SignedOrder represents an order with its signature
type SignedOrder struct {
	Order     *Order
	Signature string
}

// Order represents an EIP712 order structure
type Order struct {
	Salt          string
	Maker         string
	Signer        string
	Taker         string
	TokenID       string
	MakerAmount   string
	TakerAmount   string
	Expiration    string
	Nonce         string
	FeeRateBps    string
	Side          string
	SignatureType string
}

// FeeRateSettings represents fee rate settings from the FeeManager contract
type FeeRateSettings struct {
	MakerMaxFeeRate float64
	TakerMaxFeeRate float64
	Enabled         bool
}

// ChildMarket represents a child market within a categorical market
type ChildMarket struct {
	MarketID      int    `json:"marketId"`
	MarketTitle   string `json:"marketTitle"`
	Status        int    `json:"status"`
	StatusEnum    string `json:"statusEnum"`
	YesLabel      string `json:"yesLabel"`
	NoLabel       string `json:"noLabel"`
	Rules         string `json:"rules"`
	YesTokenID    string `json:"yesTokenId"`
	NoTokenID     string `json:"noTokenId"`
	ConditionID   string `json:"conditionId"`
	ResultTokenID string `json:"resultTokenId"`
	Volume        string `json:"volume"`
	QuoteToken    string `json:"quoteToken"`
	ChainID       string `json:"chainId"`
	QuestionID    string `json:"questionId"`
	CreatedAt     int64  `json:"createdAt"`
	CutoffAt      int64  `json:"cutoffAt"`
	ResolvedAt    int64  `json:"resolvedAt"`
}

// Market represents detailed market information
type Market struct {
	MarketID        int                    `json:"marketId"`
	MarketTitle     string                 `json:"marketTitle"`
	Status          int                    `json:"status"`
	StatusEnum      string                 `json:"statusEnum"`
	MarketType      int                    `json:"marketType"`
	ChildMarkets    []ChildMarket          `json:"childMarkets"`
	YesLabel        string                 `json:"yesLabel"`
	NoLabel         string                 `json:"noLabel"`
	Rules           string                 `json:"rules"`
	YesTokenID      string                 `json:"yesTokenId"`
	NoTokenID       string                 `json:"noTokenId"`
	ConditionID     string                 `json:"conditionId"`
	ResultTokenID   string                 `json:"resultTokenId"`
	Volume          string                 `json:"volume"`
	Volume24H       string                 `json:"volume24h"`
	Volume7D        string                 `json:"volume7d"`
	QuoteToken      string                 `json:"quoteToken"`
	ChainID         string                 `json:"chainId"`
	QuestionID      string                 `json:"questionId"`
	IncentiveFactor map[string]interface{} `json:"incentiveFactor"`
	CreatedAt       int64                  `json:"createdAt"`
	CutoffAt        int64                  `json:"cutoffAt"`
	ResolvedAt      int64                  `json:"resolvedAt"`
}

// GetMarketResponse represents the API response for GetMarket
type GetMarketResponse struct {
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Result struct {
		Data Market `json:"data"`
	} `json:"result"`
}

// GetMarketsResponse represents the API response for GetMarkets
type GetMarketsResponse struct {
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Result struct {
		Total int      `json:"total"`
		List  []Market `json:"list"`
	} `json:"result"`
}

// QuoteToken represents a supported quote token
type QuoteToken struct {
	ID                 int    `json:"id"`
	QuoteTokenName     string `json:"quoteTokenName"`
	QuoteTokenAddress  string `json:"quoteTokenAddress"`
	CTFExchangeAddress string `json:"ctfExchangeAddress"`
	Decimal            int    `json:"decimal"`
	Symbol             string `json:"symbol"`
	ChainID            string `json:"chainId"`
	CreatedAt          int64  `json:"createdAt"`
}

// GetQuoteTokensResponse represents the API response for GetQuoteTokens
type GetQuoteTokensResponse struct {
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Result struct {
		Total int          `json:"total"`
		List  []QuoteToken `json:"list"`
	} `json:"result"`
}

// BatchOrderResult represents the result of a single order in a batch operation
type BatchOrderResult struct {
	Index   int                  `json:"index"`
	Success bool                 `json:"success"`
	Result  interface{}          `json:"result,omitempty"`
	Error   string               `json:"error,omitempty"`
	Order   *PlaceOrderDataInput `json:"order,omitempty"`
}

// BatchCancelResult represents the result of a single cancel in a batch operation
type BatchCancelResult struct {
	Index   int         `json:"index"`
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
	OrderID string      `json:"orderId,omitempty"`
}

// CancelAllOrdersResult represents the summary of cancelling all orders
type CancelAllOrdersResult struct {
	TotalOrders int                 `json:"totalOrders"`
	Cancelled   int                 `json:"cancelled"`
	Failed      int                 `json:"failed"`
	Results     []BatchCancelResult `json:"results"`
}

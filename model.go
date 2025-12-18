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
	MarketID              int
	TokenID               string
	MakerAmountInQuoteToken *string // Optional: amount in quote token (e.g., USDC)
	MakerAmountInBaseToken  *string // Optional: amount in base token (e.g., YES token)
	Price                 string
	Side                  OrderSide
	OrderType             OrderType
}

// OrderData represents the data for building an order
type OrderData struct {
	Maker        string
	Taker        string
	TokenID      string
	MakerAmount  string
	TakerAmount  string
	Side         OrderSide
	FeeRateBps   string
	Nonce        string
	Signer       string
	Expiration   string
	SignatureType SignatureType
}

// SignedOrder represents an order with its signature
type SignedOrder struct {
	Order     *Order
	Signature string
}

// Order represents an EIP712 order structure
type Order struct {
	Salt         string
	Maker        string
	Signer       string
	Taker        string
	TokenID      string
	MakerAmount  string
	TakerAmount  string
	Expiration   string
	Nonce        string
	FeeRateBps   string
	Side         string
	SignatureType string
}

// FeeRateSettings represents fee rate settings from the FeeManager contract
type FeeRateSettings struct {
	MakerMaxFeeRate float64
	TakerMaxFeeRate float64
	Enabled         bool
}


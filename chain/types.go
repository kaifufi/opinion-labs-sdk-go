package chain

// OrderSide represents the side of an order
type OrderSide int

const (
	OrderSideBuy OrderSide = iota
	OrderSideSell
)

// SignatureType represents the signature type for orders
type SignatureType int

const (
	SignatureTypeEOA SignatureType = iota
	SignatureTypePolyGnosisSafe
	SignatureTypePolyProxy
)

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

// SignedOrder represents an order with its signature
type SignedOrder struct {
	Order     *Order
	Signature string
}

// FeeRateSettings represents fee rate settings from the FeeManager contract
type FeeRateSettings struct {
	MakerMaxFeeRate float64
	TakerMaxFeeRate float64
	Enabled         bool
}


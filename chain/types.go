package chain

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

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

// ERC20 ABI JSON for allowance and approve functions
const erc20ABIJSON = `[
	{
		"constant": true,
		"inputs": [
			{"name": "owner", "type": "address"},
			{"name": "spender", "type": "address"}
		],
		"name": "allowance",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{"name": "spender", "type": "address"},
			{"name": "amount", "type": "uint256"}
		],
		"name": "approve",
		"outputs": [{"name": "", "type": "bool"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "decimals",
		"outputs": [{"name": "", "type": "uint8"}],
		"type": "function"
	}
]`

// ConditionalTokens ABI JSON for isApprovedForAll and setApprovalForAll
const conditionalTokensABIJSON = `[
	{
		"constant": true,
		"inputs": [
			{"name": "owner", "type": "address"},
			{"name": "operator", "type": "address"}
		],
		"name": "isApprovedForAll",
		"outputs": [{"name": "", "type": "bool"}],
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{"name": "operator", "type": "address"},
			{"name": "approved", "type": "bool"}
		],
		"name": "setApprovalForAll",
		"outputs": [],
		"type": "function"
	}
]`

// Multisend ABI JSON
const multisendABIJSON = `[
	{
		"constant": false,
		"inputs": [
			{"name": "transactions", "type": "bytes"}
		],
		"name": "multiSend",
		"outputs": [],
		"type": "function"
	}
]`

// GetERC20ABI returns the parsed ERC20 ABI
func GetERC20ABI() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		panic("failed to parse ERC20 ABI: " + err.Error())
	}
	return parsed
}

// GetConditionalTokensABI returns the parsed ConditionalTokens ABI
func GetConditionalTokensABI() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(conditionalTokensABIJSON))
	if err != nil {
		panic("failed to parse ConditionalTokens ABI: " + err.Error())
	}
	return parsed
}

// GetMultisendABI returns the parsed Multisend ABI
func GetMultisendABI() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(multisendABIJSON))
	if err != nil {
		panic("failed to parse Multisend ABI: " + err.Error())
	}
	return parsed
}

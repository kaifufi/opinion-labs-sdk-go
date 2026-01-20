package chain

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// OrderBuilder builds and signs orders
type OrderBuilder struct {
	exchangeAddr common.Address
	chainID      *big.Int
	signer       *ecdsa.PrivateKey
}

// NewOrderBuilder creates a new OrderBuilder
func NewOrderBuilder(exchangeAddr string, chainID int64, signer *ecdsa.PrivateKey) (*OrderBuilder, error) {
	return &OrderBuilder{
		exchangeAddr: common.HexToAddress(exchangeAddr),
		chainID:      big.NewInt(chainID),
		signer:       signer,
	}, nil
}

// BuildOrder builds an order from OrderData
func (ob *OrderBuilder) BuildOrder(data *OrderData) (*Order, error) {
	if err := ob.validateInputs(data); err != nil {
		return nil, err
	}

	// Generate salt if not provided
	salt := ob.generateSalt()

	// Set defaults
	if data.Signer == "" {
		data.Signer = data.Maker
	}

	if data.Expiration == "" {
		data.Expiration = "0"
	}

	// Convert side to string
	sideStr := "0"
	if data.Side == OrderSideSell {
		sideStr = "1"
	}

	// Convert signature type to string
	sigTypeStr := "0"
	if data.SignatureType == SignatureTypePolyGnosisSafe {
		sigTypeStr = "1"
	} else if data.SignatureType == SignatureTypePolyProxy {
		sigTypeStr = "2"
	}

	order := &Order{
		Salt:          salt,
		Maker:         normalizeAddress(data.Maker),
		Signer:        normalizeAddress(data.Signer),
		Taker:         normalizeAddress(data.Taker),
		TokenID:       data.TokenID,
		MakerAmount:   data.MakerAmount,
		TakerAmount:   data.TakerAmount,
		Expiration:    data.Expiration,
		Nonce:         data.Nonce,
		FeeRateBps:    data.FeeRateBps,
		Side:          sideStr,
		SignatureType: sigTypeStr,
	}

	return order, nil
}

// BuildSignedOrder builds and signs an order
func (ob *OrderBuilder) BuildSignedOrder(data *OrderData) (*SignedOrder, error) {
	order, err := ob.BuildOrder(data)
	if err != nil {
		return nil, err
	}

	signature, err := ob.SignOrder(order)
	if err != nil {
		return nil, err
	}

	return &SignedOrder{
		Order:     order,
		Signature: signature,
	}, nil
}

// SignOrder signs an order using EIP712 typed data signing
// This follows the EIP712 specification matching the Python SDK implementation
func (ob *OrderBuilder) SignOrder(order *Order) (string, error) {
	// Convert the Order to typed data for EIP712 hashing
	typedData, err := OrderToTypedData(order)
	if err != nil {
		return "", fmt.Errorf("failed to convert order to typed data: %w", err)
	}

	// Create the EIP712 domain
	domain := NewEIP712Domain(ob.chainID, ob.exchangeAddr)

	// Create the EIP712 sign hash: keccak256("\x19\x01" ++ domainSeparator ++ structHash)
	signHash := CreateOrderSignHash(domain, typedData)

	// Sign the hash with ECDSA
	signature, err := crypto.Sign(signHash.Bytes(), ob.signer)
	if err != nil {
		return "", fmt.Errorf("failed to sign order: %w", err)
	}

	// Adjust recovery ID (v) to be 27 or 28 for Ethereum compatibility
	// go-ethereum's crypto.Sign returns v as 0 or 1, but Ethereum expects 27 or 28
	signature[64] += 27

	return fmt.Sprintf("0x%x", signature), nil
}

func (ob *OrderBuilder) validateInputs(data *OrderData) error {
	if data.Maker == "" {
		return fmt.Errorf("maker is required")
	}
	if data.TokenID == "" {
		return fmt.Errorf("tokenId is required")
	}
	if data.MakerAmount == "" {
		return fmt.Errorf("makerAmount is required")
	}
	if data.TakerAmount == "" {
		return fmt.Errorf("takerAmount is required")
	}
	if data.Side != OrderSideBuy && data.Side != OrderSideSell {
		return fmt.Errorf("invalid side")
	}
	return nil
}

func (ob *OrderBuilder) generateSalt() string {
	now := time.Now().Unix()
	random := rand.Int63()
	return strconv.FormatInt(now*random, 10)
}

func normalizeAddress(addr string) string {
	return common.HexToAddress(addr).Hex()
}

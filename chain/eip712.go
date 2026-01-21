package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// EIP712 related errors
var (
	ErrInvalidOrderSalt   = errors.New("invalid order salt")
	ErrInvalidTokenID     = errors.New("invalid token ID")
	ErrInvalidMakerAmount = errors.New("invalid maker amount")
	ErrInvalidTakerAmount = errors.New("invalid taker amount")
)

// EIP712 Domain constants matching Python SDK
const (
	EIP712DomainName    = "OPINION CTF Exchange"
	EIP712DomainVersion = "1"
)

// Pre-computed type hashes using keccak256
var (
	// EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)
	EIP712DomainTypeHash = crypto.Keccak256Hash([]byte(
		"EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)",
	))

	// Order(uint256 salt,address maker,address signer,address taker,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint256 expiration,uint256 nonce,uint256 feeRateBps,uint8 side,uint8 signatureType)
	OrderTypeHash = crypto.Keccak256Hash([]byte(
		"Order(uint256 salt,address maker,address signer,address taker,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint256 expiration,uint256 nonce,uint256 feeRateBps,uint8 side,uint8 signatureType)",
	))
)

// EIP712Domain represents the EIP712 domain separator data
type EIP712Domain struct {
	Name              string
	Version           string
	ChainID           *big.Int
	VerifyingContract common.Address
}

// NewEIP712Domain creates a new EIP712Domain with the standard values
func NewEIP712Domain(chainID *big.Int, verifyingContract common.Address) *EIP712Domain {
	return &EIP712Domain{
		Name:              EIP712DomainName,
		Version:           EIP712DomainVersion,
		ChainID:           chainID,
		VerifyingContract: verifyingContract,
	}
}

// Hash computes the EIP712 domain separator hash
func (d *EIP712Domain) Hash() common.Hash {
	// Encode: typeHash + keccak256(name) + keccak256(version) + chainId + verifyingContract
	nameHash := crypto.Keccak256Hash([]byte(d.Name))
	versionHash := crypto.Keccak256Hash([]byte(d.Version))

	// ABI encode the domain separator data
	// The encoding is: typeHash ++ keccak256(name) ++ keccak256(version) ++ chainId ++ verifyingContract
	bytes32Type, _ := abi.NewType("bytes32", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	addressType, _ := abi.NewType("address", "", nil)

	arguments := abi.Arguments{
		{Type: bytes32Type}, // typeHash
		{Type: bytes32Type}, // nameHash
		{Type: bytes32Type}, // versionHash
		{Type: uint256Type}, // chainId
		{Type: addressType}, // verifyingContract
	}

	encoded, err := arguments.Pack(
		EIP712DomainTypeHash,
		nameHash,
		versionHash,
		d.ChainID,
		d.VerifyingContract,
	)
	if err != nil {
		panic("failed to encode domain separator: " + err.Error())
	}

	return crypto.Keccak256Hash(encoded)
}

// OrderTypedData represents the order data for EIP712 hashing
type OrderTypedData struct {
	Salt          *big.Int
	Maker         common.Address
	Signer        common.Address
	Taker         common.Address
	TokenID       *big.Int
	MakerAmount   *big.Int
	TakerAmount   *big.Int
	Expiration    *big.Int
	Nonce         *big.Int
	FeeRateBps    *big.Int
	Side          uint8
	SignatureType uint8
}

// Hash computes the struct hash for the order
func (o *OrderTypedData) Hash() common.Hash {
	// ABI encode the order data
	bytes32Type, _ := abi.NewType("bytes32", "", nil)
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	uint8Type, _ := abi.NewType("uint8", "", nil)

	arguments := abi.Arguments{
		{Type: bytes32Type}, // typeHash
		{Type: uint256Type}, // salt
		{Type: addressType}, // maker
		{Type: addressType}, // signer
		{Type: addressType}, // taker
		{Type: uint256Type}, // tokenId
		{Type: uint256Type}, // makerAmount
		{Type: uint256Type}, // takerAmount
		{Type: uint256Type}, // expiration
		{Type: uint256Type}, // nonce
		{Type: uint256Type}, // feeRateBps
		{Type: uint8Type},   // side
		{Type: uint8Type},   // signatureType
	}

	encoded, err := arguments.Pack(
		OrderTypeHash,
		o.Salt,
		o.Maker,
		o.Signer,
		o.Taker,
		o.TokenID,
		o.MakerAmount,
		o.TakerAmount,
		o.Expiration,
		o.Nonce,
		o.FeeRateBps,
		o.Side,
		o.SignatureType,
	)
	if err != nil {
		panic("failed to encode order struct: " + err.Error())
	}

	return crypto.Keccak256Hash(encoded)
}

// CreateOrderSignHash creates the final EIP712 hash to be signed
// This follows the EIP712 specification: keccak256("\x19\x01" ++ domainSeparator ++ structHash)
func CreateOrderSignHash(domain *EIP712Domain, order *OrderTypedData) common.Hash {
	domainSeparator := domain.Hash()
	structHash := order.Hash()

	// EIP712 prefix: \x19\x01
	prefix := []byte{0x19, 0x01}

	// Concatenate prefix + domainSeparator + structHash
	data := make([]byte, 0, 2+32+32)
	data = append(data, prefix...)
	data = append(data, domainSeparator.Bytes()...)
	data = append(data, structHash.Bytes()...)

	return crypto.Keccak256Hash(data)
}

// OrderToTypedData converts an Order to OrderTypedData for EIP712 hashing
func OrderToTypedData(order *Order) (*OrderTypedData, error) {
	salt, ok := new(big.Int).SetString(order.Salt, 10)
	if !ok {
		return nil, ErrInvalidOrderSalt
	}

	tokenID, ok := new(big.Int).SetString(order.TokenID, 10)
	if !ok {
		return nil, ErrInvalidTokenID
	}

	makerAmount, ok := new(big.Int).SetString(order.MakerAmount, 10)
	if !ok {
		return nil, ErrInvalidMakerAmount
	}

	takerAmount, ok := new(big.Int).SetString(order.TakerAmount, 10)
	if !ok {
		return nil, ErrInvalidTakerAmount
	}

	expiration, ok := new(big.Int).SetString(order.Expiration, 10)
	if !ok {
		expiration = big.NewInt(0)
	}

	nonce, ok := new(big.Int).SetString(order.Nonce, 10)
	if !ok {
		nonce = big.NewInt(0)
	}

	feeRateBps, ok := new(big.Int).SetString(order.FeeRateBps, 10)
	if !ok {
		feeRateBps = big.NewInt(0)
	}

	side := uint8(0)
	if order.Side == "1" {
		side = 1
	}

	signatureType := uint8(0)
	if order.SignatureType == "1" {
		signatureType = 1
	} else if order.SignatureType == "2" {
		signatureType = 2
	}

	return &OrderTypedData{
		Salt:          salt,
		Maker:         common.HexToAddress(order.Maker),
		Signer:        common.HexToAddress(order.Signer),
		Taker:         common.HexToAddress(order.Taker),
		TokenID:       tokenID,
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Expiration:    expiration,
		Nonce:         nonce,
		FeeRateBps:    feeRateBps,
		Side:          side,
		SignatureType: signatureType,
	}, nil
}

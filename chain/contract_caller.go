package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ContractCaller handles blockchain contract interactions
type ContractCaller struct {
	client                  *ethclient.Client
	privateKey             *ecdsa.PrivateKey
	multiSigAddr           common.Address
	conditionalTokensAddr  common.Address
	multisendAddr           common.Address
	feeManagerAddr          common.Address
	enableTradingCheckInterval time.Duration
	enableTradingLastTime   time.Time
	tokenDecimalsCache      map[string]int
}

// NewContractCaller creates a new ContractCaller instance
func NewContractCaller(
	rpcURL string,
	privateKeyHex string,
	multiSigAddr string,
	conditionalTokensAddr string,
	multisendAddr string,
	feeManagerAddr string,
	enableTradingCheckInterval time.Duration,
) (*ContractCaller, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	return &ContractCaller{
		client:                  client,
		privateKey:              privateKey,
		multiSigAddr:            common.HexToAddress(multiSigAddr),
		conditionalTokensAddr:  common.HexToAddress(conditionalTokensAddr),
		multisendAddr:           common.HexToAddress(multisendAddr),
		feeManagerAddr:           common.HexToAddress(feeManagerAddr),
		enableTradingCheckInterval: enableTradingCheckInterval,
		tokenDecimalsCache:      make(map[string]int),
	}, nil
}

// GetSignerAddress returns the address of the signer
func (cc *ContractCaller) GetSignerAddress() common.Address {
	publicKey := cc.privateKey.Public()
	publicKeyECDSA, _ := publicKey.(*ecdsa.PublicKey)
	return crypto.PubkeyToAddress(*publicKeyECDSA)
}

// GetMultiSigAddress returns the multi-sig address
func (cc *ContractCaller) GetMultiSigAddress() common.Address {
	return cc.multiSigAddr
}

// CheckGasBalance checks if signer has enough gas tokens
func (cc *ContractCaller) CheckGasBalance(ctx context.Context, estimatedGas uint64) error {
	signerAddr := cc.GetSignerAddress()
	balance, err := cc.client.BalanceAt(ctx, signerAddr, nil)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	gasPrice, err := cc.client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	// Add 20% safety margin
	estimatedGasWithMargin := new(big.Int).Mul(big.NewInt(int64(estimatedGas)), big.NewInt(120))
	estimatedGasWithMargin.Div(estimatedGasWithMargin, big.NewInt(100))
	
	requiredEth := new(big.Int).Mul(estimatedGasWithMargin, gasPrice)

		if balance.Cmp(requiredEth) < 0 {
		return fmt.Errorf("insufficient gas balance: signer %s has %s ETH, but needs approximately %s ETH for gas",
			signerAddr.Hex(),
			balance.String(),
			requiredEth.String(),
		)
	}

	return nil
}

// GetTokenDecimals gets token decimals with caching
func (cc *ContractCaller) GetTokenDecimals(ctx context.Context, tokenAddr common.Address) (int, error) {
	tokenKey := tokenAddr.Hex()
	
	if decimals, ok := cc.tokenDecimalsCache[tokenKey]; ok {
		return decimals, nil
	}

	// TODO: Implement proper ERC20 decimals() call using ERC20 ABI
	// This is a simplified version - in production, you'd use the ERC20 ABI
	decimals := 18 // Default to 18
	
	cc.tokenDecimalsCache[tokenKey] = decimals
	return decimals, nil
}

// Split splits collateral into outcome tokens
func (cc *ContractCaller) Split(ctx context.Context, collateralToken common.Address, conditionID []byte, amount *big.Int) (*types.Transaction, error) {
	if err := cc.CheckGasBalance(ctx, 300000); err != nil {
		return nil, err
	}

	// TODO: Check balance using ERC20 contract ABI
	// In production, you'd call the ERC20 contract to check balance
	// For now, this is a placeholder

	// TODO: Implement splitPosition using ConditionalTokens contract ABI
	// Build transaction data for splitPosition
	// This would use the ConditionalTokens ABI in production
	// For now, returning a placeholder transaction
	
	auth, err := bind.NewKeyedTransactorWithChainID(cc.privateKey, big.NewInt(56))
	if err != nil {
		return nil, err
	}

	// Placeholder - actual implementation would call conditionalTokens.splitPosition
	// This requires the ConditionalTokens contract ABI
	_ = collateralToken
	_ = conditionID
	_ = amount
	_ = auth

	return nil, fmt.Errorf("split not fully implemented - requires ConditionalTokens ABI")
}

// Merge merges outcome tokens back into collateral
func (cc *ContractCaller) Merge(ctx context.Context, collateralToken common.Address, conditionID []byte, amount *big.Int) (*types.Transaction, error) {
	if err := cc.CheckGasBalance(ctx, 300000); err != nil {
		return nil, err
	}

	// TODO: Implement mergePositions using ConditionalTokens contract ABI
	// Placeholder implementation
	_ = collateralToken
	_ = conditionID
	_ = amount

	return nil, fmt.Errorf("merge not fully implemented - requires ConditionalTokens ABI")
}

// Redeem redeems winning outcome tokens for collateral
func (cc *ContractCaller) Redeem(ctx context.Context, collateralToken common.Address, conditionID []byte) (*types.Transaction, error) {
	if err := cc.CheckGasBalance(ctx, 300000); err != nil {
		return nil, err
	}

	// TODO: Implement redeemPositions using ConditionalTokens contract ABI
	// Placeholder implementation
	_ = collateralToken
	_ = conditionID

	return nil, fmt.Errorf("redeem not fully implemented - requires ConditionalTokens ABI")
}

// EnableTrading enables trading by approving necessary tokens
func (cc *ContractCaller) EnableTrading(ctx context.Context, supportedQuoteTokens map[string]string) (*types.Transaction, error) {
	// Check if we should skip based on interval
	if !cc.enableTradingLastTime.IsZero() {
		elapsed := time.Since(cc.enableTradingLastTime)
		if elapsed < cc.enableTradingCheckInterval {
			return nil, nil // Skip if within interval
		}
	}
	cc.enableTradingLastTime = time.Now()

	if err := cc.CheckGasBalance(ctx, 500000); err != nil {
		return nil, err
	}

	// TODO: Implement full enable trading functionality:
	// 1. Check allowances for each quote token using ERC20 ABI
	// 2. Build multisend transaction with approvals
	// 3. Execute via Safe wallet (requires Safe wallet integration)
	// Placeholder implementation
	_ = supportedQuoteTokens

	return nil, fmt.Errorf("enable_trading not fully implemented - requires Safe wallet and ERC20 ABIs")
}

// GetFeeRateSettings gets fee rate settings from FeeManager contract
func (cc *ContractCaller) GetFeeRateSettings(ctx context.Context, tokenID *big.Int) (*FeeRateSettings, error) {
	// TODO: Implement getFeeRateSettings call using FeeManager contract ABI
	// In production, this would call feeManager.getFeeRateSettings(tokenID)
	_ = tokenID

	return nil, fmt.Errorf("get_fee_rate_settings not fully implemented - requires FeeManager ABI")
}

// Close closes the Ethereum client connection
func (cc *ContractCaller) Close() {
	if cc.client != nil {
		cc.client.Close()
	}
}


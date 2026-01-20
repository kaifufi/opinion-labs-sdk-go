package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ContractCaller handles blockchain contract interactions
type ContractCaller struct {
	client                     *ethclient.Client
	privateKey                 *ecdsa.PrivateKey
	multiSigAddr               common.Address
	conditionalTokensAddr      common.Address
	multisendAddr              common.Address
	feeManagerAddr             common.Address
	enableTradingCheckInterval time.Duration
	enableTradingLastTime      time.Time
	tokenDecimalsCache         map[string]int
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
		client:                     client,
		privateKey:                 privateKey,
		multiSigAddr:               common.HexToAddress(multiSigAddr),
		conditionalTokensAddr:      common.HexToAddress(conditionalTokensAddr),
		multisendAddr:              common.HexToAddress(multisendAddr),
		feeManagerAddr:             common.HexToAddress(feeManagerAddr),
		enableTradingCheckInterval: enableTradingCheckInterval,
		tokenDecimalsCache:         make(map[string]int),
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

	// Check balance of collateral token
	balance, err := cc.getERC20Balance(ctx, collateralToken, cc.multiSigAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get collateral balance: %w", err)
	}
	if balance.Cmp(amount) < 0 {
		return nil, fmt.Errorf("insufficient collateral balance: has %s, needs %s", balance.String(), amount.String())
	}

	// Build splitPosition call data
	conditionalTokensABI := GetConditionalTokensABI()

	// Convert conditionID to [32]byte
	var conditionIDBytes32 [32]byte
	copy(conditionIDBytes32[:], conditionID)

	// parentCollectionId is NULL_HASH (all zeros)
	var parentCollectionID [32]byte

	// partition = [1, 2] for binary markets (YES and NO outcomes)
	partition := []*big.Int{big.NewInt(1), big.NewInt(2)}

	splitData, err := conditionalTokensABI.Pack("splitPosition",
		collateralToken,
		parentCollectionID,
		conditionIDBytes32,
		partition,
		amount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pack splitPosition: %w", err)
	}

	// Build multisend transaction
	multiSendTxs := []MultiSendTx{
		{
			Operation: MultiSendOperationCall,
			To:        cc.conditionalTokensAddr,
			Value:     big.NewInt(0),
			Data:      splitData,
		},
	}

	// Execute via multisend
	tx, err := cc.executeMultisend(ctx, multiSendTxs)
	if err != nil {
		return nil, fmt.Errorf("failed to execute splitPosition: %w", err)
	}

	// Wait for transaction receipt and validate
	receipt, err := cc.waitForReceipt(ctx, tx.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to wait for split transaction: %w", err)
	}
	if receipt.Status != 1 {
		return nil, fmt.Errorf("split transaction failed: tx hash %s", tx.Hash().Hex())
	}

	return tx, nil
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

// EnableTrading enables trading by approving necessary tokens.
// This checks ERC20 allowances and builds approval transactions for:
// 1. ERC20 tokens -> CTF Exchange (for trading)
// 2. ERC20 tokens -> ConditionalTokens (for splitting)
// 3. ConditionalTokens -> CTF Exchange (setApprovalForAll)
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

	// Collect all approval transactions to execute via multisend
	var multiSendTxs []MultiSendTx

	// ERC20 ABI for allowance and approve functions
	erc20ABI := GetERC20ABI()

	for erc20Address, ctfExchangeAddress := range supportedQuoteTokens {
		erc20Addr := common.HexToAddress(erc20Address)
		ctfExchangeAddr := common.HexToAddress(ctfExchangeAddress)

		// Get token decimals
		decimals, err := cc.GetTokenDecimals(ctx, erc20Addr)
		if err != nil {
			return nil, fmt.Errorf("failed to get decimals for %s: %w", erc20Address, err)
		}

		// Calculate minimum threshold: 1 billion * 10^decimals
		minThreshold := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
		minThreshold = minThreshold.Mul(minThreshold, big.NewInt(1000000000))

		// Unlimited approval amount (max uint256)
		maxUint256 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))

		// Check allowance for CTF Exchange
		allowance, err := cc.getERC20Allowance(ctx, erc20Addr, cc.multiSigAddr, ctfExchangeAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to get allowance: %w", err)
		}

		if allowance.Cmp(minThreshold) < 0 {
			// If there's existing allowance > 0, reset to 0 first (USDT-style protection)
			if allowance.Sign() > 0 {
				resetData, err := erc20ABI.Pack("approve", ctfExchangeAddr, big.NewInt(0))
				if err != nil {
					return nil, fmt.Errorf("failed to pack reset approve: %w", err)
				}
				multiSendTxs = append(multiSendTxs, MultiSendTx{
					Operation: MultiSendOperationCall,
					To:        erc20Addr,
					Value:     big.NewInt(0),
					Data:      resetData,
				})
			}

			// Approve unlimited allowance
			approveData, err := erc20ABI.Pack("approve", ctfExchangeAddr, maxUint256)
			if err != nil {
				return nil, fmt.Errorf("failed to pack approve: %w", err)
			}
			multiSendTxs = append(multiSendTxs, MultiSendTx{
				Operation: MultiSendOperationCall,
				To:        erc20Addr,
				Value:     big.NewInt(0),
				Data:      approveData,
			})
		}

		// Check allowance for ConditionalTokens (used for splitting)
		allowanceForCT, err := cc.getERC20Allowance(ctx, erc20Addr, cc.multiSigAddr, cc.conditionalTokensAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to get allowance for conditional tokens: %w", err)
		}

		if allowanceForCT.Cmp(minThreshold) < 0 {
			// Reset to 0 first if needed
			if allowanceForCT.Sign() > 0 {
				resetData, err := erc20ABI.Pack("approve", cc.conditionalTokensAddr, big.NewInt(0))
				if err != nil {
					return nil, fmt.Errorf("failed to pack reset approve for CT: %w", err)
				}
				multiSendTxs = append(multiSendTxs, MultiSendTx{
					Operation: MultiSendOperationCall,
					To:        erc20Addr,
					Value:     big.NewInt(0),
					Data:      resetData,
				})
			}

			// Approve unlimited allowance
			approveData, err := erc20ABI.Pack("approve", cc.conditionalTokensAddr, maxUint256)
			if err != nil {
				return nil, fmt.Errorf("failed to pack approve for CT: %w", err)
			}
			multiSendTxs = append(multiSendTxs, MultiSendTx{
				Operation: MultiSendOperationCall,
				To:        erc20Addr,
				Value:     big.NewInt(0),
				Data:      approveData,
			})
		}

		// Check if CTF Exchange is approved for all on ConditionalTokens
		isApprovedForAll, err := cc.isApprovedForAll(ctx, cc.multiSigAddr, ctfExchangeAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to check isApprovedForAll: %w", err)
		}

		if !isApprovedForAll {
			conditionalTokensABI := GetConditionalTokensABI()
			setApprovalData, err := conditionalTokensABI.Pack("setApprovalForAll", ctfExchangeAddr, true)
			if err != nil {
				return nil, fmt.Errorf("failed to pack setApprovalForAll: %w", err)
			}
			multiSendTxs = append(multiSendTxs, MultiSendTx{
				Operation: MultiSendOperationCall,
				To:        cc.conditionalTokensAddr,
				Value:     big.NewInt(0),
				Data:      setApprovalData,
			})
		}
	}

	// If no approvals needed, return nil transaction
	if len(multiSendTxs) == 0 {
		return nil, nil
	}

	// Execute all approvals via multisend
	tx, err := cc.executeMultisend(ctx, multiSendTxs)
	if err != nil {
		return nil, fmt.Errorf("failed to execute multisend: %w", err)
	}

	return tx, nil
}

// getERC20Allowance returns the ERC20 allowance for owner to spender
func (cc *ContractCaller) getERC20Allowance(ctx context.Context, token, owner, spender common.Address) (*big.Int, error) {
	erc20ABI := GetERC20ABI()
	data, err := erc20ABI.Pack("allowance", owner, spender)
	if err != nil {
		return nil, err
	}

	result, err := cc.client.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: data,
	}, nil)
	if err != nil {
		return nil, err
	}

	var allowance *big.Int
	err = erc20ABI.UnpackIntoInterface(&allowance, "allowance", result)
	if err != nil {
		return nil, err
	}

	return allowance, nil
}

// getERC20Balance returns the ERC20 balance for an account
func (cc *ContractCaller) getERC20Balance(ctx context.Context, token, account common.Address) (*big.Int, error) {
	erc20ABI := GetERC20ABI()
	data, err := erc20ABI.Pack("balanceOf", account)
	if err != nil {
		return nil, err
	}

	result, err := cc.client.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: data,
	}, nil)
	if err != nil {
		return nil, err
	}

	var balance *big.Int
	err = erc20ABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// waitForReceipt waits for a transaction receipt with timeout
func (cc *ContractCaller) waitForReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	for {
		receipt, err := cc.client.TransactionReceipt(timeoutCtx, txHash)
		if err == nil {
			return receipt, nil
		}

		// Check if context is done
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction receipt: %s", txHash.Hex())
		default:
			// Wait a bit before retrying
			time.Sleep(2 * time.Second)
		}
	}
}

// isApprovedForAll checks if an operator is approved for all tokens on ConditionalTokens
func (cc *ContractCaller) isApprovedForAll(ctx context.Context, owner, operator common.Address) (bool, error) {
	conditionalTokensABI := GetConditionalTokensABI()
	data, err := conditionalTokensABI.Pack("isApprovedForAll", owner, operator)
	if err != nil {
		return false, err
	}

	result, err := cc.client.CallContract(ctx, ethereum.CallMsg{
		To:   &cc.conditionalTokensAddr,
		Data: data,
	}, nil)
	if err != nil {
		return false, err
	}

	var approved bool
	err = conditionalTokensABI.UnpackIntoInterface(&approved, "isApprovedForAll", result)
	if err != nil {
		return false, err
	}

	return approved, nil
}

// MultiSendTx represents a single transaction in a multisend batch
type MultiSendTx struct {
	Operation uint8
	To        common.Address
	Value     *big.Int
	Data      []byte
}

const (
	MultiSendOperationCall         uint8 = 0
	MultiSendOperationDelegateCall uint8 = 1
)

// executeMultisend executes multiple transactions via the multisend contract
func (cc *ContractCaller) executeMultisend(ctx context.Context, txs []MultiSendTx) (*types.Transaction, error) {
	// Build encoded multisend data
	var encodedTxs []byte
	for _, tx := range txs {
		// Encode: operation (1 byte) + to (20 bytes) + value (32 bytes) + dataLength (32 bytes) + data
		packed := make([]byte, 0)
		packed = append(packed, tx.Operation)
		packed = append(packed, tx.To.Bytes()...)
		packed = append(packed, common.LeftPadBytes(tx.Value.Bytes(), 32)...)
		packed = append(packed, common.LeftPadBytes(big.NewInt(int64(len(tx.Data))).Bytes(), 32)...)
		packed = append(packed, tx.Data...)
		encodedTxs = append(encodedTxs, packed...)
	}

	multisendABI := GetMultisendABI()
	callData, err := multisendABI.Pack("multiSend", encodedTxs)
	if err != nil {
		return nil, fmt.Errorf("failed to pack multisend: %w", err)
	}

	// Build and sign transaction
	chainID, err := cc.client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	nonce, err := cc.client.PendingNonceAt(ctx, cc.GetSignerAddress())
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := cc.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	tx := types.NewTransaction(
		nonce,
		cc.multisendAddr,
		big.NewInt(0),
		uint64(500000),
		gasPrice,
		callData,
	)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), cc.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	if err := cc.client.SendTransaction(ctx, signedTx); err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx, nil
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

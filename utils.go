package opinionclob

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const (
	MaxDecimals = 18
	ZeroAddress = "0x0000000000000000000000000000000000000000"
)

// SafeAmountToWei safely converts human-readable amount to wei units
func SafeAmountToWei(amount float64, decimals int) (*big.Int, error) {
	if amount <= 0 {
		return nil, &InvalidParamError{Message: fmt.Sprintf("amount must be positive, got: %f", amount)}
	}

	if decimals < 0 || decimals > MaxDecimals {
		return nil, &InvalidParamError{Message: fmt.Sprintf("decimals must be between 0 and %d, got: %d", MaxDecimals, decimals)}
	}

	// Convert to string for precision
	amountStr := strconv.FormatFloat(amount, 'f', -1, 64)
	
	// Split into integer and decimal parts
	parts := strings.Split(amountStr, ".")
	if len(parts) > 2 {
		return nil, &InvalidParamError{Message: "invalid amount format"}
	}

	integerPart := parts[0]
	decimalPart := ""
	if len(parts) == 2 {
		decimalPart = parts[1]
	}

	// Pad or truncate decimal part to match decimals
	if len(decimalPart) > decimals {
		decimalPart = decimalPart[:decimals]
	} else {
		decimalPart = decimalPart + strings.Repeat("0", decimals-len(decimalPart))
	}

	// Combine integer and decimal parts
	combined := integerPart + decimalPart

	// Convert to big.Int
	result, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, &InvalidParamError{Message: "failed to convert amount to big.Int"}
	}

	// Validate result fits in uint256
	maxUint256 := new(big.Int)
	maxUint256.Exp(big.NewInt(2), big.NewInt(256), nil)
	if result.Cmp(maxUint256) >= 0 {
		return nil, &InvalidParamError{Message: fmt.Sprintf("amount too large for uint256: %s", result.String())}
	}

	if result.Sign() <= 0 {
		return nil, &InvalidParamError{Message: "calculated amount is zero or negative"}
	}

	return result, nil
}

// CalculateOrderAmounts calculates maker and taker amounts based on price and side
func CalculateOrderAmounts(price float64, makerAmount *big.Int, side OrderSide, decimals int) (*big.Int, *big.Int, error) {
	// Validate price
	if price <= 0.001 || price >= 0.999 {
		return nil, nil, &InvalidParamError{Message: fmt.Sprintf("price must be between 0.001 and 0.999, got: %f", price)}
	}

	// Convert price to fraction for exact representation
	// Simplified version - in production, use proper fraction library
	priceBig := new(big.Float).SetFloat64(price)
	
	var recalculatedMakerAmount, takerAmount *big.Int

	if side == OrderSideBuy {
		// For BUY: price = maker/taker, so taker = maker/price
		// Round maker to 4 significant digits
		maker4Digit := roundToSignificantDigits(makerAmount, 4)
		
		// Calculate taker amount
		takerFloat := new(big.Float).Quo(new(big.Float).SetInt(maker4Digit), priceBig)
		takerAmount, _ = takerFloat.Int(nil)
		recalculatedMakerAmount = maker4Digit
	} else {
		// For SELL: price = taker/maker, so taker = maker*price
		// Round maker to 4 significant digits
		maker4Digit := roundToSignificantDigits(makerAmount, 4)
		
		// Calculate taker amount
		takerFloat := new(big.Float).Mul(new(big.Float).SetInt(maker4Digit), priceBig)
		takerAmount, _ = takerFloat.Int(nil)
		recalculatedMakerAmount = maker4Digit
	}

	// Ensure amounts are at least 1
	if takerAmount.Sign() <= 0 {
		takerAmount = big.NewInt(1)
	}
	if recalculatedMakerAmount.Sign() <= 0 {
		recalculatedMakerAmount = big.NewInt(1)
	}

	return recalculatedMakerAmount, takerAmount, nil
}

func roundToSignificantDigits(value *big.Int, n int) *big.Int {
	if value.Sign() == 0 {
		return big.NewInt(0)
	}

	// Get magnitude (number of digits)
	magnitude := len(value.String())

	// If already n digits or fewer, return as-is
	if magnitude <= n {
		return new(big.Int).Set(value)
	}

	// Calculate divisor to round to n significant digits
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(magnitude-n)), nil)

	// Round to n significant digits
	rounded := new(big.Int).Div(value, divisor)
	rounded.Mul(rounded, divisor)

	return rounded
}


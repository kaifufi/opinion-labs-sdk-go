package opinionclob

import "errors"

var (
	// ErrInvalidParam represents an invalid parameter error
	ErrInvalidParam = errors.New("invalid parameter")
	
	// ErrOpenAPI represents an OpenAPI error
	ErrOpenAPI = errors.New("openapi error")
	
	// ErrBalanceNotEnough represents insufficient balance error
	ErrBalanceNotEnough = errors.New("balance not enough")
	
	// ErrNoPositionsToRedeem represents no positions to redeem error
	ErrNoPositionsToRedeem = errors.New("no positions to redeem")
	
	// ErrInsufficientGasBalance represents insufficient gas balance error
	ErrInsufficientGasBalance = errors.New("insufficient gas balance")
)

// InvalidParamError represents an invalid parameter error with context
type InvalidParamError struct {
	Message string
}

func (e *InvalidParamError) Error() string {
	return e.Message
}

// OpenAPIError represents an OpenAPI error with context
type OpenAPIError struct {
	Message string
}

func (e *OpenAPIError) Error() string {
	return e.Message
}


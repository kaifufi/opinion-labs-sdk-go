package opinionclob

// ChainID represents a blockchain chain ID
type ChainID int

const (
	ChainIDBNBMainnet ChainID = 56 // BNB Chain (BSC) mainnet
)

// SupportedChainIDs lists all supported chain IDs
var SupportedChainIDs = []ChainID{ChainIDBNBMainnet}

// ContractAddresses holds contract addresses for each chain
type ContractAddresses struct {
	ConditionalTokens string
	Multisend         string
	FeeManager        string
}

// DefaultContractAddresses maps chain IDs to their contract addresses
var DefaultContractAddresses = map[ChainID]ContractAddresses{
	ChainIDBNBMainnet: {
		ConditionalTokens: "0xAD1a38cEc043e70E83a3eC30443dB285ED10D774",
		Multisend:          "0x998739BFdAAdde7C933B942a68053933098f9EDa",
		FeeManager:         "0xC9063Dc52dEEfb518E5b6634A6b8D624bc5d7c36",
	},
}


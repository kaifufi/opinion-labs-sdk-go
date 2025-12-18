# Configuration constants for Opinion CLOB SDK

# Supported chain IDs
SUPPORTED_CHAIN_IDS = [56]  # BNB Chain (BSC) mainnet

# BNB Chain (BSC) Mainnet Contract Addresses
# All addresses are in checksum format as required by web3.py
BNB_CHAIN_CONDITIONAL_TOKENS_ADDR = "0xAD1a38cEc043e70E83a3eC30443dB285ED10D774"
BNB_CHAIN_MULTISEND_ADDR = "0x998739BFdAAdde7C933B942a68053933098f9EDa"
BNB_CHAIN_FEE_MANAGER_ADDR = "0xC9063Dc52dEEfb518E5b6634A6b8D624bc5d7c36"

# Default contract addresses by chain ID
DEFAULT_CONTRACT_ADDRESSES = {
    56: {  # BNB Chain Mainnet
        "conditional_tokens": BNB_CHAIN_CONDITIONAL_TOKENS_ADDR,
        "multisend": BNB_CHAIN_MULTISEND_ADDR,
        "fee_manager": BNB_CHAIN_FEE_MANAGER_ADDR,
    }
}

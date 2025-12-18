#!/usr/bin/env python3
"""
Verify that the FeeManager contract address is properly formatted
and can be used with web3.py without checksum errors.
"""

import sys
from web3 import Web3

# Import the addresses
from opinion_clob_sdk.config import (
    BNB_CHAIN_FEE_MANAGER_ADDR,
    BNB_CHAIN_CONDITIONAL_TOKENS_ADDR,
    BNB_CHAIN_MULTISEND_ADDR,
)

def verify_address(name: str, address: str):
    """Verify an address is in valid checksum format"""
    try:
        is_valid = Web3.is_checksum_address(address)
        if is_valid:
            print(f"✅ {name}: {address}")
            print(f"   Valid checksum format")
        else:
            print(f"❌ {name}: {address}")
            print(f"   Invalid checksum format")
            return False
    except Exception as e:
        print(f"❌ {name}: {address}")
        print(f"   Error: {e}")
        return False
    return True

def main():
    print("=" * 80)
    print("Contract Address Checksum Verification")
    print("=" * 80)
    print()

    addresses = {
        "ConditionalTokens": BNB_CHAIN_CONDITIONAL_TOKENS_ADDR,
        "Multisend": BNB_CHAIN_MULTISEND_ADDR,
        "FeeManager": BNB_CHAIN_FEE_MANAGER_ADDR,
    }

    all_valid = True
    for name, address in addresses.items():
        if not verify_address(name, address):
            all_valid = False
        print()

    print("=" * 80)
    if all_valid:
        print("✅ All contract addresses are in valid checksum format!")
        print("\nYou can now use these addresses with web3.py without errors.")
        return 0
    else:
        print("❌ Some addresses are not in valid checksum format!")
        print("\nPlease update config.py with proper checksum addresses.")
        return 1

if __name__ == "__main__":
    sys.exit(main())

from eth_typing import ChecksumAddress
from web3 import Web3


def get_safe_contract(w3: Web3, address: ChecksumAddress):
    from .safe_v1_3_0 import abi
    return w3.eth.contract(
        address=address,
        abi=abi
    )


def get_compatibility_fallback_handler_contract(w3: Web3, address: ChecksumAddress):
    from .compatibility_fallback_handler_v1_3_0 import abi
    return w3.eth.contract(
        address=address,
        abi=abi
    )


def get_multi_send_contract(w3: Web3, address: ChecksumAddress):
    from .multisend_v1_3_0 import abi
    return w3.eth.contract(
        address=address,
        abi=abi
    )

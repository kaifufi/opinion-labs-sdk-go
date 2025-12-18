from eth_typing import ChecksumAddress, HexAddress, HexStr, Hash32
from enum import Enum

NULL_ADDRESS: ChecksumAddress = ChecksumAddress(HexAddress(HexStr("0x" + "0" * 40)))
NULL_HASH: Hash32 = Hash32(bytes.fromhex(HexStr("0" * 64)))

# Gas required to transfer a regular token. Assume the worst scenario with a regular token transfer without storage
# initialized (payment_receiver no previous owner of token)
TOKEN_TRANSFER_GAS = 60_000


class TxSpeed(Enum):
    SLOWEST = 0
    VERY_SLOW = 1
    SLOW = 2
    NORMAL = 3
    FAST = 4
    VERY_FAST = 5
    FASTEST = 6

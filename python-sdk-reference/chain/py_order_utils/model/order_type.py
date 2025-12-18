from enum import IntEnum  # Changed from Enum to IntEnum

class OrderType(IntEnum):  # Inherits from IntEnum
    MARKET_ORDER = 1
    LIMIT_ORDER = 2

LIMIT_ORDER = 2
MARKET_ORDER = 1


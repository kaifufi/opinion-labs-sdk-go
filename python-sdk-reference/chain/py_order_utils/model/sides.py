from enum import IntEnum  # Changed from Enum to IntEnum

class OrderSide(IntEnum):  # Inherits from IntEnum
    BUY = 0
    SELL = 1

BUY = OrderSide.BUY
SELL = OrderSide.SELL

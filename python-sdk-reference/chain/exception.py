class BalanceNotEnough(Exception):
    pass


class NoPositionsToRedeem(Exception):
    pass


class InsufficientGasBalance(Exception):
    """Raised when the signer doesn't have enough gas tokens to execute a transaction"""
    pass

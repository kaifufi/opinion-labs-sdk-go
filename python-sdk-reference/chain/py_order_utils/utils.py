import math
import logging
from eth_utils import to_checksum_address
from string import punctuation
from random import random
from datetime import datetime, timezone
from .model.sides import OrderSide
from decimal import Decimal, ROUND_DOWN

max_int = math.pow(2, 32)


def normalize(s: str) -> str:
    lowered = s.lower()
    for p in punctuation:
        lowered = lowered.replace(p, "")
    return lowered


def normalize_address(address: str) -> str:
    return to_checksum_address(address)


def generate_seed() -> int:
    """
    Pseudo random seed
    """
    now = datetime.now().replace(tzinfo=timezone.utc).timestamp()
    return round(now * random())


def round_to_significant_digits(value: int, n: int) -> int:
    """
    Round an integer to n significant digits.

    Args:
        value: The integer to round
        n: Number of significant digits to keep

    Returns:
        The rounded integer

    Example:
        round_to_significant_digits(123456789, 6) -> 123457000
        round_to_significant_digits(10000000000000000000, 6) -> 10000000000000000000
    """
    if value == 0:
        return 0

    # Get the magnitude (number of digits)
    magnitude = len(str(abs(value)))

    # If already n digits or fewer, return as-is
    if magnitude <= n:
        return value

    # Calculate the divisor to round to n significant digits
    divisor = 10 ** (magnitude - n)

    # Round to n significant digits
    rounded = round(value / divisor) * divisor

    return int(rounded)


def prepend_zx(in_str: str) -> str:
    """
    Prepend 0x to the input string if it is missing
    """
    s = in_str
    if len(s) > 2 and s[:2] != "0x":
        s = f"0x{s}"
    return s


def calculate_order_amounts(price: float, maker_amount: int, side: OrderSide, decimals: int) -> tuple[int, int]:
    """
    Calculate the maker and taker amounts based on the price and side.

    Uses precise Decimal arithmetic and ensures the calculated price from amounts
    exactly matches the input price by using fractional representation.

    Args:
        price: The price of the order (between 0.001 and 0.999)
        maker_amount: The maker amount in base units
        side: The order side (BUY or SELL)
        decimals: The number of decimal places for the currency (unused, kept for compatibility)

    Returns:
        tuple[int, int]: A tuple containing (recalculated_maker_amount, taker_amount)
        For BUY: price = maker/taker, so taker = maker/price
        For SELL: price = taker/maker, so taker = maker*price
    """

    # Validate price using Decimal for exact comparison
    try:
        price_decimal = Decimal(str(price))
    except (ValueError, TypeError) as e:
        raise ValueError(f"Invalid price format: {price}") from e

    # Define min/max price as Decimal for exact comparison
    min_price = Decimal("0.001")
    max_price = Decimal("0.999")

    if not (min_price <= price_decimal <= max_price):
        raise ValueError(f"Price must be between {min_price} and {max_price} (inclusive), got {price}")

    # Ensure price doesn't have excessive precision (max 6 decimal places to match engine precision)
    if price_decimal.as_tuple().exponent < -6:
        raise ValueError(f"Price precision cannot exceed 6 decimal places, got {price}")

    # Convert price to fraction for exact representation
    # E.g., 0.111 = 111/1000, 0.222 = 222/1000 = 111/500 (simplified)
    from fractions import Fraction
    import math

    price_fraction = Fraction(str(price)).limit_denominator(1000000)  # Limit denominator to avoid huge numbers

    if side == OrderSide.BUY:
        # For BUY: price = maker/taker
        # Goal: Ensure maker/taker = price_num/price_denom EXACTLY
        #
        # Strategy:
        # 1. Round maker to 4 significant digits
        # 2. Find the closest (maker', taker') pair where:
        #    maker'/taker' = price_num/price_denom exactly
        #    maker' is close to maker_4digit
        #
        # Method: maker' = k * price_num, taker' = k * price_denom
        # Choose k such that maker' ≈ maker_4digit

        # Step 1: Round maker to 4 significant digits as target
        maker_4digit = round_to_significant_digits(maker_amount, 4)

        # Step 2: Find the scaling factor k
        # We want: k * price_num ≈ maker_4digit
        # So: k ≈ maker_4digit / price_num
        k = maker_4digit // price_fraction.numerator
        if k == 0:
            k = 1  # Ensure at least k=1

        # Step 3: Calculate exact maker and taker using k
        # This guarantees: recalculated_maker_amount / taker_amount = price_num / price_denom
        recalculated_maker_amount = k * price_fraction.numerator
        taker_amount = k * price_fraction.denominator

    else:  # SELL
        # For SELL: price = taker/maker
        # Goal: Ensure taker/maker = price_num/price_denom EXACTLY
        #
        # Method: taker = k * price_num, maker = k * price_denom
        # Choose k such that maker ≈ maker_4digit

        # Step 1: Round maker to 4 significant digits
        maker_4digit = round_to_significant_digits(maker_amount, 4)

        # Step 2: Find the scaling factor k
        # We want: k * price_denom ≈ maker_4digit
        # So: k ≈ maker_4digit / price_denom
        k = maker_4digit // price_fraction.denominator
        if k == 0:
            k = 1

        # Step 3: Calculate exact maker and taker using k
        recalculated_maker_amount = k * price_fraction.denominator
        taker_amount = k * price_fraction.numerator

    # Ensure amounts are at least 1
    taker_amount = int(max(1, taker_amount))
    recalculated_maker_amount = int(max(1, recalculated_maker_amount))

    logging.debug(f"Order calculation: taker_amount={taker_amount}, recalculated_maker_amount={recalculated_maker_amount}")

    # Validate the calculated price is within bounds
    calculated_price = recalculated_maker_amount / taker_amount if side == OrderSide.BUY else taker_amount / recalculated_maker_amount

    if calculated_price > 0.999 or calculated_price < 0.001:
        raise ValueError("invalid taker_amount and recalculated_maker_amount")

    return recalculated_maker_amount, taker_amount
from typing import List, Optional, Dict, Any, Tuple
import logging
import math
from time import time
from decimal import Decimal

from eth_typing import ChecksumAddress, HexStr
from opinion_api.api.prediction_market_api import PredictionMarketApi
from opinion_api.api.user_api import UserApi
from opinion_api.api_client import ApiClient
from opinion_api.configuration import Configuration
from opinion_clob_sdk.chain.safe.utils import fast_to_checksum_address
from .chain.contract_caller import ContractCaller
from .chain.py_order_utils.builders.order_builder import OrderBuilder
from .chain.py_order_utils.model.order import OrderDataInput, OrderData, PlaceOrderDataInput
from .chain.py_order_utils.constants import ZERO_ADDRESS, ZX
from .chain.py_order_utils.model.signatures import POLY_GNOSIS_SAFE
from .chain.py_order_utils.model.sides import BUY, SELL, OrderSide
from .chain.py_order_utils.model.order_type import LIMIT_ORDER, MARKET_ORDER
from .model import TopicStatus, TopicStatusFilter, TopicType, TopicSortType
from .chain.py_order_utils.utils import calculate_order_amounts
from .config import DEFAULT_CONTRACT_ADDRESSES

API_INTERNAL_ERROR_MSG = "Unable to process your request. Please contact technical support."
MISSING_MARKET_ID_MSG = "market_id is required."
MISSING_TOKEN_ID_MSG = "token_id is required."
MAX_DECIMALS = 18  # Standard maximum for ERC20 tokens (ETH, DAI, etc.)

# Supported blockchain chain IDs
CHAIN_ID_BNB_MAINNET = 56  # BNB Chain (BSC) mainnet
SUPPORTED_CHAIN_IDS = [CHAIN_ID_BNB_MAINNET]

class InvalidParamError(Exception):
    pass

class OpenApiError(Exception):
    pass

def safe_amount_to_wei(amount: float, decimals: int) -> int:
    """
    Safely convert human-readable amount to wei units without precision loss.

    Args:
        amount: Human-readable amount (e.g., 1.5 for 1.5 tokens)
        decimals: Token decimals (e.g., 6 for USDC, 18 for ETH)

    Returns:
        Integer amount in wei units

    Raises:
        InvalidParamError: If amount or decimals are invalid
    """
    if amount <= 0:
        raise InvalidParamError(f"Amount must be positive, got: {amount}")

    if decimals < 0 or decimals > MAX_DECIMALS:
        raise InvalidParamError(f"Decimals must be between 0 and {MAX_DECIMALS}, got: {decimals}")

    # Use Decimal for exact calculation
    amount_decimal = Decimal(str(amount))
    multiplier = Decimal(10) ** decimals

    result_decimal = amount_decimal * multiplier

    # Convert to int
    result = int(result_decimal)

    # Validate result fits in uint256
    if result >= 2**256:
        raise InvalidParamError(f"Amount too large for uint256: {result}")

    if result <= 0:
        raise InvalidParamError(f"Calculated amount is zero or negative: {result}")

    return result

class Client:
    def __init__(
        self,
        host: str = '',
        apikey: str = '',
        chain_id: Optional[int] = None,
        rpc_url: str = '',
        private_key: HexStr = HexStr(''),
        multi_sig_addr: str = '',
        conditional_tokens_addr: Optional[ChecksumAddress] = None,
        multisend_addr: Optional[ChecksumAddress] = None,
        fee_manager_addr: Optional[ChecksumAddress] = None,
        enable_trading_check_interval: int = 3600,
        quote_tokens_cache_ttl: int = 3600,
        market_cache_ttl: int = 300
    ) -> None:
        """
        Initialize the Opinion CLOB SDK client.

        Args:
            host: API host URL
            apikey: API authentication key
            chain_id: Blockchain chain ID (56 for BNB Chain mainnet)
            rpc_url: RPC endpoint URL
            private_key: Private key for signing transactions
            multi_sig_addr: Multi-signature wallet address
            conditional_tokens_addr: Conditional tokens contract address (optional, uses default if not provided)
            multisend_addr: Multisend contract address (optional, uses default if not provided)
            fee_manager_addr: Fee manager contract address (optional, uses default if not provided)
            enable_trading_check_interval: Time interval (in seconds) to cache enable_trading checks.
                Default is 3600 (1 hour). Set to 0 to check every time.
                This significantly improves performance for frequent operations.
            quote_tokens_cache_ttl: Time interval (in seconds) to cache quote tokens data.
                Default is 3600 (1 hour). Set to 0 to disable caching.
            market_cache_ttl: Time interval (in seconds) to cache market data.
                Default is 300 (5 minutes). Set to 0 to disable caching.
        """
        # Validate and set chain_id first
        if chain_id not in SUPPORTED_CHAIN_IDS:
            raise InvalidParamError(f'chain_id must be one of {SUPPORTED_CHAIN_IDS}')
        self.chain_id = chain_id

        # Use default contract addresses if not provided
        if conditional_tokens_addr is None:
            conditional_tokens_addr = ChecksumAddress(
                DEFAULT_CONTRACT_ADDRESSES[chain_id]["conditional_tokens"]
            )
        if multisend_addr is None:
            multisend_addr = ChecksumAddress(
                DEFAULT_CONTRACT_ADDRESSES[chain_id]["multisend"]
            )
        if fee_manager_addr is None:
            fee_manager_addr = ChecksumAddress(
                DEFAULT_CONTRACT_ADDRESSES[chain_id]["fee_manager"]
            )

        self.conf = Configuration(host=host)
        self.conf.api_key['ApiKeyAuth'] = apikey
        self.api_key = apikey
        multi_sig_addr = fast_to_checksum_address(multi_sig_addr)
        self.contract_caller = ContractCaller(rpc_url=rpc_url, private_key=private_key, multi_sig_addr=multi_sig_addr,
                                              conditional_tokens_addr=conditional_tokens_addr,
                                              multisend_addr=multisend_addr,
                                              fee_manager_addr=fee_manager_addr,
                                              enable_trading_check_interval=enable_trading_check_interval)
        self.api_client = ApiClient(self.conf)
        self.market_api = PredictionMarketApi(self.api_client)
        self.user_api = UserApi(self.api_client)

        # Cache configuration
        self.quote_tokens_cache_ttl = quote_tokens_cache_ttl
        self.market_cache_ttl = market_cache_ttl
        self._quote_tokens_cache: Optional[Any] = None
        self._quote_tokens_cache_time: float = 0
        self._market_cache: Dict[int, Tuple[Any, float]] = {}  # market_id -> (data, timestamp)

    def _validate_market_response(self, response: Any, operation_name: str = "operation") -> Any:
        """Validate and extract market data from API response"""
        if hasattr(response, 'errno') and response.errno != 0:
            raise OpenApiError(f"Failed to {operation_name}: {response}")

        if not hasattr(response, 'result') or not hasattr(response.result, 'data'):
            raise OpenApiError(f"Invalid response format for {operation_name}")

        return response.result.data

    def _parse_list_response(self, response: Any, operation_name: str = "operation") -> List[Any]:
        """Parse response that contains a list"""
        if hasattr(response, 'errno') and response.errno != 0:
            raise OpenApiError(f"Failed to {operation_name}: {response}")

        if not hasattr(response, 'result') or not hasattr(response.result, 'list'):
            raise OpenApiError(f"Invalid list response format for {operation_name}")

        return response.result.list

    def _format_transaction_result(self, tx_hash: Any, safe_tx_hash: Any, return_value: Any) -> Tuple[str, str, Optional[str]]:
        """Convert transaction result to JSON-serializable format

        Args:
            tx_hash: Transaction hash (HexBytes or similar)
            safe_tx_hash: Safe transaction hash (HexBytes or similar)
            return_value: Contract call return value (bytes or similar)

        Returns:
            Tuple of (tx_hash_hex, safe_tx_hash_hex, return_value_hex)
            where all values are hex strings or None
        """
        # Convert tx_hash to hex string
        tx_hash_hex = tx_hash.hex() if hasattr(tx_hash, 'hex') and tx_hash else None

        # Convert safe_tx_hash to hex string
        safe_tx_hash_hex = safe_tx_hash.hex() if hasattr(safe_tx_hash, 'hex') and safe_tx_hash else None

        # Convert return_value (bytes) to hex string
        return_value_hex = None
        if return_value is not None:
            if hasattr(return_value, 'hex'):
                return_value_hex = return_value.hex()
            elif isinstance(return_value, bytes):
                return_value_hex = '0x' + return_value.hex()

        return tx_hash_hex, safe_tx_hash_hex, return_value_hex

    def enable_trading(self) -> Tuple[Optional[str], Optional[str], Optional[str]]:
        """Enable trading by approving necessary tokens.

        Returns:
            Tuple of (tx_hash, safe_tx_hash, return_value) as hex strings
        """
        quote_token_list_response = self.get_quote_tokens()
        quote_token_list = self._parse_list_response(quote_token_list_response, "get quote tokens")

        supported_quote_tokens: dict = {}

        # for each quote token, check if the chain_id is the same as the chain_id in the contract_caller
        for quote_token in quote_token_list:
            quote_token_address = fast_to_checksum_address(quote_token.quote_token_address)
            ctf_exchange_address = fast_to_checksum_address(quote_token.ctf_exchange_address)
            supported_quote_tokens[quote_token_address] = ctf_exchange_address

        logging.info(f'Supported quote tokens: {supported_quote_tokens}')
        if len(supported_quote_tokens) == 0:
            raise OpenApiError('No supported quote tokens found')
        tx_hash, safe_tx_hash, return_value = self.contract_caller.enable_trading(supported_quote_tokens)
        return self._format_transaction_result(tx_hash, safe_tx_hash, return_value)

    def split(self, market_id: int, amount: int, check_approval: bool = True) -> Tuple[Optional[str], Optional[str], Optional[str]]:
        """Split collateral into outcome tokens for a market.

        Args:
            market_id: The market ID to split tokens for (required)
            amount: Amount of collateral to split in wei (required)
            check_approval: Whether to check and enable trading approvals first

        Returns:
            Tuple of (tx_hash, safe_tx_hash, return_value) as hex strings
        """
        try:
            if not market_id or market_id <= 0:
                raise InvalidParamError("market_id must be a positive integer")
            if not amount or amount <= 0:
                raise InvalidParamError("amount must be a positive integer")

            # Enable trading first for all trade operations.
            if check_approval:
                self.enable_trading()

            topic_detail = self.get_market(market_id)
            market_data = self._validate_market_response(topic_detail, "get market for split")

            if int(market_data.chain_id) != self.chain_id:
                raise OpenApiError('Cannot split on different chain')

            status = market_data.status
            if not (status == TopicStatus.ACTIVATED.value or status == TopicStatus.RESOLVED.value or status == TopicStatus.RESOLVING.value):
                raise OpenApiError('Cannot split on non-activated/resolving/resolved market')
            collateral = fast_to_checksum_address(market_data.quote_token)
            condition_id = market_data.condition_id

            tx_hash, safe_tx_hash, return_value = self.contract_caller.split(collateral_token=collateral, condition_id=bytes.fromhex(condition_id), amount=amount)
            return self._format_transaction_result(tx_hash, safe_tx_hash, return_value)
        except InvalidParamError as e:
            logging.error(f"Validation error in split: {e}")
            raise
        except OpenApiError as e:
            logging.error(f"API error in split: {e}")
            raise
        except Exception as e:
            logging.error(f"Unexpected error in split: {e}")
            raise OpenApiError(f"Failed to split collateral: {e}")

    def merge(self, market_id: int, amount: int, check_approval: bool = True) -> Tuple[Optional[str], Optional[str], Optional[str]]:
        """Merge outcome tokens back into collateral for a market.

        Args:
            market_id: The market ID to merge tokens for (required)
            amount: Amount of outcome tokens to merge in wei (required)
            check_approval: Whether to check and enable trading approvals first

        Returns:
            Tuple of (tx_hash, safe_tx_hash, return_value) as hex strings
        """
        try:
            if not market_id or market_id <= 0:
                raise InvalidParamError("market_id must be a positive integer")
            if not amount or amount <= 0:
                raise InvalidParamError("amount must be a positive integer")

            # Enable trading first for all trade operations.
            if check_approval:
                self.enable_trading()

            topic_detail = self.get_market(market_id)
            market_data = self._validate_market_response(topic_detail, "get market for merge")

            if int(market_data.chain_id) != self.chain_id:
                raise OpenApiError('Cannot merge on different chain')

            status = market_data.status
            if not (status == TopicStatus.ACTIVATED.value or status == TopicStatus.RESOLVED.value or status == TopicStatus.RESOLVING.value):
                raise OpenApiError('Cannot merge on non-activated/resolving/resolved market')
            collateral = fast_to_checksum_address(market_data.quote_token)
            condition_id = market_data.condition_id

            tx_hash, safe_tx_hash, return_value = self.contract_caller.merge(collateral_token=collateral, condition_id=bytes.fromhex(condition_id),
                                              amount=amount)
            return self._format_transaction_result(tx_hash, safe_tx_hash, return_value)
        except InvalidParamError as e:
            logging.error(f"Validation error in merge: {e}")
            raise
        except OpenApiError as e:
            logging.error(f"API error in merge: {e}")
            raise
        except Exception as e:
            logging.error(f"Unexpected error in merge: {e}")
            raise OpenApiError(f"Failed to merge tokens: {e}")

    def redeem(self, market_id: int, check_approval: bool = True) -> Tuple[Optional[str], Optional[str], Optional[str]]:
        """Redeem winning outcome tokens for collateral after market resolution.

        Args:
            market_id: The market ID to redeem tokens for (required)
            check_approval: Whether to check and enable trading approvals first

        Returns:
            Tuple of (tx_hash, safe_tx_hash, return_value) as hex strings
        """
        try:
            if not market_id or market_id <= 0:
                raise InvalidParamError("market_id must be a positive integer")

            # Enable trading first for all trade operations.
            if check_approval:
                self.enable_trading()

            topic_detail = self.get_market(market_id)
            market_data = self._validate_market_response(topic_detail, "get market for redeem")

            if int(market_data.chain_id) != self.chain_id:
                raise OpenApiError('Cannot redeem on different chain')

            status = market_data.status
            if not status == TopicStatus.RESOLVED.value:
                raise OpenApiError('Cannot redeem on non-resolved market')
            collateral = fast_to_checksum_address(market_data.quote_token)
            condition_id = market_data.condition_id
            logging.info(f'Redeem: collateral={collateral}, condition_id={condition_id}')
            tx_hash, safe_tx_hash, return_value = self.contract_caller.redeem(collateral_token=collateral, condition_id=bytes.fromhex(condition_id))
            return self._format_transaction_result(tx_hash, safe_tx_hash, return_value)
        except InvalidParamError as e:
            logging.error(f"Validation error in redeem: {e}")
            raise
        except OpenApiError as e:
            logging.error(f"API error in redeem: {e}")
            raise
        except Exception as e:
            logging.error(f"Unexpected error in redeem: {e}")
            raise OpenApiError(f"Failed to redeem tokens: {e}")

    def get_quote_tokens(self, use_cache: bool = True) -> Any:
        """Get list of supported quote tokens

        Args:
            use_cache: Whether to use cached data if available (default True).
                Set to False to force a fresh API call.
        """
        current_time = time()

        # Check cache if enabled
        if use_cache and self.quote_tokens_cache_ttl > 0:
            if self._quote_tokens_cache is not None:
                cache_age = current_time - self._quote_tokens_cache_time
                if cache_age < self.quote_tokens_cache_ttl:
                    logging.debug(f"Using cached quote tokens (age: {cache_age:.1f}s)")
                    return self._quote_tokens_cache

        # Fetch fresh data
        logging.debug("Fetching fresh quote tokens from API")
        result = self.market_api.openapi_quote_token_get(apikey=self.api_key, chain_id=str(self.chain_id))

        # Update cache
        if self.quote_tokens_cache_ttl > 0:
            self._quote_tokens_cache = result
            self._quote_tokens_cache_time = current_time

        return result

    # Deprecated: use get_quote_tokens() instead
    def get_currencies(self) -> Any:
        """Deprecated: Use get_quote_tokens() instead"""
        return self.get_quote_tokens()

    def get_markets(
        self,
        topic_type: TopicType = TopicType.ALL,
        page: int = 1,
        limit: int = 20,
        status: Optional[TopicStatusFilter] = None,
        sort_by: Optional['TopicSortType'] = None
    ) -> Any:
        """Get markets with pagination support.

        Args:
            topic_type: Filter by topic type. Defaults to TopicType.ALL (2).
                - TopicType.ALL (2): All Markets (default)
                - TopicType.CATEGORICAL (1): Categorical Market
                - TopicType.BINARY (0): Binary Market
            page: Page number (>= 1)
            limit: Number of items per page (1-20)
            status: Optional filter by status
            sort_by: Optional sort type (TopicSortType enum). Defaults to BY_TIME_DESC if not specified.
        """
        if page < 1:
            raise InvalidParamError("page must be >= 1")
        if not 1 <= limit <= 20:
            raise InvalidParamError("limit must be between 1 and 20")

        # Default to BY_TIME_DESC (1) if sort_by is not specified
        sort_value = sort_by.value if sort_by else TopicSortType.BY_TIME_DESC.value

        result = self.market_api.openapi_market_get(
            apikey=self.api_key,
            market_type=topic_type.value if topic_type else None,
            page=page,
            limit=limit,
            chain_id=str(self.chain_id),
            status=status.value if status else None,
            sort_by=sort_value
        )
        return result

    def get_market(self, market_id, use_cache: bool = True):
        """Get detailed information about a specific market

        Args:
            market_id: The market ID to query
            use_cache: Whether to use cached data if available (default True).
                Set to False to force a fresh API call.
        """
        try:
            if not market_id:
                raise InvalidParamError(MISSING_MARKET_ID_MSG)

            current_time = time()

            # Check cache if enabled
            if use_cache and self.market_cache_ttl > 0:
                if market_id in self._market_cache:
                    cached_data, cache_time = self._market_cache[market_id]
                    cache_age = current_time - cache_time
                    if cache_age < self.market_cache_ttl:
                        logging.debug(f"Using cached market {market_id} (age: {cache_age:.1f}s)")
                        return cached_data

            # Fetch fresh data
            logging.debug(f"Fetching fresh market {market_id} from API")
            result = self.market_api.openapi_market_market_id_get(apikey=self.api_key, market_id=market_id)

            # Update cache
            if self.market_cache_ttl > 0:
                self._market_cache[market_id] = (result, current_time)

            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get market: {e}")

    def get_categorical_market(self, market_id):
        """Get detailed information about a categorical market"""
        try:
            if not market_id:
                raise InvalidParamError(MISSING_MARKET_ID_MSG)

            result = self.market_api.openapi_market_categorical_market_id_get(apikey=self.api_key, market_id=market_id)
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get categorical market: {e}")

    def get_price_history(self, token_id, interval="1h", start_at=None, end_at=None):
        """Get price history/candlestick data for a token

        Args:
            token_id: Token ID
            interval: Price data interval: 1m, 1h, 1d, 1w, max (default: 1h)
            start_at: Start timestamp in Unix seconds (optional)
            end_at: End timestamp in Unix seconds (optional)
        """
        try:
            if not token_id:
                raise InvalidParamError(MISSING_TOKEN_ID_MSG)

            if not interval:
                raise InvalidParamError('interval is required')

            result = self.market_api.openapi_token_price_history_get(
                apikey=self.api_key,
                token_id=token_id,
                interval=interval,
                start_at=start_at,
                end_at=end_at
            )
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get price history: {e}")

    def get_orderbook(self, token_id):
        """Get orderbook for a specific token"""
        try:
            if not token_id:
                raise InvalidParamError(MISSING_TOKEN_ID_MSG)

            result = self.market_api.openapi_token_orderbook_get(apikey=self.api_key, token_id=token_id)
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get orderbook: {e}")

    def get_latest_price(self, token_id):
        """Get latest price for a token"""
        try:
            if not token_id:
                raise InvalidParamError(MISSING_TOKEN_ID_MSG)

            result = self.market_api.openapi_token_latest_price_get(apikey=self.api_key, token_id=token_id)
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get latest price: {e}")

    def get_fee_rates(self, token_id):
        """Get fee rates for a token from FeeManager contract on-chain

        Args:
            token_id: Token ID to query fee rates for

        Returns:
            Dictionary containing:
                - maker_max_fee_rate: Maker maximum fee rate as decimal (e.g., 0.02 = 2%)
                - taker_max_fee_rate: Taker maximum fee rate as decimal (e.g., 0.02 = 2%)
                - enabled: Whether fee is enabled
        """
        try:
            if not token_id:
                raise InvalidParamError(MISSING_TOKEN_ID_MSG)

            return self.contract_caller.get_fee_rate_settings(token_id)
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"Chain query error: {e}")
            raise OpenApiError(f"Failed to get fee rates from chain: {e}")

    def _place_order(self, data: OrderDataInput, exchange_addr='', chain_id=0, currency_addr='', currency_decimal=0, check_approval=False):
        if check_approval:
            self.enable_trading()
        try:
            if not exchange_addr:
                raise InvalidParamError('exchange_addr is required')

            # Validate currency_decimal to prevent overflow
            if currency_decimal < 0:
                raise InvalidParamError(f'currency_decimal must be non-negative, got: {currency_decimal}')
            if currency_decimal > MAX_DECIMALS:
                raise InvalidParamError(f'currency_decimal too large (max {MAX_DECIMALS}): {currency_decimal}')

            chain_id = self.chain_id

            builder = OrderBuilder(exchange_addr, chain_id, self.contract_caller.signer)
            takerAmount = 0

            if data.orderType == MARKET_ORDER:
                takerAmount = 0
                data.price = "0"
                # Use safe conversion to avoid precision loss
                recalculated_maker_amount = safe_amount_to_wei(data.makerAmount, currency_decimal)
            if data.orderType == LIMIT_ORDER:
                # Use safe conversion to avoid precision loss
                maker_amount_wei = safe_amount_to_wei(data.makerAmount, currency_decimal)
                recalculated_maker_amount, takerAmount = calculate_order_amounts(
                    price=float(data.price),
                    maker_amount=maker_amount_wei,
                    side=data.side,
                    decimals=currency_decimal
                )

            order_data = OrderData(
                maker=self.contract_caller.multi_sig_addr,
                taker=ZERO_ADDRESS,
                tokenId=data.tokenId,
                makerAmount=recalculated_maker_amount,
                takerAmount=takerAmount,
                feeRateBps='0',
                side=data.side,
                signatureType=POLY_GNOSIS_SAFE,
                signer=self.contract_caller.signer.address()
            )
            signerOrder = builder.build_signed_order(order_data)

            order_dict = signerOrder.order.dict()

            # Create V2AddOrderReq object for opinion_api
            from opinion_api.models.v2_add_order_req import V2AddOrderReq

            v2_add_order_req = V2AddOrderReq(
                salt=str(order_dict['salt']),
                topic_id=data.marketId,
                maker=order_dict['maker'],
                signer=order_dict['signer'],
                taker=order_dict['taker'],
                token_id=str(order_dict['tokenId']),
                maker_amount=str(order_dict['makerAmount']),
                taker_amount=str(order_dict['takerAmount']),
                expiration=str(order_dict['expiration']),
                nonce=str(order_dict['nonce']),
                fee_rate_bps=str(order_dict['feeRateBps']),
                side=str(order_dict['side']),
                signature_type=str(order_dict['signatureType']),
                signature=signerOrder.signature,
                sign=signerOrder.signature,
                contract_address="",
                currency_address=currency_addr,
                price=data.price,
                trading_method=int(data.orderType),
                timestamp=int(time()),
                safe_rate='0',
                order_exp_time='0'
            )

            result = self.market_api.openapi_order_post(apikey=self.api_key, add_order_req=v2_add_order_req)
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to place order: {e}")

    def place_order(self, data: PlaceOrderDataInput, check_approval=False):
        """Place an order on the market"""
        quote_token_list_response = self.get_quote_tokens()
        quote_token_list = self._parse_list_response(quote_token_list_response, "get quote tokens")

        market_response = self.get_market(data.marketId)
        market = self._validate_market_response(market_response, "get market for place order")

        if int(market.chain_id) != self.chain_id:
            raise OpenApiError('Cannot place order on different chain')

        quote_token_addr = market.quote_token

        # find quote token from quote_token_list by quote_token_address
        quote_token = next((item for item in quote_token_list if str.lower(item.quote_token_address) == str.lower(quote_token_addr)), None)
        if not quote_token:
            raise OpenApiError('Quote token not found for this market')

        exchange_addr = quote_token.ctf_exchange_address
        chain_id = quote_token.chain_id

        makerAmount = 0
        minimal_maker_amount = 1

        # reject if market buy and makerAmountInBaseToken is provided
        if(data.side == OrderSide.BUY and data.orderType == MARKET_ORDER and data.makerAmountInBaseToken):
            raise InvalidParamError('makerAmountInBaseToken is not allowed for market buy')

        # reject if market sell and makerAmountInQuoteToken is provided
        if(data.side == OrderSide.SELL and data.orderType == MARKET_ORDER and data.makerAmountInQuoteToken):
            raise InvalidParamError('makerAmountInQuoteToken is not allowed for market sell')

        # Validate price for limit orders (prevent division by zero)
        if data.orderType == LIMIT_ORDER:
            try:
                price_decimal = Decimal(str(data.price))
                if price_decimal <= 0:
                    raise InvalidParamError(f'Price must be positive for limit orders, got: {data.price}')
            except (ValueError, TypeError, InvalidParamError):
                raise
            except Exception as e:
                raise InvalidParamError(f'Invalid price format: {data.price}') from e

        # need amount to be in quote token
        if(data.side == OrderSide.BUY):
            # e.g. yes/no
            if(data.makerAmountInBaseToken):
                # Use Decimal for precise calculation to avoid floating point errors
                base_amount = Decimal(str(data.makerAmountInBaseToken))
                price_decimal = Decimal(str(data.price))
                makerAmount = float(base_amount * price_decimal)
                # makerAmountInBaseToken should be at least 1 otherwise throw error
                if(float(data.makerAmountInBaseToken) < minimal_maker_amount):
                    raise InvalidParamError("makerAmountInBaseToken must be at least 1")
            # e.g. usdc
            elif(data.makerAmountInQuoteToken):
                makerAmount = float(data.makerAmountInQuoteToken)
                # makerAmountInQuoteToken should be at least 1 otherwise throw error
                if(float(data.makerAmountInQuoteToken) < minimal_maker_amount):
                    raise InvalidParamError("makerAmountInQuoteToken must be at least 1")
            else:
                raise InvalidParamError("Either makerAmountInBaseToken or makerAmountInQuoteToken must be provided for BUY orders")

        elif(data.side == OrderSide.SELL):
            # e.g. yes/no
            if(data.makerAmountInBaseToken):
                makerAmount = float(data.makerAmountInBaseToken)
                # makerAmountInBaseToken should be at least 1 otherwise throw error
                if(float(data.makerAmountInBaseToken) < minimal_maker_amount):
                    raise InvalidParamError("makerAmountInBaseToken must be at least 1")
            # e.g. usdc
            elif(data.makerAmountInQuoteToken):
                # Use Decimal for precise division to avoid floating point errors
                quote_amount = Decimal(str(data.makerAmountInQuoteToken))
                price_decimal = Decimal(str(data.price))
                if price_decimal == 0:
                    raise InvalidParamError("Price cannot be zero for SELL orders with makerAmountInQuoteToken")
                makerAmount = float(quote_amount / price_decimal)
                # makerAmountInQuoteToken should be at least 1 otherwise throw error
                if(float(data.makerAmountInQuoteToken) < minimal_maker_amount):
                    raise InvalidParamError("makerAmountInQuoteToken must be at least 1")
            else:
                raise InvalidParamError("Either makerAmountInBaseToken or makerAmountInQuoteToken must be provided for SELL orders")

        # Final validation: ensure makerAmount was properly calculated
        if makerAmount <= 0:
            raise InvalidParamError(f"Calculated makerAmount must be positive, got: {makerAmount}")


        input = OrderDataInput(
            marketId=data.marketId,
            tokenId=data.tokenId,
            makerAmount=makerAmount,
            price=data.price,
            orderType=data.orderType,
            side=data.side
        )

        return self._place_order(input, exchange_addr, chain_id, quote_token_addr, int(quote_token.decimal), check_approval)

    def cancel_order(self, order_id):
        """
        Cancel an existing order.

        Args:
            order_id: Order ID to cancel (str)

        Returns:
            API response for order cancellation
        """
        if not order_id or not isinstance(order_id, str):
            raise InvalidParamError('order_id must be a non-empty string')

        from opinion_api.models.openapi_cancel_order_request_open_api import OpenapiCancelOrderRequestOpenAPI

        # Use order_id for API request
        request_body = OpenapiCancelOrderRequestOpenAPI(order_id=order_id)
        result = self.market_api.openapi_order_cancel_post(apikey=self.api_key, cancel_order_req=request_body)
        return result

    def place_orders_batch(self, orders: List[PlaceOrderDataInput], check_approval: bool = False) -> List[Any]:
        """
        Place multiple orders in batch to reduce API calls.

        Args:
            orders: List of PlaceOrderDataInput objects
            check_approval: Whether to check and enable trading approvals first (done once for all orders)

        Returns:
            List of order placement results

        Raises:
            InvalidParamError: If orders list is empty or invalid
        """
        if not orders or not isinstance(orders, list):
            raise InvalidParamError('orders must be a non-empty list')

        if len(orders) == 0:
            raise InvalidParamError('orders list cannot be empty')

        # Enable trading once for all orders if needed
        if check_approval:
            self.enable_trading()

        results = []
        errors = []

        for i, order in enumerate(orders):
            try:
                # Place each order without checking approval again
                result = self.place_order(order, check_approval=False)
                results.append({
                    'index': i,
                    'success': True,
                    'result': result,
                    'order': order
                })
            except Exception as e:
                logging.error(f"Failed to place order at index {i}: {e}")
                errors.append({
                    'index': i,
                    'success': False,
                    'error': str(e),
                    'order': order
                })
                results.append({
                    'index': i,
                    'success': False,
                    'error': str(e),
                    'order': order
                })

        if errors:
            logging.warning(f"Batch order placement completed with {len(errors)} errors out of {len(orders)} orders")

        return results

    def cancel_orders_batch(self, order_ids: List[str]) -> List[Any]:
        """
        Cancel multiple orders in batch.

        Args:
            order_ids: List of order IDs to cancel

        Returns:
            List of cancellation results

        Raises:
            InvalidParamError: If order_ids list is empty or invalid
        """
        if not order_ids or not isinstance(order_ids, list):
            raise InvalidParamError('order_ids must be a non-empty list')

        if len(order_ids) == 0:
            raise InvalidParamError('order_ids list cannot be empty')

        results = []
        errors = []

        for i, order_id in enumerate(order_ids):
            try:
                result = self.cancel_order(order_id)
                results.append({
                    'index': i,
                    'success': True,
                    'result': result,
                    'order_id': order_id
                })
            except Exception as e:
                logging.error(f"Failed to cancel order {order_id}: {e}")
                errors.append({
                    'index': i,
                    'success': False,
                    'error': str(e),
                    'order_id': order_id
                })
                results.append({
                    'index': i,
                    'success': False,
                    'error': str(e),
                    'order_id': order_id
                })

        if errors:
            logging.warning(f"Batch order cancellation completed with {len(errors)} errors out of {len(order_ids)} orders")

        return results

    def cancel_all_orders(self, market_id: Optional[int] = None, side: Optional[OrderSide] = None) -> dict:
        """
        Cancel all open orders, optionally filtered by market and/or side.
        Uses pagination to fetch all orders (max 20 per page).

        Args:
            market_id: Optional filter - only cancel orders for this market
            side: Optional filter - only cancel BUY or SELL orders

        Returns:
            Dictionary with cancellation summary: {
                'total_orders': int,
                'cancelled': int,
                'failed': int,
                'results': List[dict]
            }
        """
        # Collect all open orders using pagination
        all_orders_list = []
        page = 1
        limit = 20  # Backend max page size
        max_pages = 100  # Safety limit to prevent infinite loops

        while page <= max_pages:
            # Get orders for current page
            page_orders = self.get_my_orders(
                market_id=market_id if market_id else 0,
                status='1',  # 1 = pending/open orders
                limit=limit,
                page=page
            )

            # Parse response to get order list
            orders_list = self._parse_list_response(page_orders, f"get open orders page {page}")

            if not orders_list or len(orders_list) == 0:
                # No more orders on this page
                break

            all_orders_list.extend(orders_list)

            # If we got fewer orders than the limit, we've reached the last page
            if len(orders_list) < limit:
                break

            page += 1

        if page > max_pages:
            logging.warning(f"Reached maximum page limit ({max_pages}), there may be more orders")

        if not all_orders_list or len(all_orders_list) == 0:
            logging.info("No open orders to cancel")
            return {
                'total_orders': 0,
                'cancelled': 0,
                'failed': 0,
                'results': []
            }

        # Filter by side if specified
        if side:
            all_orders_list = [order for order in all_orders_list if int(order.side) == int(side.value)]

        # Extract order IDs from the response
        order_ids = [order.order_id for order in all_orders_list if hasattr(order, 'order_id')]

        if not order_ids:
            logging.info("No orders match the filter criteria")
            return {
                'total_orders': 0,
                'cancelled': 0,
                'failed': 0,
                'results': []
            }

        logging.info(f"Found {len(order_ids)} orders to cancel")

        # Cancel all orders in batch
        results = self.cancel_orders_batch(order_ids)

        # Count successes and failures
        cancelled = sum(1 for r in results if r.get('success'))
        failed = sum(1 for r in results if not r.get('success'))

        logging.info(f"Cancelled {cancelled} orders, {failed} failed out of {len(order_ids)} total")

        return {
            'total_orders': len(order_ids),
            'cancelled': cancelled,
            'failed': failed,
            'results': results
        }

    def get_my_orders(self, market_id=0, status="", limit=10, page=1):
        """Get user's orders with optional filters"""
        try:
            if not isinstance(market_id, int):
                raise InvalidParamError('market_id must be an integer')

            result = self.market_api.openapi_order_get(
                apikey=self.api_key,
                market_id=market_id if market_id > 0 else None,
                status=status if status else None,
                limit=limit,
                page=page,
                chain_id=str(self.chain_id)
            )
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get orders: {e}")

    def get_order_by_id(self, order_id):
        """Get detailed information about a specific order"""
        try:
            if not order_id or not isinstance(order_id, str):
                raise InvalidParamError('order_id must be a non-empty string')

            result = self.market_api.openapi_order_order_id_get(apikey=self.api_key, order_id=order_id)
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get order by id: {e}")

    def get_my_positions(
        self,
        market_id: int = 0,
        page: int = 1,
        limit: int = 10
    ) -> Any:
        """Get user's positions with optional filters

        Args:
            market_id: Optional filter by market ID (0 for all markets)
            page: Page number (default 1)
            limit: Number of items per page (default 10)
        """
        try:
            if not isinstance(market_id, int):
                raise InvalidParamError('market_id must be an integer')

            if not isinstance(page, int):
                raise InvalidParamError('page must be an integer')

            if not isinstance(limit, int):
                raise InvalidParamError('limit must be an integer')

            result = self.market_api.openapi_positions_get(
                apikey=self.api_key,
                market_id=market_id if market_id > 0 else None,
                page=page,
                limit=limit,
                chain_id=str(self.chain_id)
            )
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get positions: {e}")

    def get_my_balances(self):
        """Get user's balances (uses authenticated user from apikey)"""
        try:
            result = self.market_api.openapi_user_balance_get(
                apikey=self.api_key,
                chain_id=str(self.chain_id)
            )
            return result
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get balances: {e}")

    def get_my_trades(self, market_id=None, page=1, limit=10):
        """Get user's trade history

        Args:
            market_id: Market ID filter (optional)
            page: Page number (default: 1)
            limit: Number of items per page, max 20 (default: 10)
        """
        try:
            if market_id is not None and not isinstance(market_id, int):
                raise InvalidParamError('market_id must be an integer')

            result = self.market_api.openapi_trade_get(
                apikey=self.api_key,
                market_id=market_id,
                page=page,
                limit=limit,
                chain_id=str(self.chain_id)
            )
            return result
        except InvalidParamError as e:
            logging.error(f"Validation error: {e}")
            raise
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get trades: {e}")

    def get_user_auth(self):
        """Get authenticated user information"""
        try:
            result = self.user_api.openapi_user_auth_get(apikey=self.api_key)
            return result
        except Exception as e:
            logging.error(f"API error: {e}")
            raise OpenApiError(f"Failed to get user auth: {e}")

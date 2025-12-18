from typing import List, Any
import time
import logging

from eth_typing import HexStr, ChecksumAddress, Hash32
from hexbytes import HexBytes
from web3 import Web3
from web3.contract import Contract
from web3.providers import HTTPProvider

# Handle both old and new web3.py versions
try:
    from web3.middleware import ExtraDataToPOAMiddleware
    geth_poa_middleware = ExtraDataToPOAMiddleware
except ImportError:
    from web3.middleware import geth_poa_middleware

from .exception import BalanceNotEnough, NoPositionsToRedeem, InsufficientGasBalance
from .safe.constants import NULL_HASH
from .safe.multisend import MultiSendTx, MultiSendOperation
from .safe.safe import Safe
from .py_order_utils.signer import Signer
from .safe.utils import get_empty_tx_params


class ContractCaller:
    def __init__(self, rpc_url='', private_key: HexStr = '', multi_sig_addr: ChecksumAddress = '',
                 conditional_tokens_addr: ChecksumAddress = '', multisend_addr: ChecksumAddress = '',
                 fee_manager_addr: ChecksumAddress = '', enable_trading_check_interval=3600):
        """
        Initialize ContractCaller for blockchain interactions.

        Args:
            rpc_url: RPC endpoint URL
            private_key: Private key for signing transactions
            multi_sig_addr: Multi-signature wallet address
            conditional_tokens_addr: Conditional tokens contract address
            multisend_addr: Multisend contract address
            fee_manager_addr: Fee manager contract address
            enable_trading_check_interval: Time interval (in seconds) to cache enable_trading checks.
                Default is 3600 (1 hour). Within this interval, enable_trading() will return
                immediately without checking blockchain state, improving performance significantly.
        """
        self.private_key = private_key
        self.signer = Signer(self.private_key)

        self.multi_sig_addr = multi_sig_addr
        self.conditional_tokens_addr = conditional_tokens_addr
        self.multisend_addr = multisend_addr
        self.fee_manager_addr = fee_manager_addr
        w3 = Web3(HTTPProvider(rpc_url))
        # Inject POA middleware to handle BNB Chain (BSC) which is a Proof of Authority chain
        # BNB Chain includes extra validator data in block headers (280 bytes instead of 32)
        w3.middleware_onion.inject(geth_poa_middleware, layer=0)
        self.w3 = w3
        self.safe = Safe(w3, private_key, multi_sig_addr, multisend_addr)
        self.__enable_trading_check_interval: int = enable_trading_check_interval
        self.__enable_trading_last_time: float = None
        # Cache for token decimals to avoid repeated contract calls
        self._token_decimals_cache: dict = {}

    @property
    def conditional_tokens(self) -> Contract:
        from .contracts.conditional_tokens import abi
        return self.w3.eth.contract(self.conditional_tokens_addr, abi=abi)

    @property
    def fee_manager(self) -> Contract:
        from .py_order_utils.abi.FeeManager import abi
        return self.w3.eth.contract(self.fee_manager_addr, abi=abi)

    def get_erc20_contract(self, address: ChecksumAddress):
        from .contracts.erc20 import abi
        return self.w3.eth.contract(address, abi=abi)

    def get_token_decimals(self, token_address: ChecksumAddress) -> int:
        """Get token decimals with caching to avoid repeated contract calls"""
        token_key = token_address.lower()

        if token_key not in self._token_decimals_cache:
            erc20_contract = self.get_erc20_contract(token_address)
            try:
                decimals = erc20_contract.functions.decimals().call()
                self._token_decimals_cache[token_key] = decimals
                logging.info(f'Token {token_address} uses {decimals} decimals')
            except Exception as e:
                logging.warning(f'Failed to get decimals for {token_address}, defaulting to 18: {e}')
                # Default to 18 if call fails (standard for most tokens)
                decimals = 18
                self._token_decimals_cache[token_key] = decimals

        return self._token_decimals_cache[token_key]

    def check_gas_balance(self, estimated_gas: int = 500000) -> None:
        """
        Check if signer has enough gas tokens (ETH) to execute transaction.

        Args:
            estimated_gas: Estimated gas units needed (default: 500000)

        Raises:
            InsufficientGasBalance: If signer doesn't have enough ETH for gas
        """
        signer_address = self.signer.address()
        gas_balance = self.w3.eth.get_balance(signer_address)

        # Get current gas price with safety margin
        base_fee = self.w3.eth.get_block('latest').get('baseFeePerGas', 0)

        # For EIP-1559 chains, calculate max fee
        if base_fee > 0:
            # Priority fee (tip) - typically 1-2 gwei on Base
            max_priority_fee = self.w3.to_wei(2, 'gwei')
            # Max fee = base fee * 2 + priority fee (allows for 2x base fee increase)
            max_fee_per_gas = (base_fee * 2) + max_priority_fee
            gas_price = max_fee_per_gas
        else:
            # Fallback for legacy transactions
            gas_price = self.w3.eth.gas_price

        # Add 20% safety margin to estimated gas
        estimated_gas_with_margin = int(estimated_gas * 1.2)

        # Calculate required ETH (gas * gas_price)
        required_eth = estimated_gas_with_margin * gas_price

        if gas_balance < required_eth:
            gas_balance_eth = self.w3.from_wei(gas_balance, 'ether')
            required_eth_formatted = self.w3.from_wei(required_eth, 'ether')
            gas_price_gwei = self.w3.from_wei(gas_price, 'gwei')
            raise InsufficientGasBalance(
                f"Insufficient gas balance. Signer {signer_address} has {gas_balance_eth} ETH, "
                f"but needs approximately {required_eth_formatted} ETH for gas "
                f"(gas: {estimated_gas_with_margin}, price: {gas_price_gwei} gwei)"
            )

        logging.info(
            f"Gas balance check passed. Signer has {self.w3.from_wei(gas_balance, 'ether')} ETH, "
            f"estimated cost: {self.w3.from_wei(required_eth, 'ether')} ETH "
            f"(gas: {estimated_gas_with_margin}, price: {self.w3.from_wei(gas_price, 'gwei')} gwei)"
        )

    def estimate_transaction_gas(self, tx_params: dict) -> int:
        """
        Estimate gas for a transaction using web3's gas estimation.

        Args:
            tx_params: Transaction parameters dict with 'from', 'to', 'data', etc.

        Returns:
            Estimated gas units needed
        """
        try:
            estimated = self.w3.eth.estimate_gas(tx_params)
            logging.debug(f"Estimated gas for transaction: {estimated}")
            return estimated
        except Exception as e:
            logging.warning(f"Gas estimation failed, using fallback: {e}")
            # Fallback to conservative estimate
            return 500000


    def split(self, collateral_token: ChecksumAddress, condition_id: Hash32,
              amount: int, partition: list = [1, 2], parent_collection_id: Hash32 = NULL_HASH) -> tuple[HexBytes, HexBytes, Any]:

        # Check gas balance before executing transaction
        self.check_gas_balance(estimated_gas=300000)

        # Check balance of collateral
        balance = self.get_erc20_contract(collateral_token).functions \
            .balanceOf(self.multi_sig_addr).call()
        logging.info(f'Collateral balance: {balance}')
        if balance < amount:
            raise BalanceNotEnough()

        multi_send_txs: List[MultiSendTx] = []

        data = HexBytes(
            self.conditional_tokens.functions.splitPosition(
                collateral_token, parent_collection_id, condition_id, partition, amount
            ).build_transaction(get_empty_tx_params())["data"]
        )

        multi_send_txs.append(MultiSendTx(
            operation=MultiSendOperation.CALL.value,
            to=self.conditional_tokens_addr,
            value=0,
            data=data,
        ))

        tx_hash, safe_tx_hash, return_value = self.safe.execute_multisend(multi_send_txs)

        # Validate transaction was successful
        receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)
        if receipt['status'] != 1:
            raise Exception(f"Split transaction failed. Transaction hash: {tx_hash.hex()}")

        logging.info(f"Split successful. Transaction hash: {tx_hash.hex()}")
        return tx_hash, safe_tx_hash, return_value

    def merge(self, collateral_token: ChecksumAddress, condition_id: Hash32,
              amount: int, partition: list = [1, 2], parent_collection_id: Hash32 = NULL_HASH) -> tuple[HexBytes, HexBytes, Any]:

        # Check gas balance before executing transaction
        self.check_gas_balance(estimated_gas=300000)

        # Check balance of positions
        for index_set in partition:
            position_id = self.get_position_id(condition_id, index_set=index_set, collateral_token=collateral_token)
            balance = self.conditional_tokens.functions \
                .balanceOf(self.multi_sig_addr, position_id).call()
            # print('balance: {}'.format(balance))
            if balance < amount:
                raise BalanceNotEnough()

        multi_send_txs: List[MultiSendTx] = []

        data = HexBytes(
            self.conditional_tokens.functions.mergePositions(
                collateral_token, parent_collection_id, condition_id, partition, amount
            ).build_transaction(get_empty_tx_params())["data"]
        )

        multi_send_txs.append(MultiSendTx(
            operation=MultiSendOperation.CALL.value,
            to=self.conditional_tokens_addr,
            value=0,
            data=data,
        ))

        tx_hash, safe_tx_hash, return_value = self.safe.execute_multisend(multi_send_txs)

        # Validate transaction was successful
        receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)
        if receipt['status'] != 1:
            raise Exception(f"Merge transaction failed. Transaction hash: {tx_hash.hex()}")

        logging.info(f"Merge successful. Transaction hash: {tx_hash.hex()}")
        return tx_hash, safe_tx_hash, return_value

    def redeem(self, collateral_token: ChecksumAddress, condition_id: Hash32,
              partition: list = [1, 2], parent_collection_id: Hash32 = NULL_HASH) -> tuple[HexBytes, HexBytes, Any]:

        # Check gas balance before executing transaction
        self.check_gas_balance(estimated_gas=300000)

        # Check balance of positions
        has_positions = False
        for index_set in partition:
            position_id = self.get_position_id(condition_id, index_set=index_set, collateral_token=collateral_token)
            balance = self.conditional_tokens.functions \
                .balanceOf(self.multi_sig_addr, position_id).call()
            # print('balance: {}'.format(balance))
            if balance > 0:
                has_positions = True
                break

        if not has_positions:
            raise NoPositionsToRedeem

        multi_send_txs: List[MultiSendTx] = []

        data = HexBytes(
            self.conditional_tokens.functions.redeemPositions(
                collateral_token, parent_collection_id, condition_id, partition
            ).build_transaction(get_empty_tx_params())["data"]
        )

        multi_send_txs.append(MultiSendTx(
            operation=MultiSendOperation.CALL.value,
            to=self.conditional_tokens_addr,
            value=0,
            data=data,
        ))

        tx_hash, safe_tx_hash, return_value = self.safe.execute_multisend(multi_send_txs)

        # Validate transaction was successful
        receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)
        if receipt['status'] != 1:
            raise Exception(f"Redeem transaction failed. Transaction hash: {tx_hash.hex()}")

        logging.info(f"Redeem successful. Transaction hash: {tx_hash.hex()}")
        return tx_hash, safe_tx_hash, return_value

    def enable_trading(self, supported_quote_tokens: dict) -> tuple[HexBytes, HexBytes, Any]:
        if self.__enable_trading_last_time is not None and \
                time.time() - self.__enable_trading_last_time < self.__enable_trading_check_interval:
            return HexBytes(b'0x'), HexBytes(b'0x'), None

        self.__enable_trading_last_time = time.time()

        # Check gas balance before executing transaction (approve operations can be gas-heavy)
        self.check_gas_balance(estimated_gas=500000)

        multi_send_txs: List[MultiSendTx] = []

        from .contracts.erc20 import abi
        for erc20_address, ctf_exchange_address in supported_quote_tokens.items():
            erc20_contract = self.w3.eth.contract(erc20_address, abi=abi)
            allowance = erc20_contract.functions.allowance(self.multi_sig_addr, ctf_exchange_address).call()

            # Get actual token decimals from contract
            decimals = self.get_token_decimals(erc20_address)

            # Used for trading on ctf_exchange
            min_threshold = 1000000000 * 10**decimals
            # Use unlimited approval (max uint256) - industry standard for trusted contracts
            # This eliminates need for re-approval and saves gas long-term
            allowance_to_update = 2**256 - 1
            if allowance < min_threshold:
                # DH1 Fix: Reset approval to 0 first (required for some tokens like USDT)
                # to prevent approval race condition attack
                if allowance > 0:
                    reset_data = HexBytes(
                        erc20_contract.functions.approve(
                            ctf_exchange_address, 0
                        ).build_transaction(get_empty_tx_params())["data"]
                    )
                    multi_send_txs.append(MultiSendTx(
                        operation=MultiSendOperation.CALL.value,
                        to=erc20_address,
                        value=0,
                        data=reset_data,
                    ))
                    logging.info(f'Resetting approval to 0 for {erc20_address} -> {ctf_exchange_address}')

                # Now set the new approval amount
                data = HexBytes(
                    erc20_contract.functions.approve(
                        ctf_exchange_address, allowance_to_update
                    ).build_transaction(get_empty_tx_params())["data"]
                )

                multi_send_txs.append(MultiSendTx(
                    operation=MultiSendOperation.CALL.value,
                    to=erc20_address,
                    value=0,
                    data=data,
                ))
                logging.info(f'Approving unlimited allowance for {erc20_address} -> {ctf_exchange_address}')

            # Used for splitting
            allowance = erc20_contract.functions.allowance(self.multi_sig_addr, self.conditional_tokens_addr).call()
            if allowance < min_threshold:
                # DH1 Fix: Reset approval to 0 first (required for some tokens like USDT)
                if allowance > 0:
                    reset_data = HexBytes(
                        erc20_contract.functions.approve(
                            self.conditional_tokens_addr, 0
                        ).build_transaction(get_empty_tx_params())["data"]
                    )
                    multi_send_txs.append(MultiSendTx(
                        operation=MultiSendOperation.CALL.value,
                        to=erc20_address,
                        value=0,
                        data=reset_data,
                    ))
                    logging.info(f'Resetting approval to 0 for {erc20_address} -> {self.conditional_tokens_addr}')

                # Now set the new approval amount
                data = HexBytes(
                    erc20_contract.functions.approve(
                        self.conditional_tokens_addr, allowance_to_update
                    ).build_transaction(get_empty_tx_params())["data"]
                )

                multi_send_txs.append(MultiSendTx(
                    operation=MultiSendOperation.CALL.value,
                    to=erc20_address,
                    value=0,
                    data=data,
                ))
                logging.info(f'Approving unlimited allowance for {erc20_address} -> {self.conditional_tokens_addr}')

            # Approve ctf_exchange for using conditional tokens
            is_approved_for_all = self.conditional_tokens.functions.isApprovedForAll(
                self.multi_sig_addr, ctf_exchange_address).call()
            if is_approved_for_all is False:
                data = HexBytes(
                    self.conditional_tokens.functions.setApprovalForAll(
                        ctf_exchange_address, True
                    ).build_transaction(get_empty_tx_params())["data"]
                )

                multi_send_txs.append(MultiSendTx(
                    operation=MultiSendOperation.CALL.value,
                    to=self.conditional_tokens_addr,
                    value=0,
                    data=data,
                ))

        if len(multi_send_txs) > 0:
            tx_hash, safe_tx_hash, return_value = self.safe.execute_multisend(multi_send_txs)

            # Validate transaction was successful
            receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)
            if receipt['status'] != 1:
                raise Exception(f"Enable trading transaction failed. Transaction hash: {tx_hash.hex()}")

            logging.info(f"Enable trading successful. Transaction hash: {tx_hash.hex()}")
            return tx_hash, safe_tx_hash, return_value
        else:
            return HexBytes(b'0x'), HexBytes(b'0x'), None

    def get_position_id(self, condition_id: Hash32, index_set: int, collateral_token: ChecksumAddress,
                        parent_condition_id=NULL_HASH):
        collection_id = self.conditional_tokens.functions.getCollectionId(
            parent_condition_id, condition_id, index_set).call()

        return self.conditional_tokens.functions.getPositionId(collateral_token, collection_id).call()

    def get_fee_rate_settings(self, token_id: int) -> dict:
        """
        Get fee rate settings from FeeManager contract.

        Args:
            token_id: The token ID to query fee rates for

        Returns:
            Dictionary containing:
                - maker_max_fee_rate: Maker maximum fee rate as decimal (e.g., 0.02 = 2%)
                - taker_max_fee_rate: Taker maximum fee rate as decimal (e.g., 0.02 = 2%)
                - enabled: Whether fee is enabled
        """
        try:
            result = self.fee_manager.functions.getFeeRateSettings(token_id).call()
            maker_fee_rate_bps = result[0]
            taker_fee_rate_bps = result[1]

            # Convert basis points to max fee rate percentage
            # Formula: fee_rate_bps * 0.25 / 10000
            # Example: 800 * 0.25 / 10000 = 0.02 (2%)
            maker_max_fee_rate = maker_fee_rate_bps * 0.25 / 10000
            taker_max_fee_rate = taker_fee_rate_bps * 0.25 / 10000

            return {
                "maker_max_fee_rate": maker_max_fee_rate,
                "taker_max_fee_rate": taker_max_fee_rate,
                "enabled": result[2]
            }
        except Exception as e:
            logging.error(f"Failed to get fee rate settings for token {token_id}: {e}")
            raise

from eth_account import Account
from web3 import Web3
from eth_typing import ChecksumAddress, HexStr
from typing import Any, Callable, Dict, List, Optional, Type, Union
import logging

from web3.middleware import SignAndSendRawMiddlewareBuilder

from .enums import SafeOperationEnum
from .constants import NULL_ADDRESS
from .safe_tx import SafeTx
from functools import cached_property
from web3.types import BlockIdentifier, TxParams, Wei

from .multisend import MultiSendTx, MultiSend
from .safe_contracts.utils import get_safe_contract

VERSION = 'v1.3.0'


class Safe:
    def __init__(
            self,
            w3: Web3,
            private_key: HexStr,
            address: ChecksumAddress,
            multisend_address: ChecksumAddress
    ):
        account = Account.from_key(private_key)
        w3.middleware_onion.inject(SignAndSendRawMiddlewareBuilder.build(account), layer=0)
        self.w3 = w3

        self.private_key = private_key
        self.address = address
        self.multisend_address = multisend_address
        self.multisend = MultiSend(self.w3, self.multisend_address, False)
        self.contract = get_safe_contract(self.w3, self.address)

    @cached_property
    def chain_id(self) -> int:
        return self.w3.eth.chain_id

    def get_version(self) -> str:
        """
        :return: String with Safe Master Copy semantic version, must match `retrieve_version()`
        """
        return VERSION

    def retrieve_nonce(
            self, block_identifier: Optional[BlockIdentifier] = "latest"
    ) -> int:
        return self.contract.functions.nonce().call(
            block_identifier=block_identifier or "latest"
        )

    def build_multisend_tx(self,
        multi_send_txs: List[MultiSendTx],
        safe_nonce: Optional[int] = None,
    ) ->  SafeTx:
        safe_gas = self.multisend.estimate_gas(multi_send_txs, self.address)
        data = self.multisend.build_tx_data(multi_send_txs)
        safe_tx = self.build_multisig_tx(
            to=self.multisend_address,
            value=0,
            data=data,
            operation=SafeOperationEnum.DELEGATE_CALL.value,
            safe_tx_gas=safe_gas,
            safe_nonce=safe_nonce,
        )

        return safe_tx

    def execute_multisend(self, multi_send_txs: List[MultiSendTx], safe_nonce: Optional[int] = None):
        safe_tx = self.build_multisend_tx(multi_send_txs, safe_nonce)
        safe_tx.sign(self.private_key)
        safe_tx_gas = safe_tx.recommended_gas()
        tx_params = {
            "from": Account.from_key(self.private_key).address,
            "gas": round(safe_tx_gas*1.2),
        }
        logging.debug(f'execute_multisend call with params: {tx_params}')

        return_value = safe_tx.w3_tx.call(tx_params)
        # return_value = None
        logging.debug(f'execute_multisend return value: {return_value} for safe_nonce {safe_tx.safe_nonce}, w3_tx: {safe_tx.w3_tx}')
        tx_hash = safe_tx.w3_tx.transact(tx_params)

        return tx_hash, safe_tx.safe_tx_hash, return_value

    def build_multisig_tx(
            self,
            to: ChecksumAddress,
            value: int,
            data: bytes,
            operation: int = SafeOperationEnum.CALL.value,
            safe_tx_gas: int = 0,
            base_gas: int = 0,
            gas_price: int = 0,
            gas_token: ChecksumAddress = NULL_ADDRESS,
            refund_receiver: ChecksumAddress = NULL_ADDRESS,
            signatures: bytes = b"",
            safe_nonce: Optional[int] = None,
    ) -> SafeTx:
        """
        Allows to execute a Safe transaction confirmed by required number of owners and then pays the account
        that submitted the transaction. The fees are always transfered, even if the user transaction fails

        :param to: Destination address of Safe transaction
        :param value: Ether value of Safe transaction
        :param data: Data payload of Safe transaction
        :param operation: Operation type of Safe transaction
        :param safe_tx_gas: Gas that should be used for the Safe transaction
        :param base_gas: Gas costs for that are independent of the transaction execution
            (e.g. base transaction fee, signature check, payment of the refund)
        :param gas_price: Gas price that should be used for the payment calculation
        :param gas_token: Token address (or `0x000..000` if ETH) that is used for the payment
        :param refund_receiver: Address of receiver of gas payment (or `0x000..000` if tx.origin).
        :param signatures: Packed signature data ({bytes32 r}{bytes32 s}{uint8 v})
        :param safe_nonce: Nonce of the safe (to calculate hash)
        :return: SafeTx
        """

        if safe_nonce is None:
            safe_nonce = self.retrieve_nonce()
        return SafeTx(
            self.w3,
            self.address,
            to,
            value,
            data,
            operation,
            safe_tx_gas,
            base_gas,
            gas_price,
            gas_token,
            refund_receiver,
            signatures=signatures,
            safe_nonce=safe_nonce,
            safe_version=self.get_version(),
            chain_id=self.chain_id,
        )

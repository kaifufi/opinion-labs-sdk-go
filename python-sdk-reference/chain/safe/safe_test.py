import unittest
from web3 import Web3
from web3.providers import HTTPProvider

from .constants import NULL_ADDRESS
from .enums import SafeOperationEnum
from .safe import Safe


class TestSafeRx(unittest.TestCase):
    def setUp(self) -> None:
        self.private_key = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80'
        self.safe_address = '0x8F58a1ab58e18Bb3f8ACf5E14c046D4F7add824a'
        self.compatibility_handler_address = '0xf48f2B2d2a534e402487b3ee7C18c33Aec0Fe5e4'
        self.rpc_url = 'https://bsc-dataseed.binance.org'
        self.w3 = Web3(HTTPProvider(self.rpc_url))

    def test_sign_multi_sig_tx(self):
        # self.w3.keccak(text='hash')

        safe = Safe(self.w3, self.private_key, self.safe_address, '')
        safe_tx = safe.build_multisig_tx(
            to='',
            value=0,
            data=b'',
            operation=SafeOperationEnum.CALL.value,
            safe_tx_gas=0,
            base_gas=0,
            gas_price=0,
            gas_token=NULL_ADDRESS,
            refund_receiver=NULL_ADDRESS,
            safe_nonce=0,
        )

        signatures = safe_tx.sign(self.private_key)
        expected_sig = '2e83d2137e103b2cd5bfbf624d796acd7aec0ca8ea007f3a8093e3844a0df12360493c35f4c49b42b55b17c2290b91ed9a9b787641f1433268552ebcb14edd2c1b'
        self.assertEqual(expected_sig, signatures.hex(), 'invalid signature')

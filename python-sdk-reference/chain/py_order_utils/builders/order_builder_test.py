import unittest
from .order_builder import OrderBuilder
from ..model.order import OrderData
from ..model.sides import BUY
from ..model import signatures
from ..constants import ZERO_ADDRESS
from ..signer import Signer
import json


class TestInterfaces(unittest.TestCase):
    def test_sign_order(self):
        exchange_address = "0xF0aebf65490374a477100351291c736c73c11D9F"
        chain_id = 56
        # Test private key, please do not use the key in production env.
        signer = Signer("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
        builder = OrderBuilder(exchange_address, chain_id, signer, lambda: 1)

        print('signer address: {}'.format(signer.address()))
        self.assertEqual(signer.address(), '0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266', "invalid signer address")
        # Create and sign the order
        order = builder.build_signed_order(
            OrderData(
                maker='0x8edbd5d17f368a50a7f8c0b1bbc0c9fcd0c2ccb3',
                taker=ZERO_ADDRESS,
                tokenId='102955147056674320605625831094933410586073394253729381009399467166952809400644',
                makerAmount='50',
                takerAmount='100',
                side=BUY,
                feeRateBps='0',
                signer=signer.address(),
                signatureType=signatures.POLY_GNOSIS_SAFE,
            )
        )

        # Generate the Order and Signature json to be sent to the CLOB API
        print('order: {}'.format(json.dumps(order.dict())))
        expected_signature = '0x4e2fbeb4959ddee243c682d2ebce61785cb03c1accb6b13a058df62d935ddf4941226aaa28dd1d71f150dc1708e9bfc22aab0bbb77690609e5692d2b7fd8ef3d1c'
        self.assertEqual(order.signature, expected_signature, 'unexpected signature')


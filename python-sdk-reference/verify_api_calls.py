"""
Verification script to check all API method signatures in opinion_clob_sdk
This script validates that all API calls pass the correct parameters to opinion_api
"""

import inspect
from opinion_api.api.prediction_market_api import PredictionMarketApi
from opinion_api.api.user_api import UserApi

# Expected API calls with required parameters
EXPECTED_API_CALLS = {
    # Market API calls
    'openapi_quote_token_get': ['apikey', 'chain_id'],  # chain_id should be str
    'openapi_market_get': ['apikey', 'chain_id'],  # chain_id should be str
    'openapi_market_market_id_get': ['apikey', 'market_id'],
    'openapi_market_categorical_market_id_get': ['apikey', 'market_id'],
    'openapi_token_price_history_get': ['apikey', 'token_id', 'interval', 'start_time', 'bars'],
    'openapi_token_orderbook_get': ['apikey', 'token_id'],
    'openapi_token_latest_price_get': ['apikey', 'token_id'],
    'openapi_token_fee_rates_get': ['apikey', 'token_id'],
    'openapi_order_post': ['apikey', 'v2_add_order_req'],
    'openapi_order_cancel_post': ['apikey', 'openapi_cancel_order_request_open_api'],
    'openapi_order_get': ['apikey', 'chain_id'],  # chain_id should be str
    'openapi_order_order_id_get': ['apikey', 'order_id'],
    'openapi_positions_get': ['apikey', 'chain_id'],  # chain_id should be str
    'openapi_user_balance_get': ['apikey', 'wallet_address', 'chain_id'],  # chain_id should be str
    'openapi_trade_get': ['apikey', 'chain_id'],  # chain_id should be str

    # User API calls
    'openapi_user_auth_get': ['apikey'],
}

def verify_api_signatures():
    """Verify that all API methods have the expected signatures"""
    errors = []

    # Check PredictionMarketApi methods
    for method_name, expected_params in EXPECTED_API_CALLS.items():
        if method_name == 'openapi_user_auth_get':
            continue  # This is in UserApi

        if not hasattr(PredictionMarketApi, method_name):
            errors.append(f"❌ Method {method_name} not found in PredictionMarketApi")
            continue

        method = getattr(PredictionMarketApi, method_name)
        sig = inspect.signature(method)
        params = list(sig.parameters.keys())

        # Remove 'self' from params
        if 'self' in params:
            params.remove('self')

        # Check if all expected params are in the signature
        for expected_param in expected_params:
            if expected_param not in params:
                errors.append(f"❌ {method_name}: missing parameter '{expected_param}'")

    # Check UserApi methods
    if hasattr(UserApi, 'openapi_user_auth_get'):
        method = getattr(UserApi, 'openapi_user_auth_get')
        sig = inspect.signature(method)
        params = list(sig.parameters.keys())

        if 'self' in params:
            params.remove('self')

        if 'apikey' not in params:
            errors.append("❌ openapi_user_auth_get: missing parameter 'apikey'")
    else:
        errors.append("❌ Method openapi_user_auth_get not found in UserApi")

    return errors

def check_sdk_calls():
    """Check that SDK methods call the API with correct parameters"""
    import re

    # Read sdk.py
    with open('/Users/nikli/Work/openapi/openapi/python_sdk/opinion_clob_sdk/sdk.py', 'r') as f:
        sdk_content = f.read()

    issues = []

    # Find all API calls in SDK
    api_call_pattern = r'self\.(market_api|user_api)\.(\w+)\((.*?)\)'
    matches = re.finditer(api_call_pattern, sdk_content, re.MULTILINE | re.DOTALL)

    for match in matches:
        api_type = match.group(1)
        method_name = match.group(2)
        params_str = match.group(3)

        # Check for apikey parameter
        if 'apikey=' not in params_str:
            issues.append(f"⚠️  {method_name}: missing apikey parameter")
        elif 'apikey=self.api_key' not in params_str:
            issues.append(f"⚠️  {method_name}: apikey should be self.api_key")

        # Check for chain_id parameter (should be str)
        if 'chain_id=' in params_str:
            if 'chain_id=str(self.chain_id)' not in params_str:
                issues.append(f"⚠️  {method_name}: chain_id should be str(self.chain_id)")

    return issues

if __name__ == '__main__':
    print("=" * 70)
    print("Verifying API Signatures in opinion_api")
    print("=" * 70)

    errors = verify_api_signatures()
    if errors:
        for error in errors:
            print(error)
        print(f"\n❌ Found {len(errors)} signature issues")
    else:
        print("✅ All API signatures verified successfully")

    print("\n" + "=" * 70)
    print("Checking SDK API Calls")
    print("=" * 70)

    issues = check_sdk_calls()
    if issues:
        for issue in issues:
            print(issue)
        print(f"\n⚠️  Found {len(issues)} potential issues in SDK calls")
    else:
        print("✅ All SDK API calls verified successfully")

    if not errors and not issues:
        print("\n" + "=" * 70)
        print("🎉 All verifications passed!")
        print("=" * 70)

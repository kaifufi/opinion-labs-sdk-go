import json
import os

# Load FeeManager ABI from JSON file
_current_dir = os.path.dirname(os.path.abspath(__file__))
_abi_path = os.path.join(_current_dir, 'FeeManager.json')

with open(_abi_path, 'r') as f:
    abi = json.load(f)

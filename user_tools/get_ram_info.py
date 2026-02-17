
import sys
import json
import os

# Rick Agent Argüman Okuyucu
args = {}
try:
    if len(sys.argv) > 1:
        # Gelen argüman bir JSON string mi diye bakıyoruz
        raw_arg = sys.argv[1]
        try:
            args = json.loads(raw_arg)
        except:
            # Değilse düz text olarak kabul et (Bazen lazım olabilir)
            args = {"raw": raw_arg}
except:
    pass

# --- RICK'IN KODU BAŞLANGIÇ ---
import psutil
ram = psutil.virtual_memory()
print(f"Total RAM: {ram.total / (1024**2):.2f} MB")
# --- RICK'IN KODU BİTİŞ ---

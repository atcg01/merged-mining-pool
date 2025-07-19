# Recalcul de la fonction getExpectedIndex après réinitialisation
def get_expected_index(nonce, chain_id, h):
    rand = nonce
    rand = rand * 1103515245 + 12345
    rand += chain_id
    rand = rand * 1103515245 + 12345
    return rand % (1 << h)

# Variables d'entrée
nonce = 0
height = 4
chain_ids = [0, 0x62, 16, 0x2023, 0x3f]

# Calcul des index pour chaque chainId
indexes = {chain_id: get_expected_index(nonce, chain_id, height) for chain_id in chain_ids}
print(indexes)

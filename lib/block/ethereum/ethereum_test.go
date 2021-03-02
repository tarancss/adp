package ethereum

import (
	"testing"
)

// block contains the sample data to decode.
var block = map[string]interface{}{"difficulty": "0x7ee56684", "extraData": "0x414952412f7630", "gasLimit": "0x47b784", "gasUsed": "0x47addd", "hash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "logsBloom": "0x0000000001400004002008000002000080000000000120200120002400208220000040000001000000000004804800000104000000000c0000000008201000005000200000010000140000084000000000000000100010400000080000040080100082000000000000000000004000021000800400802000000000501000000200000400000200020040010040000010105000000000040120000008000800200801000008004000000400004040000100000000000400000d005000020000008000004280010000000000000000000020010180100000140000000000020000000000000000008008000000000040000040100004001002c040000000000000", "miner": "0x00d8ae40d9a06d0e7a2877b62e32eb959afbe16d", "mixHash": "0xd93c06ec00e2c653b7958114ba8224aad8749caf8de6aee2c2f465c5f09cc0cc", "nonce": "0x34b98c94071402d8", "number": "0x29bf9b", "parentHash": "0x25e2e6cfc2f49ef320c652d91a7bea99a2d115d29ea832631e5f11911a463158", "receiptsRoot": "0x0506189cdc814f4440690b43aaf7cf278a9b346b8ef3174c03dde2d23aa820ea", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", "size": "0x299a", "stateRoot": "0xf8be81979f9a92cd123f8e6295dca2660184df4f58e275c6c9fe7adee0016e7c", "timestamp": "0x5a952da9", "totalDifficulty": "0x1bd6b7e3c7b473", "transactions": []interface{}{map[string]interface{}{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0xc4581843a8dacd100c7d435bb00b2a20d038e31d", "gas": "0x47b760", "gasPrice": "0x174876e800", "hash": "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65", "input": "0x4bdb8ab50804004410241002040000c60890801000000000000000000000000000000000", "nonce": "0x46", "r": "0xdd38a14e41b886d156a1073cc7ae914f4ee70d282925652b366bf953311d5862", "s": "0x4ecacbcef27ca7ebb7f8f628036a555f934a124063869fa8ba256ef7731218cf", "to": "0x7762440182222620a7435195208038708d27ee41", "transactionIndex": "0x0", "v": "0x1c", "value": "0x0"}, map[string]interface{}{"blockHash": "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6", "blockNumber": "0x29bf9b", "from": "0x1cd434711fbae1f2d9c70001409fd82d71fdccaa", "gas": "0xff59", "gasPrice": "0x98bca5a00", "hash": "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60", "input": "0x", "nonce": "0x0", "r": "0xb506e6cf81364d01c126028ec0acb771ca372269c8b157e551238a1e2d1b7ecb", "s": "0x2d7ea699220630938f57fe05fa581abd5a21f3aa105668a7128fba49598bbd70", "to": "0xa34de7bd2b4270c0b12d5fd7a0c219a4d68d732f", "transactionIndex": "0x1", "v": "0x29", "value": "0x16345785d8a0000"}}, "transactionsRoot": "0x08e95959ada5ebbe3aae1a4b9179f811c326c0969b7a5fea75b4e427c2870f96", "uncles": []string{}} //nolint:gochecknoglobals, lll // testdata

// TestEthereum tests the DecodeBlock and DecodeTxs functions only as the other are direct calls to the ethcli package.
func TestEthereum(t *testing.T) {
	var e *Ethereum = new(Ethereum)

	b, err := e.DecodeBlock(block)
	if err != nil || (b.Hash != "0xd44a255e40eee23bd90a54a792f7a35c175400958de22a9bbfe08a7b2c244ed6" ||
		b.Number != "0x29bf9b" ||
		b.PHash != "0x25e2e6cfc2f49ef320c652d91a7bea99a2d115d29ea832631e5f11911a463158" ||
		b.TS != "0x5a952da9") {
		t.Errorf("DecodeBlock error:%e Block:%+v", err, b)
	}

	txs, err := e.DecodeTxs(block) // (txs []types.Trans, err error)
	if err != nil || (len(txs) != 2 ||
		txs[0].Hash != "0xc39f3c2c2b5c0a772e8605bbeef7d341937b85e739a3c55d1e7384ac88f31c65" ||
		txs[1].Hash != "0xdbd3184b2f947dab243071000df22cf5acc6efdce90a04aaf057521b1ee5bf60") {
		t.Errorf("DecodeTxs error:%e txs:%+v", err, txs)
	}
}

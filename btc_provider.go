package easywallet

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"sort"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type BtcProvider struct {
	BaseProvider
	MempoolClient *MempoolClient
	LatestUtxos   []UTXO
	chain         *chaincfg.Params
}

func NewBtcProvider(key *hdkeychain.ExtendedKey, serviceUrl string, proxyUrl string, chain *chaincfg.Params) *BtcProvider {
	mempoolClient, _ := NewMempoolClient(serviceUrl, proxyUrl)
	return &BtcProvider{BaseProvider: NewBaseProvider(key, serviceUrl, proxyUrl), MempoolClient: mempoolClient, chain: chain}
}

func getBech32Address(pubKeyBytes []byte, netParams *chaincfg.Params) (btcutil.Address, error) {
	// Hash the public key
	witnessProgram := btcutil.Hash160(pubKeyBytes)

	addr, err := btcutil.NewAddressWitnessPubKeyHash(witnessProgram, netParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create bech32 address: %v", err)
	}
	return addr, nil
}

func (b *BtcProvider) GetAddress() (string, error) {
	pubKey, err := b.Key.ECPubKey()

	if err != nil {
		return "", err
	}

	// Get the public key bytes in compressed format
	pubKeyBytes := pubKey.SerializeCompressed()

	addrData, err := getBech32Address(pubKeyBytes, b.chain)

	if err != nil {
		return "", err
	}

	address := addrData.EncodeAddress()
	return address, nil
}

func (b *BtcProvider) GetBalance() (*big.Float, error) {
	address, err := b.GetAddress()
	if err != nil {
		return nil, err
	}

	utxos, err := b.MempoolClient.GetUTXOs(address)
	if err != nil {
		return nil, err
	}

	// Cache latest utxos
	b.LatestUtxos = utxos

	totalValue := lo.SumBy(utxos, func(u UTXO) int64 {
		return u.Value
	})
	resultAmount := new(big.Float).Quo(new(big.Float).SetInt64(totalValue), big.NewFloat(100000000.0))

	return resultAmount, nil
}

// Sizes for P2WPKH
const (
	P2WPKHInputBaseSize = 41  // outpoint(36) + scriptSig varint(1) + sequence(4)
	P2WPKHOutputSize    = 31  // 8(value) + 1(len) + 22(script)
	P2WPKHWitnessSize   = 108 // 1(count) + (1+72 sig) + (1+33 pubkey)
)

// EstimateVSize estimates vsize for a transaction with only P2WPKH ins/outs
func EstimateVSize(numInputs, numOutputs int) int {
	// --- Non-witness part ---
	nonWitness := 0
	nonWitness += 4                                           // version
	nonWitness += wire.VarIntSerializeSize(uint64(numInputs)) // input count
	nonWitness += numInputs * P2WPKHInputBaseSize
	nonWitness += wire.VarIntSerializeSize(uint64(numOutputs)) // output count
	nonWitness += numOutputs * P2WPKHOutputSize
	nonWitness += 4 // locktime

	// --- Witness part ---
	witness := 0
	if numInputs > 0 {
		witness += 2 // marker + flag
	}
	witness += numInputs * P2WPKHWitnessSize

	// --- Weight and vsize ---
	weight := 4*nonWitness + witness
	return int(math.Ceil(float64(weight) / 4.0))
}

const SatoshisPerBitcoin = 100000000

const SatsPerByte = 2

func SendRawTransaction(client *MempoolClient, rawHex string) (string, error) {
	// Create the request
	req, err := http.NewRequest("POST", client.BaseURL+"/tx", bytes.NewBufferString(rawHex))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Send the request
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	return string(body), nil
}

func (b *BtcProvider) Send(recipientAddress string, amount decimal.Decimal) (string, error) {

	if len(b.LatestUtxos) == 0 {
		_, err := b.GetBalance()

		if err != nil {
			return "", err
		}
	}

	amountSatoshis := amount.Shift(8).IntPart()

	sort.Slice(b.LatestUtxos, func(i, j int) bool {
		return b.LatestUtxos[i].Value > b.LatestUtxos[j].Value
	})

	var utxosForTx []UTXO

	utxosSum := int64(0)

	for _, value := range b.LatestUtxos {
		utxosSum += value.Value
		utxosForTx = append(utxosForTx, value)

		if utxosSum >= amountSatoshis {
			break
		}
	}

	changeAddress, err := b.GetAddress()

	if err != nil {
		return "", err
	}

	txSize := EstimateVSize(len(utxosForTx), 2)
	feePerTx := SatsPerByte * txSize

	destinations := map[string]int64{
		recipientAddress: amountSatoshis,
		changeAddress:    utxosSum - amountSatoshis - int64(feePerTx),
	}

	if amountSatoshis >= (utxosSum - int64(feePerTx)) {
		return "", errors.New("amount is too small")
	}

	privKey, err := b.Key.ECPrivKey()

	if err != nil {
		return "", err
	}

	tx := wire.NewMsgTx(wire.TxVersion)

	for _, utxo := range utxosForTx {
		prevHash, err := chainhash.NewHashFromStr(utxo.TxID)
		if err != nil {
			log.Fatal(err)
		}
		outPoint := wire.NewOutPoint(prevHash, utxo.Vout)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		tx.AddTxIn(txIn)
	}

	for addrStr, amt := range destinations {
		addr, err := btcutil.DecodeAddress(addrStr, b.chain)
		if err != nil {
			log.Fatal(err)
		}
		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			log.Fatal(err)
		}
		txOut := wire.NewTxOut(int64(amt), pkScript)
		tx.AddTxOut(txOut)
	}

	for i, utxo := range utxosForTx {

		pkh := btcutil.Hash160(privKey.PubKey().SerializeCompressed())
		witnessProg, err := txscript.NewScriptBuilder().AddOp(txscript.OP_0).AddData(pkh).Script()
		if err != nil {
			log.Fatal(err)
		}

		// Custom fetcher to pass taproot check, as we don't use taproot
		fetcher := txscript.NewCannedPrevOutputFetcher([]byte{}, int64(utxo.Value))

		// Create witness
		witness, err := txscript.WitnessSignature(
			tx,
			txscript.NewTxSigHashes(tx, fetcher),
			i,
			utxo.Value,
			witnessProg,
			txscript.SigHashAll,
			privKey,
			true,
		)
		if err != nil {
			log.Fatal(err)
		}
		tx.TxIn[i].Witness = witness
	}

	var buffer bytes.Buffer
	err = tx.Serialize(&buffer)
	if err != nil {
		log.Fatal(err)
		return "", err
	}

	txHexRes := hex.EncodeToString(buffer.Bytes())

	transaction, err := SendRawTransaction(b.MempoolClient, txHexRes)
	if err != nil {
		return "", err
	}

	return transaction, nil
}

package easywallet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/sha3"
)

const (
	// Tron address prefix (0x41)
	TronAddressPrefix = 0x41
	// TRX has 6 decimals (SUN)
	TRXDecimals = 6
)

// Compile-time interface check
var _ Provider = (*TrxProvider)(nil)

type TrxProvider struct {
	BaseProvider
	httpClient *http.Client
}

func NewTrxProvider(key *hdkeychain.ExtendedKey, serviceUrl string, proxyUrl string) *TrxProvider {
	httpClient, _ := GetClientWithProxy(proxyUrl)
	return &TrxProvider{
		BaseProvider: NewBaseProvider(key, serviceUrl, proxyUrl),
		httpClient:   httpClient,
	}
}

// GetAddress returns the Tron address derived from the private key
func (t *TrxProvider) GetAddress() (string, error) {
	privKey, err := t.Key.ECPrivKey()
	if err != nil {
		return "", err
	}

	pubKeyBytes := privKey.PubKey().SerializeUncompressed()

	// Take Keccak-256 hash of the public key
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubKeyBytes[1:])
	digest := hash.Sum(nil)

	// TRON адрес в Hex: префикс 0x41 + последние 20 байт хеша
	addressBytes := append([]byte{0x41}, digest[len(digest)-20:]...)

	// Encode to Base58Check using gotron-sdk
	address := common.EncodeCheck(addressBytes)
	return address, nil
}

// TronBalanceResponse represents the response from Tron API for balance query
type TronBalanceResponse struct {
	Balance int64 `json:"balance"`
}

// TronTransactionResponse represents the response from Tron API for broadcast
type TronTransactionResponse struct {
	Result bool   `json:"result"`
	Txid   string `json:"txid"`
	Code   string `json:"code,omitempty"`
}

// GetBalance returns the TRX balance
func (t *TrxProvider) GetBalance() (*big.Float, error) {
	address, err := t.GetAddress()
	if err != nil {
		return nil, err
	}

	// Tron API call to get balance
	reqBody := map[string]string{
		"address": address,
		"visible": "true",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", t.serviceUrl+"/wallet/getaccount", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var accountResp struct {
		Balance int64 `json:"balance"`
	}
	if err := json.Unmarshal(body, &accountResp); err != nil {
		return nil, err
	}

	// Convert SUN to TRX (6 decimals)
	balanceFloat := new(big.Float).Quo(
		new(big.Float).SetInt64(accountResp.Balance),
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(TRXDecimals), nil)),
	)

	return balanceFloat, nil
}

// Send sends TRX to the recipient address
func (t *TrxProvider) Send(recipientAddress string, amount decimal.Decimal) (string, error) {
	fromAddress, err := t.GetAddress()
	if err != nil {
		return "", err
	}

	// Convert amount to SUN (TRX has 6 decimals)
	amountSun := amount.Shift(TRXDecimals).IntPart()

	// Build the transaction
	txData := map[string]interface{}{
		"owner_address": fromAddress,
		"to_address":    recipientAddress,
		"amount":        amountSun,
		"visible":       true,
		"fee_limit":     1000000, // 1 TRX max fee
	}

	jsonBody, _ := json.Marshal(txData)

	// Create unsigned transaction
	req, err := http.NewRequest("POST", t.serviceUrl+"/wallet/createtransaction", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var unsignedTx map[string]interface{}
	if err := json.Unmarshal(body, &unsignedTx); err != nil {
		return "", err
	}

	// Check for error in response
	if errMsg, ok := unsignedTx["Error"]; ok {
		return "", fmt.Errorf("failed to create transaction: %v", errMsg)
	}

	// Get the raw data hex for signing
	rawDataHex, ok := unsignedTx["raw_data_hex"].(string)
	if !ok {
		return "", fmt.Errorf("missing raw_data_hex in transaction")
	}

	rawDataBytes, err := hex.DecodeString(rawDataHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode raw data: %v", err)
	}

	// Sign the transaction
	signature, err := t.signTransaction(rawDataBytes)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Add signature to transaction
	unsignedTx["signature"] = []string{hex.EncodeToString(signature)}

	// Broadcast the signed transaction
	signedTxBody, _ := json.Marshal(unsignedTx)
	broadcastReq, err := http.NewRequest("POST", t.serviceUrl+"/wallet/broadcasttransaction", bytes.NewBuffer(signedTxBody))
	if err != nil {
		return "", err
	}
	broadcastReq.Header.Set("Content-Type", "application/json")

	broadcastResp, err := t.httpClient.Do(broadcastReq)
	if err != nil {
		return "", err
	}
	defer broadcastResp.Body.Close()

	broadcastBody, err := io.ReadAll(broadcastResp.Body)
	if err != nil {
		return "", err
	}

	var broadcastResult TronTransactionResponse
	if err := json.Unmarshal(broadcastBody, &broadcastResult); err != nil {
		return "", err
	}

	if !broadcastResult.Result {
		return "", fmt.Errorf("transaction broadcast failed: %s", broadcastResult.Code)
	}

	// Return the transaction ID
	txid, ok := unsignedTx["txID"].(string)
	if !ok {
		return "", fmt.Errorf("missing txID in transaction")
	}

	return txid, nil
}

// signTransaction signs the transaction data using the private key
func (t *TrxProvider) signTransaction(data []byte) ([]byte, error) {
	privKey, err := t.Key.ECPrivKey()
	if err != nil {
		return nil, err
	}

	// Tron uses double SHA-256 for signing
	hash := sha256.Sum256(data)
	hash = sha256.Sum256(hash[:])

	// Sign the hash using btcec/v2/ecdsa
	signature := ecdsa.Sign(privKey, hash[:])

	return signature.Serialize(), nil
}

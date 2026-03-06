package easywallet

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/shopspring/decimal"
)

// Compile-time interface check
var _ Provider = (*TrxTokenProvider)(nil)

// TrxTokenProvider handles TRC-20 token operations on the Tron network
type TrxTokenProvider struct {
	*TrxProvider
	TokenAddress string
}

// NewTrxTokenProvider creates a new TRC-20 token provider
func NewTrxTokenProvider(key *hdkeychain.ExtendedKey, tokenAddress string, serviceUrl string, proxyUrl string) *TrxTokenProvider {
	return &TrxTokenProvider{
		TrxProvider:  NewTrxProvider(key, serviceUrl, proxyUrl),
		TokenAddress: tokenAddress,
	}
}

// TronTriggerContractResponse represents the response from triggersmartcontract API
type TronTriggerContractResponse struct {
	Result struct {
		Result bool `json:"result"`
	} `json:"result"`
	ConstantResult []string `json:"constant_result"`
	Message        string   `json:"message,omitempty"`
}

// TronContractBalanceResponse represents the balance response from contract call
type TronContractBalanceResponse struct {
	Balance string `json:"constant_result"`
}

// getTokenDecimals fetches the decimals of the TRC-20 token
func (t *TrxTokenProvider) getTokenDecimals() (uint8, error) {
	// decimals() function selector: 0x313ce567
	decimalsSelector := "313ce567"

	// Pad token address to 32 bytes (remove 0x41 prefix first, then pad)
	tokenAddrHex, err := common.DecodeCheck(t.TokenAddress)
	if err != nil {
		return 0, fmt.Errorf("failed to decode token address: %v", err)
	}
	if len(tokenAddrHex) != 21 {
		return 0, fmt.Errorf("invalid token address length")
	}

	// Build parameter: pad the address part (last 20 bytes) to 32 bytes
	param := make([]byte, 32)
	copy(param[12:], tokenAddrHex[1:]) // Skip the 0x41 prefix

	dataHex := decimalsSelector + hex.EncodeToString(param)

	reqBody := map[string]interface{}{
		"owner_address":     "410000000000000000000000000000000000000000",
		"contract_address":  t.TokenAddress,
		"function_selector": "decimals()",
		"parameter":         "",
		"data":              dataHex,
		"visible":           true,
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", t.serviceUrl+"/wallet/triggersmartcontract", bytes.NewBuffer(jsonBody))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result TronTriggerContractResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	if len(result.ConstantResult) == 0 {
		return 0, fmt.Errorf("no decimals result returned from contract")
	}

	// Parse the hex result to get decimals
	decimalsHex := result.ConstantResult[0]
	decimalsInt, err := strconv.ParseInt(decimalsHex, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse decimals: %v", err)
	}

	return uint8(decimalsInt), nil
}

// GetBalance returns the TRC-20 token balance
func (t *TrxTokenProvider) GetBalance() (*big.Float, error) {
	address, err := t.GetAddress()
	if err != nil {
		return nil, err
	}

	// balanceOf(address) function selector: 0x70a08231
	balanceOfSelector := "70a08231"

	// Convert address to proper format
	addressBytes, err := common.DecodeCheck(address)
	if err != nil {
		return nil, fmt.Errorf("failed to decode address: %v", err)
	}
	if len(addressBytes) != 21 {
		return nil, fmt.Errorf("invalid address length")
	}

	// Build parameter: pad the address part (last 20 bytes) to 32 bytes
	param := make([]byte, 32)
	copy(param[12:], addressBytes[1:]) // Skip the 0x41 prefix

	dataHex := balanceOfSelector + hex.EncodeToString(param)

	reqBody := map[string]interface{}{
		"owner_address":     address,
		"contract_address":  t.TokenAddress,
		"function_selector": "balanceOf(address)",
		"parameter":         hex.EncodeToString(param),
		"data":              dataHex,
		"visible":           true,
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", t.serviceUrl+"/wallet/triggersmartcontract", bytes.NewBuffer(jsonBody))
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

	var result TronTriggerContractResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.ConstantResult) == 0 {
		return big.NewFloat(0), nil
	}

	// Parse the balance from hex
	balanceHex := result.ConstantResult[0]
	balanceInt := new(big.Int)
	balanceInt, ok := balanceInt.SetString(balanceHex, 16)
	if !ok {
		return nil, fmt.Errorf("failed to parse balance hex: %s", balanceHex)
	}

	// Get token decimals
	decimals, err := t.getTokenDecimals()
	if err != nil {
		// Default to 18 decimals if we can't fetch
		decimals = 18
	}

	// Convert to human readable format
	humanBalance := new(big.Float).Quo(
		new(big.Float).SetInt(balanceInt),
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)),
	)

	return humanBalance, nil
}

// Send sends TRC-20 tokens to the recipient address
func (t *TrxTokenProvider) Send(recipientAddress string, amount decimal.Decimal) (string, error) {
	fromAddress, err := t.GetAddress()
	if err != nil {
		return "", err
	}

	// Get token decimals
	decimals, err := t.getTokenDecimals()
	if err != nil {
		return "", fmt.Errorf("failed to get token decimals: %v", err)
	}

	// Convert amount to token units
	amountTokenUnits := amount.Shift(int32(decimals)).BigInt()

	// transfer(address,uint256) function selector: 0xa9059cbb
	transferSelector := "a9059cbb"

	// Convert recipient address to proper format
	recipientBytes, err := common.DecodeCheck(recipientAddress)
	if err != nil {
		return "", fmt.Errorf("failed to decode recipient address: %v", err)
	}
	if len(recipientBytes) != 21 {
		return "", fmt.Errorf("invalid recipient address length")
	}

	// Build parameter: address (32 bytes) + amount (32 bytes)
	param := make([]byte, 64)
	copy(param[12:], recipientBytes[1:]) // Skip the 0x41 prefix, place at bytes 12-31
	amountBytes := amountTokenUnits.Bytes()
	copy(param[64-len(amountBytes):], amountBytes) // Place amount at the end

	dataHex := transferSelector + hex.EncodeToString(param)

	// Build the transaction for trigger smart contract
	txData := map[string]interface{}{
		"owner_address":     fromAddress,
		"contract_address":  t.TokenAddress,
		"function_selector": "transfer(address,uint256)",
		"parameter":         hex.EncodeToString(param),
		"data":              dataHex,
		"fee_limit":         100000000, // 100 TRX max fee for token transfers
		"call_value":        0,
		"visible":           true,
	}

	jsonBody, _ := json.Marshal(txData)

	// Create unsigned transaction
	req, err := http.NewRequest("POST", t.serviceUrl+"/wallet/triggersmartcontract", bytes.NewBuffer(jsonBody))
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

	var triggerResult struct {
		Result struct {
			Result bool   `json:"result"`
			Code   string `json:"code"`
		} `json:"result"`
		Transaction map[string]interface{} `json:"transaction"`
		Message     string                 `json:"message,omitempty"`
	}
	if err := json.Unmarshal(body, &triggerResult); err != nil {
		return "", err
	}

	// Check for error in response
	if !triggerResult.Result.Result {
		return "", fmt.Errorf("failed to create token transfer transaction: %s", triggerResult.Message)
	}

	unsignedTx := triggerResult.Transaction
	if unsignedTx == nil {
		return "", fmt.Errorf("missing transaction in trigger response")
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

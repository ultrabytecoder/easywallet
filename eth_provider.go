package easywallet

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
)

type EthereumProvider struct {
	BaseProvider
}

// ERC-20 ABI
const erc20ABI = `[
    {
        "constant": false,
        "inputs": [
            {"name": "_to", "type": "address"},
            {"name": "_value", "type": "uint256"}
        ],
        "name": "transfer",
        "outputs": [{"name": "", "type": "bool"}],
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [{"name": "_owner", "type": "address"}],
        "name": "balanceOf",
        "outputs": [{"name": "balance", "type": "uint256"}],
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [],
        "name": "decimals",
        "outputs": [{"name": "", "type": "uint8"}],
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [],
        "name": "symbol",
        "outputs": [{"name": "", "type": "string"}],
        "type": "function"
    }
]`

func NewEthereumProvider(key *hdkeychain.ExtendedKey, serviceUrl string) *EthereumProvider {
	return &EthereumProvider{BaseProvider: NewBaseProvider(key, serviceUrl)}
}

func (e *EthereumProvider) GetAddress() (string, error) {
	privKey, err := e.Key.ECPrivKey()

	if err != nil {
		return "", err
	}

	aprivKey, err1 := gcrypto.ToECDSA(privKey.Serialize())

	if err1 != nil {
		return "", err1
	}

	addr := gcrypto.PubkeyToAddress(aprivKey.PublicKey)

	return addr.String(), nil
}

func (e *EthereumProvider) CreateEthereumClient() (*ethclient.Client, error) {
	client, err := ethclient.Dial(e.serviceUrl)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (e *EthereumProvider) GetBalance() (*big.Float, error) {
	address, err := e.GetAddress()

	if err != nil {
		return nil, err
	}

	client, err := e.CreateEthereumClient()
	if err != nil {
		println("Not connected to rpc.sepolia.org")
		return nil, err
	}

	balance, err := client.BalanceAt(context.Background(), common.HexToAddress(address), nil)

	if err != nil {
		println("Cannot get balance")
		return nil, err
	}

	resultAmount := new(big.Float).Quo(new(big.Float).SetInt64(balance.Int64()), big.NewFloat(1000000000000000000.0))

	return resultAmount, nil
}

func (e *EthereumProvider) Send(recipientAddress string, amount decimal.Decimal) (string, error) {
	address, err := e.GetAddress()

	if err != nil {
		return "", err
	}

	fa := common.HexToAddress(address)

	ra := common.HexToAddress(recipientAddress)

	amountWei := amount.Shift(18).BigInt()

	client, err := e.CreateEthereumClient()

	if err != nil {
		println("Not connected to rpc.sepolia.org")
		return "", err
	}

	// Get nonce
	nonce, err := client.PendingNonceAt(context.Background(), fa)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %v", err)
	}

	// Get chain ID
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %v", err)
	}

	// EIP-1559 Dynamic Fee Transaction
	gasTipCap, err := client.SuggestGasTipCap(context.Background())
	if err != nil {
		return "", err
	}

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return "", err
	}
	gasFeeCap := new(big.Int).Add(header.BaseFee, gasTipCap)

	txData := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       21000,
		To:        &ra,
		Value:     amountWei,
		Data:      nil,
	}

	tx := types.NewTx(txData)

	privKey, err := e.Key.ECPrivKey()

	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %v", err)
	}

	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), privKey.ToECDSA())
	if err != nil {
		return "", err
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signedTx.Hash().Hex(), nil
}

type EthTokenProvider struct {
	*EthereumProvider
	TokenAddress string
}

func getTokenDecimals(client *ethclient.Client, tokenAddress common.Address, contractABI abi.ABI) (uint8, error) {
	data, err := contractABI.Pack("decimals")
	if err != nil {
		return 0, err
	}

	msg := ethereum.CallMsg{
		To:   &tokenAddress,
		Data: data,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return 0, err
	}

	var decimals uint8
	err = contractABI.UnpackIntoInterface(&decimals, "decimals", result)
	return decimals, err
}

func (e *EthTokenProvider) GetBalance() (*big.Float, error) {
	address, err := e.GetAddress()

	if err != nil {
		return nil, err
	}

	client, err := e.CreateEthereumClient()
	if err != nil {
		println("Not connected to rpc.sepolia.org")
		return nil, err
	}

	abi, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, err
	}

	// Pack the balanceOf function call
	data, err := abi.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return nil, err
	}

	ta := common.HexToAddress(e.TokenAddress)
	msg := ethereum.CallMsg{
		To:   &ta,
		Data: data,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, err
	}

	// Unpack the result
	var balance *big.Int
	err = abi.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, err
	}

	decimals, err := getTokenDecimals(client, ta, abi)

	if err != nil {
		return nil, err
	}
	// Convert to human readable format
	humanBalance := new(big.Float).Quo(
		new(big.Float).SetInt(balance),
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)),
	)

	return humanBalance, nil
}

// convertToTokenUnits converts human-readable amount to token units
func convertToTokenUnits(amount float64, decimals uint8) *big.Int {
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	amountFloat := new(big.Float).SetFloat64(amount)
	multiplierFloat := new(big.Float).SetInt(multiplier)

	result := new(big.Float).Mul(amountFloat, multiplierFloat)
	resultInt := new(big.Int)
	result.Int(resultInt)

	return resultInt
}

func (e *EthTokenProvider) Send(recipientAddress string, amount decimal.Decimal) (string, error) {
	fromAddress, err := e.GetAddress()
	if err != nil {
		return "", err
	}

	client, err := e.CreateEthereumClient()
	if err != nil {
		println("Not connected to rpc.sepolia.org")
		return "", err
	}

	fa := common.HexToAddress(fromAddress)
	ta := common.HexToAddress(e.TokenAddress)
	ra := common.HexToAddress(recipientAddress)

	abi, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return "", err
	}

	// Get token decimals
	decimals, err := getTokenDecimals(client, ta, abi)
	if err != nil {
		return "", fmt.Errorf("failed to get token decimals: %v", err)
	}

	// Convert amount to token units
	amountTokenUnits := amount.Shift(int32(decimals)).BigInt()

	// Get transaction parameters
	nonce, err := client.PendingNonceAt(context.Background(), fa)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %v", err)
	}

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %v", err)
	}

	// Encode transfer function call
	data, err := abi.Pack("transfer", ra, amountTokenUnits)
	if err != nil {
		return "", fmt.Errorf("failed to pack transfer data: %v", err)
	}

	// Get gas parameters for EIP-1559
	gasTipCap, err := client.SuggestGasTipCap(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get gas tip cap: %v", err)
	}

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get header: %v", err)
	}
	gasFeeCap := new(big.Int).Add(header.BaseFee, gasTipCap)

	// Add buffer to gas fee cap
	gasFeeCap = new(big.Int).Mul(gasFeeCap, big.NewInt(125))
	gasFeeCap = new(big.Int).Div(gasFeeCap, big.NewInt(100))

	// Estimate gas for token transfer
	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		From:      fa,
		To:        &ta,
		Data:      data,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	})
	if err != nil {
		// Fallback gas limit for ERC-20 transfers
		gasLimit = 65000
	}

	// Add 20% buffer to gas limit
	gasLimit = gasLimit * 120 / 100

	// Create DynamicFee transaction for token transfer
	txData := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        &ta,
		Value:     big.NewInt(0), // No ETH value for token transfers
		Data:      data,
	}

	tx := types.NewTx(txData)

	privKey, err := e.Key.ECPrivKey()

	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %v", err)
	}

	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), privKey.ToECDSA())
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Send the transaction
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signedTx.Hash().Hex(), nil
}

func NewEthTokenProvider(key *hdkeychain.ExtendedKey, tokenAddress string, serviceUrl string) *EthTokenProvider {
	return &EthTokenProvider{EthereumProvider: NewEthereumProvider(key, serviceUrl), TokenAddress: tokenAddress}
}

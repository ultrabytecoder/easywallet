package easywallet

import (
	"math/big"

	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/shopspring/decimal"
	bip39 "github.com/tyler-smith/go-bip39"
)

type MultiWallet struct {
	Providers map[string]Provider
	MasterKey *hdkeychain.ExtendedKey
}

const (
	ProviderTypeBtc      string = "BtcProvider"
	ProviderTypeEth      string = "EthProvider"
	ProviderTypeEthToken string = "EthTokenProvider"
)

func NewMultiWallet(config *Config, mnemonic string) (*MultiWallet, error) {

	if mnemonic == "" {
		return nil, errors.New("empty mnemonic")
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("invalid mnemonic")
	}

	var chain chaincfg.Params
	switch config.Network {
	case "mainnet":
		chain = chaincfg.MainNetParams
	case "testnet":
		chain = chaincfg.SigNetParams
	default:
		return nil, fmt.Errorf("unknown network: %s", config.Network)
	}

	seed := bip39.NewSeed(mnemonic, "")

	masterKey, err := hdkeychain.NewMaster(seed, &chain)
	if err != nil {
		return nil, err
	}

	providers := make(map[string]Provider)

	for _, providerInfo := range config.Providers {

		key, err := deriveFromStringPath(masterKey, providerInfo.DerivationPath)
		if err != nil {
			return nil, err
		}

		var provider Provider
		switch providerInfo.ProviderType {
		case ProviderTypeBtc:
			provider = NewBtcProvider(key, providerInfo.ServiceUrl, &chain)
		case ProviderTypeEth:
			provider = NewEthereumProvider(key, providerInfo.ServiceUrl)
		case ProviderTypeEthToken:
			provider = NewEthTokenProvider(key, providerInfo.TokenAddress, providerInfo.ServiceUrl)
		}

		providers[providerInfo.Currency] = provider

	}

	return &MultiWallet{Providers: providers, MasterKey: masterKey}, nil
}

func (e *MultiWallet) GetAddress(cointype string) (string, error) {
	provider, ok := e.Providers[cointype]
	if !ok {
		return "", errors.New("cointype not found")
	}

	return provider.GetAddress()
}

func (e *MultiWallet) GetBalance(cointype string) (*big.Float, error) {

	provider, ok := e.Providers[cointype]
	if !ok {
		return nil, errors.New("cointype not found")
	}

	return provider.GetBalance()
}

func (e *MultiWallet) Send(cointype string, address string, amount decimal.Decimal) (string, error) {
	provider, ok := e.Providers[cointype]
	if !ok {
		return "", errors.New("cointype not found")
	}

	return provider.Send(address, amount)
}
func deriveFromStringPath(key *hdkeychain.ExtendedKey, path string) (*hdkeychain.ExtendedKey, error) {
	current := key

	// Split the path and process each component
	components := strings.Split(path, "/")
	for _, comp := range components {
		if comp == "m" {
			continue // Skip master indicator
		}

		hardened := false
		if strings.HasSuffix(comp, "'") || strings.HasSuffix(comp, "h") {
			hardened = true
			comp = strings.TrimRight(comp, "'h")
		}

		index, err := strconv.ParseUint(comp, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid path component: %s", comp)
		}

		if hardened {
			current, err = current.Derive(hdkeychain.HardenedKeyStart + uint32(index))
		} else {
			current, err = current.Derive(uint32(index))
		}

		if err != nil {
			return nil, err
		}
	}

	return current, nil
}

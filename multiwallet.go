package easywallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"io"
	"math/big"
	"os"

	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/shopspring/decimal"
	"github.com/tyler-smith/go-bip39"
)

type MultiWallet struct {
	Providers   map[string]Provider
	MasterKey   *hdkeychain.ExtendedKey
	SeedStorage *MasterSeedStorage
}

const (
	ProviderTypeBtc      string = "BtcProvider"
	ProviderTypeEth      string = "EthProvider"
	ProviderTypeEthToken string = "EthTokenProvider"
)

type MasterSeedStorage struct {
	MasterSeed  []byte
	IsEncrypted bool
}

func NewMultiWallet(config *Config, masterSeedEncryptionPassword string) (*MultiWallet, error) {
	// Read seed data from binary file
	seedStorage, err := ReadSeedStorage()
	if err != nil {
		return nil, err
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

	var seed []byte

	if seedStorage.IsEncrypted {
		seed, err = decryptSeed(seedStorage.MasterSeed, masterSeedEncryptionPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt seed: %w", err)
		}
	} else {
		seed = seedStorage.MasterSeed
	}

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
			provider = NewBtcProvider(key, providerInfo.ServiceUrl, config.ProxyUrl, &chain)
		case ProviderTypeEth:
			provider = NewEthereumProvider(key, providerInfo.ServiceUrl, config.ProxyUrl)
		case ProviderTypeEthToken:
			provider = NewEthTokenProvider(key, providerInfo.TokenAddress, providerInfo.ServiceUrl, config.ProxyUrl)
		}

		providers[providerInfo.Currency] = provider

	}

	return &MultiWallet{Providers: providers, MasterKey: masterKey, SeedStorage: seedStorage}, nil
}

func ReadSeedStorage() (*MasterSeedStorage, error) {
	file, err := os.Open("seed.dat")
	if err != nil {
		return nil, fmt.Errorf("failed to open seed.dat: %w", err)
	}
	defer file.Close()

	var seedStorage MasterSeedStorage
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&seedStorage); err != nil {
		return nil, fmt.Errorf("failed to decode seed storage: %w", err)
	}
	return &seedStorage, nil
}
func GenerateAndSaveMasterSeed(mnemonic string, seedEncryptionPassword string) error {
	if mnemonic == "" {
		return errors.New("empty mnemonic")
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		return errors.New("invalid mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, "")

	var masterSeed []byte
	isEncrypted := len(seedEncryptionPassword) > 0

	if isEncrypted {
		encryptedSeed, err := encryptSeed(seed, seedEncryptionPassword)
		if err != nil {
			return fmt.Errorf("failed to encrypt seed: %w", err)
		}
		masterSeed = encryptedSeed
	} else {
		masterSeed = seed
	}

	storage := MasterSeedStorage{MasterSeed: masterSeed, IsEncrypted: isEncrypted}

	file, err := os.Create("seed.dat")
	if err != nil {
		return fmt.Errorf("failed to create seed.dat: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(storage); err != nil {
		return fmt.Errorf("failed to encode seed storage: %w", err)
	}

	return nil
}

// encryptSeed encrypts the seed using AES-GCM with a key derived from the password
func encryptSeed(seed []byte, password string) ([]byte, error) {
	// Derive a 32-byte key from the password using SHA-256
	key := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt the seed and prepend the nonce
	encrypted := gcm.Seal(nonce, nonce, seed, nil)
	return encrypted, nil
}

// decryptSeed decrypts the seed using AES-GCM with a key derived from the password
func decryptSeed(encryptedSeed []byte, password string) ([]byte, error) {
	// Derive a 32-byte key from the password using SHA-256
	key := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedSeed) < nonceSize {
		return nil, errors.New("encrypted seed too short")
	}

	nonce, ciphertext := encryptedSeed[:nonceSize], encryptedSeed[nonceSize:]
	seed, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return seed, nil
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

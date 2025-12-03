package easywallet

import (
	"math/big"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/shopspring/decimal"
)

type Provider interface {
	GetAddress() (string, error)
	GetBalance() (*big.Float, error)
	Send(string, decimal.Decimal) (string, error)
}

type BaseProvider struct {
	Key        *hdkeychain.ExtendedKey
	serviceUrl string
}

func NewBaseProvider(key *hdkeychain.ExtendedKey, serviceUrl string) BaseProvider {
	return BaseProvider{Key: key, serviceUrl: serviceUrl}
}

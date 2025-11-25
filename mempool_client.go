package easywallet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// UTXO represents an unspent transaction output
type UTXO struct {
	TxID   string `json:"txid"`
	Vout   uint32 `json:"vout"`
	Status struct {
		Confirmed   bool   `json:"confirmed"`
		BlockHeight uint64 `json:"block_height,omitempty"`
		BlockHash   string `json:"block_hash,omitempty"`
		BlockTime   int64  `json:"block_time,omitempty"`
	} `json:"status"`
	Value int64 `json:"value"`
}

// MempoolClient handles API requests to mempool.space
type MempoolClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func getClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return &http.Client{
			Transport: &http.Transport{
				Protocols: &http.Protocols{},
			},
			Timeout: 30 * time.Second,
		}, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy:     http.ProxyURL(parsedURL),
		Protocols: &http.Protocols{},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}

// NewMempoolClient creates a new mempool.space client
func NewMempoolClient(baseURL string, proxyURL string) (*MempoolClient, error) {
	fullURL := baseURL + "/api"
	client, err := getClientWithProxy(proxyURL)

	if err != nil {
		return nil, err
	}

	return &MempoolClient{
		BaseURL:    fullURL,
		HTTPClient: client,
	}, nil
}

// GetUTXOs retrieves UTXOs for a given Bitcoin address
func (c *MempoolClient) GetUTXOs(address string) ([]UTXO, error) {
	url := fmt.Sprintf("%s/address/%s/utxo", c.BaseURL, address)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed: %s - %s", resp.Status, string(body))
	}

	var utxos []UTXO
	if err := json.NewDecoder(resp.Body).Decode(&utxos); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return utxos, nil
}

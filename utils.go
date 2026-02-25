package easywallet

import (
	"net/http"
	"net/url"
	"time"
)

func GetClientWithProxy(proxyURL string) (*http.Client, error) {
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

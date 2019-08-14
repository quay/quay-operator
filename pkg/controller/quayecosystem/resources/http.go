package resources

import (
	"crypto/tls"
	"net/http"
)

// GetDefaultHTTPClient returns a configured HTTP Client
func GetDefaultHTTPClient() *http.Client {
	t := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{
		Transport: &t,
	}

	return &httpClient

}

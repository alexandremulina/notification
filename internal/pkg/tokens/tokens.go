package tokens

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go-data-distributor-notification/internal/config"
)

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	NotBeforePolicy  int64  `json:"not-before-policy"`
	Scope            string `json:"scope"`
}

func GetDDToken() (string, error) {
	tokenUrl := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		config.GlobalConfig.DDIAMURL,
		config.GlobalConfig.DDRealm,
	)
	tokenResponse, err := postTokenEndpoint(tokenUrl,
		config.GlobalConfig.DDClientID,
		config.GlobalConfig.DDClientSecret,
	)

	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	return tokenResponse.AccessToken, nil
}

func GetRpToken() (string, error) {
	tokenUrl := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		config.GlobalConfig.RPIAMURL,
		config.GlobalConfig.RPRealm,
	)
	tokenResponse, err := postTokenEndpoint(tokenUrl,
		config.GlobalConfig.DDClientID,
		config.GlobalConfig.DDClientSecret,
	)

	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	return tokenResponse.AccessToken, nil
}

func postTokenEndpoint(tokenUrl, clientId, clientSecret string) (*TokenResponse, error) {

	res, err := http.PostForm(tokenUrl, url.Values{
		"client_id":     {clientId},
		"client_secret": {clientSecret},
		"grant_type":    {"client_credentials"},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("%v responded with status code %d", tokenUrl, res.StatusCode)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal token response: %w", err)
	}

	return &tokenResponse, nil
}

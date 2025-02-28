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

func GetToken() (string, error) {
	tokenUrl := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		config.GlobalConfig.IAMUrl,
		config.GlobalConfig.Realm,
	)

	res, err := http.PostForm(tokenUrl, url.Values{
		"client_id":     {config.GlobalConfig.ClientID},
		"client_secret": {config.GlobalConfig.ClientSecret},
		"grant_type":    {"client_credentials"},
	})

	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint responded with status code %d", res.StatusCode)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal token response: %w", err)
	}

	return tokenResponse.AccessToken, nil
}

package keycloak

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"go-data-distributor-notification/internal/config"
	apiErrors "go-data-distributor-notification/internal/pkg/errors"
	"go-data-distributor-notification/internal/pkg/tokens"

	"github.com/gin-gonic/gin"
)

type ClientCreationRequest struct {
	ClientID                  string   `json:"clientId"`
	Name                      string   `json:"name"`
	Enabled                   bool     `json:"enabled"`
	RedirectURIs              []string `json:"redirectUris"`
	WebOrigins                []string `json:"webOrigins"`
	Protocol                  string   `json:"protocol"`
	ClientAuthenticatorType   string   `json:"clientAuthenticatorType"`
	PublicClient              bool     `json:"publicClient"`
	DirectAccessGrantsEnabled bool     `json:"directAccessGrantsEnabled"`
	ServiceAccountsEnabled    bool     `json:"serviceAccountsEnabled"`
	AlwaysDisplayInConsole    bool     `json:"alwaysDisplayInConsole"`
}
type PartialClientResponse struct {
	ID       string `json:"id"`
	ClientID string `json:"clientId"`
	Enabled  bool   `json:"enabled"`
}

func CreateClient(clientId, name, redirectUrl string) error {
	clientReq := ClientCreationRequest{
		ClientID:                  clientId,
		Name:                      name,
		Enabled:                   true,
		RedirectURIs:              []string{redirectUrl},
		WebOrigins:                []string{"*"},
		Protocol:                  "openid-connect",
		ClientAuthenticatorType:   "client-secret",
		PublicClient:              false,
		DirectAccessGrantsEnabled: true,
		ServiceAccountsEnabled:    true,
		AlwaysDisplayInConsole:    true,
	}

	url := fmt.Sprintf("%s/admin/realms/%s/clients", config.GlobalConfig.DDIAMURL, config.GlobalConfig.DDRealm)
	_, err := keycloakRequest("POST", url, clientReq)
	if err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("Keycloak client created: %s: %s", name, clientId))
	return nil
}

func getAllClients(c *gin.Context) []PartialClientResponse {
	url := fmt.Sprintf("%s/admin/realms/%s/clients", config.GlobalConfig.DDIAMURL, config.GlobalConfig.DDRealm)
	bytes, err := keycloakRequest("GET", url, "")
	if err != nil {
		apiErrors.HandleError(c, err)
		return nil
	}

	var partialClientInfo []PartialClientResponse
	err = json.Unmarshal(bytes, &partialClientInfo)
	if err != nil {
		apiErrors.HandleError(c, err)
		return nil
	}

	return partialClientInfo
}

func ResetClientSecret(c *gin.Context, clientId string) error {
	partialClients := getAllClients(c)

	var foundClient PartialClientResponse
	for _, client := range partialClients {
		if client.ClientID == clientId {
			foundClient = client
		}
	}

	if foundClient.ID == "" {
		apiErrors.HandleError(c, apiErrors.NotFoundError(fmt.Sprintf("Couldn't find tenant with id %v", clientId)))
	}

	url := fmt.Sprintf("%s/admin/realms/%s/clients/%s/client-secret",
		config.GlobalConfig.DDIAMURL,
		config.GlobalConfig.DDRealm,
		foundClient.ID,
	)

	_, err := keycloakRequest("POST", url, "{}")
	if err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("Keycloak reset client secret: %s", clientId))
	return nil
}

func keycloakRequest(method, url string, body interface{}) ([]byte, error) {
	token, err := tokens.GetDDToken()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get DD token: %v", err))
		panic(err)
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal client creation request: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, apiErrors.ConflictError("keycloak resource already exists")
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed request to keycloak, status: %d", resp.StatusCode)
	}

	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from keycloak")
	}

	return res, nil
}

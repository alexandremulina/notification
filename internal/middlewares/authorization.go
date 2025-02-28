package middlewares

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go-data-distributor-notification/internal/config"
	apiErrors "go-data-distributor-notification/internal/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

func Authorization() gin.HandlerFunc {
	if config.GlobalConfig.Environment == "dev" || config.GlobalConfig.Environment == "local" {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		if err := validateAndParseToken(c); err != nil {
			apiErrors.HandleError(c, apiErrors.UnauthorizedError(err.Error()))
			return
		}
		c.Next()
	}
}

func validateAndParseToken(c *gin.Context) error {
	tokenStr := c.Request.Header.Get("Authorization")
	if tokenStr == "" {
		return errors.New("missing authorization header")
	}

	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")

	if err := introspectToken(tokenStr); err != nil {
		return err
	}

	token, _ := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		// we don't need to validate the token, as we are introspecting it
		return nil, nil
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if tenantID, exists := claims["clientId"]; exists {
			c.Set("tenant_id", tenantID)
		}

		if realm, exists := claims["realm_access"].(map[string]interface{}); exists {
			c.Set("roles", realm["roles"])
		}
	}

	return nil
}

func introspectToken(tokenStr string) error {
	introspectUrl := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token/introspect",
		config.GlobalConfig.IAMUrl,
		config.GlobalConfig.Realm,
	)

	resp, err := http.PostForm(introspectUrl, url.Values{
		"token_type_hint": {"requesting_party_token"},
		"token":           {tokenStr},
		"client_id":       {config.GlobalConfig.ClientID},
		"client_secret":   {config.GlobalConfig.ClientSecret},
		"grant_type":      {"client_credentials"},
	})
	if err != nil {
		return fmt.Errorf("failed to introspect token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to introspect token: %s", resp.Status)
	}

	var introspectResp struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&introspectResp); err != nil {
		return fmt.Errorf("failed to decode introspect token response: %w", err)
	}

	if !introspectResp.Active {
		return errors.New("token is not active")
	}

	return nil
}

package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type BearerChallenge struct {
	Realm   string
	Service string
	Scope   string
}

func ParseWWWAuthenticate(header string) (BearerChallenge, error) {
	if header == "" {
		return BearerChallenge{}, fmt.Errorf("missing WWW-Authenticate header")
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return BearerChallenge{}, fmt.Errorf("unsupported auth scheme")
	}
	body := strings.TrimSpace(header[len("Bearer "):])
	parts := strings.Split(body, ",")
	challenge := BearerChallenge{}
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		switch key {
		case "realm":
			challenge.Realm = value
		case "service":
			challenge.Service = value
		case "scope":
			challenge.Scope = value
		}
	}
	if challenge.Realm == "" {
		return BearerChallenge{}, fmt.Errorf("invalid bearer challenge")
	}
	return challenge, nil
}

func FetchBearerToken(client *http.Client, challenge BearerChallenge) (string, error) {
	endpoint, err := url.Parse(challenge.Realm)
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	if challenge.Service != "" {
		query.Set("service", challenge.Service)
	}
	if challenge.Scope != "" {
		query.Set("scope", challenge.Scope)
	}
	endpoint.RawQuery = query.Encode()

	resp, err := client.Get(endpoint.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get token failed: %s", resp.Status)
	}
	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.Token != "" {
		return payload.Token, nil
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	return "", fmt.Errorf("token missing in response")
}

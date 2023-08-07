package libmangal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type AnilistLoginCredentials struct {
	ID     string
	Secret string
	Code   string
}

// anilistStoreAccessCodeStoreKey is the key used to store Anilist access code.
// It's needed, since the KV interface always expects a key to be passed.
const anilistStoreAccessCodeStoreKey = "hi"

func (a *Anilist) Logout() error {
	return a.options.AccessTokenStore.Delete(anilistStoreAccessCodeStoreKey)
}

// Authorize will obtain Anilist token for API requests
func (a *Anilist) Authorize(
	ctx context.Context,
	credentials AnilistLoginCredentials,
) error {
	a.logger.Log("logging in to Anilist")

	for _, t := range []struct {
		name  string
		value string
	}{
		{"id", credentials.ID},
		{"secret", credentials.Secret},
		{"code", credentials.Code},
	} {
		if t.value == "" {
			return AnilistError{fmt.Errorf("%s is empty", t.name)}
		}
	}

	body, err := json.Marshal(map[string]string{
		"client_id":     credentials.ID,
		"client_secret": credentials.Secret,
		"code":          credentials.Code,
		"grant_type":    "authorization_code",
		"redirect_uri":  "https://anilist.co/api/v2/oauth/pin",
	})
	if err != nil {
		return err
	}

	if err != nil {
		return AnilistError{err}
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://anilist.co/api/v2/oauth/token",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return AnilistError{err}
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := a.options.HTTPClient.Do(request)
	if err != nil {
		return AnilistError{err}
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return AnilistError{errors.New(response.Status)}
	}

	var authResponse struct {
		AccessToken string `json:"access_token"`
	}

	err = json.NewDecoder(response.Body).Decode(&authResponse)
	if err != nil {
		return AnilistError{err}
	}

	if err := a.options.AccessTokenStore.Set(anilistStoreAccessCodeStoreKey, authResponse.AccessToken); err != nil {
		return err
	}

	a.accessToken = authResponse.AccessToken
	return nil
}

func (a *Anilist) IsAuthorized() bool {
	return a.accessToken != ""
}

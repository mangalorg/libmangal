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

func (a *Anilist) Login(
	ctx context.Context,
	credentials AnilistLoginCredentials,
) error {
	a.options.Log("logging in to Anilist")

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

	var buffer = bytes.NewBuffer(nil)

	err := json.NewEncoder(buffer).Encode(map[string]string{
		"client_id":     credentials.ID,
		"client_secret": credentials.Secret,
		"code":          credentials.Code,
		"grant_type":    "authorization_code",
		"redirect_uri":  "https://anilist.co/api/v2/oauth/pin",
	})
	if err != nil {
		return AnilistError{err}
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://anilist.co/api/v2/oauth/token",
		buffer,
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

	a.accessToken = authResponse.AccessToken
	return nil
}

func (a *Anilist) IsAuthorized() bool {
	return a.accessToken != ""
}

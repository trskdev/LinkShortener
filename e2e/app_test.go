//go:build integration

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLinkResp struct {
	ID        int64  `json:"id"`
	ShortCode string `json:"short_code"`
	LongUrl   string `json:"long_url"`
	Clicks    int32  `json:"clicks"`
}

type codeURL struct {
	Code string `json:"code,omitempty"`
	URL  string `json:"url,omitempty"`
}

func Test_E2E_CreateAndGetLink(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	linkPayload := codeURL{Code: "test1", URL: "https://test1.com"}
	body, err := json.Marshal(linkPayload)
	require.NoError(t, err, "failed to marshall link payload")

	createRes, err := client.Post(appURL+"/shorten", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "failed to send POST request")
	defer createRes.Body.Close()

	require.Equal(t, http.StatusCreated, createRes.StatusCode)

	var createdLink testLinkResp
	err = json.NewDecoder(createRes.Body).Decode(&createdLink)
	require.NoError(t, err, "failed to decode POST response")
	linkCode := createdLink.ShortCode

	getRes, err := client.Get(appURL + "/" + linkCode)
	require.NoError(t, err, "failed to send GET request")
	defer getRes.Body.Close()

	require.Equal(t, http.StatusFound, getRes.StatusCode)

	location := getRes.Header.Get("Location")
	assert.Equal(t, linkPayload.URL, location)
}

func Test_E2E_GetLinkAndClicks(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	linkPayload := codeURL{Code: "test2", URL: "https://test2.com"}
	body, err := json.Marshal(linkPayload)
	require.NoError(t, err, "failed to marshall link payload")

	createRes, err := client.Post(appURL+"/shorten", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "failed to send POST request")
	defer createRes.Body.Close()
	require.Equal(t, http.StatusCreated, createRes.StatusCode)

	getRes, err := client.Get(appURL + "/" + linkPayload.Code)
	require.NoError(t, err, "failed to send GET request")
	defer getRes.Body.Close()
	require.Equal(t, http.StatusFound, getRes.StatusCode)

	getClicksRes, err := client.Get(appURL + "/stats/" + linkPayload.Code)
	require.NoError(t, err, "failed to send GET request")
	defer getClicksRes.Body.Close()
	require.Equal(t, http.StatusOK, getClicksRes.StatusCode)

	var resultClicks struct {
		Clicks int32 `json:"clicks"`
	}
	err = json.NewDecoder(getClicksRes.Body).Decode(&resultClicks)
	require.NoError(t, err, "failed to decode GET response")
	assert.Equal(t, int32(1), resultClicks.Clicks)

	getRes, err = client.Get(appURL + "/" + linkPayload.Code)
	require.NoError(t, err, "failed to send GET request")
	defer getRes.Body.Close()
	require.Equal(t, http.StatusFound, getRes.StatusCode)

	getClicksRes, err = client.Get(appURL + "/stats/" + linkPayload.Code)
	require.NoError(t, err, "failed to send GET request")
	defer getClicksRes.Body.Close()
	require.Equal(t, http.StatusOK, getClicksRes.StatusCode)

	err = json.NewDecoder(getClicksRes.Body).Decode(&resultClicks)
	require.NoError(t, err, "failed to decode GET response")
	assert.Equal(t, int32(2), resultClicks.Clicks)
}

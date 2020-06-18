package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/pinpt/agent.next/sdk"
)

// Creds generic credentials object
type Creds interface {
	auth() string
}

// BasicCreds basic authorization object, username and password
type BasicCreds struct {
	Username string
	Password string
}

func (b *BasicCreds) auth() string {
	auth := b.Username + ":" + b.Password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// OAuthCreds oauth2 authorization object
type OAuthCreds struct {
	Token   string
	Refresh string
	Manager sdk.Manager
}

func (o *OAuthCreds) auth() string {
	return "Bearer " + o.Token
}
func (o *OAuthCreds) refresh(refType string) error {
	token, err := o.Manager.RefreshOAuth2Token(refType, o.Refresh)
	if err != nil {
		return err
	}
	o.Token = token
	return nil
}

var _ Creds = (*BasicCreds)(nil)
var _ Creds = (*OAuthCreds)(nil)

// API the api object
type API struct {
	customerID  string
	refType     string
	client      sdk.HTTPClient
	concurrency int
	logger      sdk.Logger
	creds       Creds
}

// New creates a new instance of the api object
func New(logger sdk.Logger, client sdk.HTTPClient, customerID string, refType string, concurrency int64, creds Creds) *API {
	return &API{
		client:      client,
		concurrency: int(concurrency),
		logger:      logger,
		refType:     refType,
		customerID:  customerID,
		creds:       creds,
	}
}

func ensuserParams(p url.Values) url.Values {
	if p == nil {
		p = url.Values{}
	}
	if p.Get("api-version") == "" {
		p.Set("api-version", "5.1")
	}
	return p
}

func (a *API) paginate(endpoint string, params url.Values, out chan<- objects) error {
	defer close(out)
	pageNum := 0
	for {

		var top string
		if top = params.Get("$top"); top == "" {
			top = "100"
			params.Set("$top", top)
		}
		maxPage, _ := strconv.Atoi(top)
		if pageNum > 0 {
			params.Set("$skip", strconv.Itoa(maxPage*pageNum))
		}
		var page pageResponse
		_, err := a.get(endpoint, params, &page)
		if err != nil {
			return err
		}
		if page.Count > 0 {
			out <- page.Value
		}
		if page.Count < int64(maxPage) {
			return nil
		}
		pageNum++
	}
}

func (a *API) get(endpoint string, params url.Values, out interface{}) (*sdk.HTTPResponse, error) {

	params = ensuserParams(params)
	resp, err := a.client.Get(out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params), sdk.WithAuthorization(a.creds.auth()))
	if resp.StatusCode == http.StatusUnauthorized {
		if creds, ok := a.creds.(*OAuthCreds); ok {
			if err := creds.refresh(a.refType); err != nil {
				return resp, err
			}
			return a.get(endpoint, params, out)
		}
		return nil, fmt.Errorf("error calling api. response code: %v", resp.StatusCode)
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (a *API) post(endpoint string, data interface{}, params url.Values, out interface{}) (*sdk.HTTPResponse, error) {
	params = ensuserParams(params)
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return a.client.Post(bytes.NewBuffer(b), &out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params), sdk.WithAuthorization(a.creds.auth()))
}

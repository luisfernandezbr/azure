package api

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/pinpt/agent.next/sdk"
)

// Creds generic credentials object

// API the api object
type API struct {
	customerID  string
	refType     string
	client      sdk.HTTPClient
	concurrency int
	logger      sdk.Logger
	creds       sdk.WithHTTPOption
}

// New creates a new instance of the api object
func New(logger sdk.Logger, client sdk.HTTPClient, customerID string, refType string, concurrency int64, creds sdk.WithHTTPOption) *API {
	return &API{
		client:      client,
		concurrency: int(concurrency),
		logger:      logger,
		refType:     refType,
		customerID:  customerID,
		creds:       creds,
	}
}

func ensureParams(p url.Values) url.Values {
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
	params = ensureParams(params)
	return a.client.Get(out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params), a.creds)
}

func (a *API) post(endpoint string, data interface{}, params url.Values, out interface{}) (*sdk.HTTPResponse, error) {
	params = ensureParams(params)
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return a.client.Post(bytes.NewBuffer(b), &out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params), a.creds)
}

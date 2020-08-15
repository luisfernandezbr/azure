package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"github.com/pinpt/agent.next/sdk"
)

// Creds generic credentials object

// API the api object
type API struct {
	customerID  string
	instanceID  string
	refType     string
	client      sdk.HTTPClient
	concurrency int
	logger      sdk.Logger
	creds       sdk.WithHTTPOption
	state       sdk.State
	// process_id - type_name - state_name
	statusesMap map[string]map[string]map[string]string
}

// New creates a new instance of the api object
func New(logger sdk.Logger, client sdk.HTTPClient, state sdk.State, customerID, instanceID, refType string, concurrency int64, creds sdk.WithHTTPOption) *API {
	return &API{
		client:      client,
		state:       state,
		concurrency: int(concurrency),
		logger:      logger,
		refType:     refType,
		customerID:  customerID,
		creds:       creds,
		instanceID:  instanceID,
		statusesMap: map[string]map[string]map[string]string{},
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
			if len(page.Value) > 0 {
				out <- page.Value
			} else if len(page.Comments) > 0 {
				out <- page.Comments
			} else {
				var out interface{}
				a.get(endpoint, params, &out)
				sdk.LogError(a.logger, "response is not standard", "resp", sdk.Stringify(out))
				return errors.New("response is not standard")
			}
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
func (a *API) delete(endpoint string, params url.Values, out interface{}) (*sdk.HTTPResponse, error) {
	params = ensureParams(params)
	return a.client.Delete(out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params), a.creds)
}

func (a *API) patch(endpoint string, data interface{}, params url.Values, out interface{}) (*sdk.HTTPResponse, error) {
	params = ensureParams(params)
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return a.client.Patch(bytes.NewBuffer(b), out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params), a.creds)
}

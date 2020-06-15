package api

import (
	"bytes"
	"encoding/json"
	"net/url"
	"sync"

	"github.com/pinpt/agent.next/sdk"
)

type API struct {
	customerID  string
	refType     string
	client      sdk.HTTPClient
	concurrency int64
	logger      sdk.Logger
}

func New(logger sdk.Logger, client sdk.HTTPClient, customerID string, refType string, concurrency int64) *API {
	return &API{
		client:      client,
		concurrency: concurrency,
		logger:      logger,
	}
}

func (a *API) get(endpoint string, params url.Values, out interface{}) (*sdk.HTTPResponse, error) {
	if params == nil {
		params = url.Values{}
	}
	if params.Get("api-version") == "" {
		params.Set("api-version", "5.1")
	}
	return a.client.Get(out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params))
}

func (a *API) post(endpoint string, data interface{}, params url.Values, out interface{}) (*sdk.HTTPResponse, error) {
	if params == nil {
		params = url.Values{}
	}
	if params.Get("api-version") == "" {
		params.Set("api-version", "5.1")
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return a.client.Post(bytes.NewBuffer(b), &out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params))
}

// Async simple async interface
type Async interface {
	Do(f func() error)
	Wait() error
}

type async struct {
	funcs chan func() error
	err   error
	wg    sync.WaitGroup
	mu    sync.Mutex
}

// NewAsync instantiates a new Async object
func NewAsync(concurrency int) Async {
	a := &async{}
	a.funcs = make(chan func() error, concurrency)
	a.wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for f := range a.funcs {
				a.mu.Lock()
				rerr := a.err
				a.mu.Unlock()
				if rerr == nil {
					if err := f(); err != nil {
						a.mu.Lock()
						a.err = err
						a.mu.Unlock()
					}
				}
			}
			a.wg.Done()
		}()
	}
	return a
}

func (a *async) Do(f func() error) {
	a.funcs <- f
}

func (a *async) Wait() error {
	close(a.funcs)
	a.wg.Wait()
	return a.err
}

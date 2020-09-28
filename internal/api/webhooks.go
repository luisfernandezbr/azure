package api

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/pinpt/agent/sdk"
)

var eventTypes = []string{
	"git.push",                // Code pushed
	"git.pullrequest.created", // Pull request created
	"git.pullrequest.merged",  // Pull request merge commit created
	"git.pullrequest.updated", // Pull request updated

	"workitem.created", // Work item created
	"workitem.deleted", // Work item deleted
	"workitem.updated", // Work item updated
}

// RemoveAllWebHooks gets the repos from a project
func (a *API) RemoveAllWebHooks() error {
	endpoint := "/_apis/hooks/subscriptions"

	params := url.Values{}
	params.Set("publisherId", "tfs")
	params.Set("consumerId", "webHooks")
	params.Set("consumerActionId", "httpRequest")

	out := make(chan objects)
	errochan := make(chan error)
	go func() {
		for object := range out {
			var value []webhookPayload
			if err := object.Unmarshal(&value); err != nil {
				errochan <- err
				return
			}
			async := sdk.NewAsync(int(a.concurrency))
			for _, res := range value {
				if strings.Contains(res.ConsumerInputs.URL, "pinpoint.com/hook") {
					id := res.ID
					async.Do(func() error {
						return a.DeleteWebhooks([]string{id})
					})
				}
			}
			if err := async.Wait(); err != nil {
				errochan <- err
				return
			}
		}
		errochan <- nil
	}()
	// ===========================================
	go func() {
		err := a.paginate(endpoint, params, out)
		if err != nil {
			errochan <- err
		}
	}()
	return <-errochan
}

// CreateWebhook creates webhook
func (a *API) CreateWebhook(url, projid string) (ids []string, _ error) {

	sdk.LogInfo(a.logger, "creating webhooks for project", "project", projid)
	mu := sync.Mutex{}
	async := sdk.NewAsync(int(a.concurrency))
	for _, _evt := range eventTypes {
		evt := _evt
		async.Do(func() error {
			var payload webhookPayload
			payload.ConsumerActionID = "httpRequest"
			payload.ConsumerID = "webHooks"
			payload.ConsumerInputs.URL = url
			payload.EventType = evt
			payload.PublisherID = "tfs"
			payload.PublisherInputs.ProjectID = projid
			payload.ResourceVersion = "1.0"
			payload.Scope = 1
			endpoint := "/_apis/hooks/subscriptions"
			var out struct {
				ID string `json:"id"`
			}
			if _, err := a.post(endpoint, payload, nil, &out); err != nil {
				return err
			}
			mu.Lock()
			ids = append(ids, out.ID)
			mu.Unlock()
			return nil
		})
	}
	err := async.Wait()
	return ids, err
}

// DeleteWebhooks removes a webhook
func (a *API) DeleteWebhooks(webhookIDs []string) error {
	async := sdk.NewAsync(a.concurrency)
	for _, _id := range webhookIDs {
		id := _id
		async.Do(func() error {
			endpoint := fmt.Sprintf("_apis/hooks/subscriptions/%s", url.PathEscape(id))
			var out interface{}
			_, err := a.delete(endpoint, nil, &out)
			return err
		})
	}
	return async.Wait()
}

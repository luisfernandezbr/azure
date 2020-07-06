package api

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/pinpt/agent.next/sdk"
)

var eventTypes = []string{
	"git.push",                // Code pushed
	"git.pullrequest.created", // Pull request created
	"git.pullrequest.merged",  // Pull request merge commit created
	"git.pullrequest.updated", // Pull request updated

	"workitem.created",   // Work item created
	"workitem.deleted",   // Work item deleted
	"workitem.restored",  // Work item restored
	"workitem.updated",   // Work item updated
	"workitem.commented", // Work item commented on
}

// RemoveAllWebHooks gets the repos from a project
func (a *API) RemoveAllWebHooks(concurr int64) error {
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
			async := sdk.NewAsync(int(concurr))
			for _, res := range value {
				if strings.Contains(res.ConsumerInputs.URL, "pinpoint.com/hook") {
					id := res.ID
					async.Do(func() error {
						return a.DeleteWebhook(id)
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

// CreateWebhook gets the repos from a project
func (a *API) CreateWebhook(url, projid string, concurr int64) error {

	sdk.LogInfo(a.logger, "creating webhooks for project", "project", projid)
	async := sdk.NewAsync(int(concurr))
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
			var out interface{}
			_, err := a.post(endpoint, payload, nil, &out)
			if err != nil {
				rerr := err.(*sdk.HTTPError)
				b, _ := ioutil.ReadAll(rerr.Body)
				fmt.Println("error", string(b), rerr.StatusCode)
				return err
			}
			fmt.Println(sdk.Stringify(out))
			return nil
		})
	}
	err := async.Wait()
	return err
}

// DeleteWebhook removes a webhook
func (a *API) DeleteWebhook(webhookID string) error {
	endpoint := fmt.Sprintf("_apis/hooks/subscriptions/%s", url.PathEscape(webhookID))
	var out interface{}
	_, err := a.delete(endpoint, nil, &out)
	if err == nil {
		fmt.Println("webhook deleted")
	}
	return err
}

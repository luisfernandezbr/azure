package api

import (
	"fmt"
	"io/ioutil"

	"github.com/pinpt/agent.next/sdk"
)

// CreateWebhook gets the repos from a project
func (a *API) CreateWebhook(url string) error {

	sdk.LogInfo(a.logger, "creating webhooks")
	projs, err := a.FetchProjects()
	if err != nil {
		return err
	}
	for _, proj := range projs {
		var payload webhookPayload
		payload.ConsumerActionID = "httpRequest"
		payload.ConsumerID = "webHooks"
		payload.ConsumerInputs.URL = url
		payload.EventType = "workitem.created"
		payload.PublisherID = "tfs"
		payload.PublisherInputs.ProjectID = proj.RefID
		payload.ResourceVersion = "1.0"
		payload.Scope = 1

		fmt.Println(sdk.Stringify(payload))
		endpoint := "/_apis/hooks/subscriptions"
		var out interface{}
		_, err := a.post(endpoint, payload, nil, &out)
		if err != nil {
			rerr := err.(*sdk.HTTPError)
			b, _ := ioutil.ReadAll(rerr.Body)
			fmt.Println("error", string(b))

			return err
		}
		fmt.Println(sdk.Stringify(out))
	}
	return nil
}

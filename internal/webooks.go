package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pinpt/agent.next.azure/internal/api"
	"github.com/pinpt/agent.next/sdk"
)

type webookPayload struct {
	EventType string `json:"eventType"`
}
type webhookPayloadCreatedDeleted struct {
	Resource struct {
		ID         int `json:"id"`
		WorkItemId int `json:"workItemId"`
	} `json:"resource"`
	ResourceContainers struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"resourceContainers"`
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *AzureIntegration) WebHook(webhook sdk.WebHook) error {
	var payload webookPayload
	if err := json.Unmarshal(webhook.Bytes(), &payload); err != nil {
		return err
	}

	pipe := webhook.Pipe()
	config := webhook.Config()
	ok, concurr := config.GetInt("concurrency")
	if !ok {
		concurr = 10
	}
	auth := config.APIKeyAuth
	if auth == nil {
		return errors.New("Missing --apikey_auth")
	}
	basicAuth := sdk.WithBasicAuth("", auth.APIKey)
	client := g.manager.HTTPManager().New(auth.URL, nil)
	customerID := webhook.CustomerID()
	rawPayload := webhook.Bytes()

	a := api.New(g.logger, client, customerID, g.refType, concurr, basicAuth)

	if strings.HasPrefix(payload.EventType, "workitem.") {
		return g.handleWorkWebHooks(customerID, webhook.IntegrationInstanceID(), payload.EventType, rawPayload, pipe, a)
	}
	return g.handleSourceCodeWebHooks(payload.EventType, pipe, a)
}
func (g *AzureIntegration) handleSourceCodeWebHooks(eventType string, pipe sdk.Pipe, a *api.API) error {
	return nil
}

func (g *AzureIntegration) handleWorkWebHooks(customerID, intID, eventType string, rawPayload []byte, pipe sdk.Pipe, a *api.API) error {

	var data webhookPayloadCreatedDeleted
	if err := json.Unmarshal(rawPayload, &data); err != nil {
		return err
	}

	if eventType == "workitem.deleted" {
		itemID := fmt.Sprint(data.Resource.ID)
		active := false
		update := sdk.WorkIssueUpdate{}
		update.Set.Active = &active
		obj := sdk.NewWorkIssueUpdate(customerID, intID, itemID, g.refType, update)
		return pipe.Write(obj)
	}

	errorchan := make(chan error)
	issueChannel := make(chan *sdk.WorkIssue)
	go func() {
		for each := range issueChannel {
			if err := pipe.Write(each); err != nil {
				errorchan <- err
				return
			}
		}
	}()
	issueCommentChannel := make(chan *sdk.WorkIssueComment)
	go func() {
		for each := range issueCommentChannel {
			if err := pipe.Write(each); err != nil {
				errorchan <- err
				return
			}
		}
	}()

	var itemID string
	if eventType == "workitem.created" {
		itemID = fmt.Sprint(data.Resource.ID)
	} else {
		itemID = fmt.Sprint(data.Resource.WorkItemId)
	}
	projectID := data.ResourceContainers.Project.ID
	go func() {
		if err := a.FetchIssues(projectID, []string{itemID}, issueChannel, issueCommentChannel); err != nil {
			errorchan <- err
			return
		}
		errorchan <- nil
	}()
	return <-errorchan
}

func (g *AzureIntegration) registerWebhook(instance sdk.Instance, concurr int64) error {

	customerID := instance.CustomerID()
	integrationID := instance.IntegrationInstanceID()
	auth := instance.Config().APIKeyAuth

	client := g.manager.HTTPManager().New(auth.URL, nil)
	a := api.New(g.logger, client, customerID, g.refType, concurr, sdk.WithBasicAuth("", auth.APIKey))

	// fetch projects
	projects, err := a.FetchProjects()
	if err != nil {
		return fmt.Errorf("error fetching projects. err: %v", err)
	}
	if err := a.RemoveAllWebHooks(concurr); err != nil {
		return err
	}
	for _, proj := range projects {
		url, err := g.manager.CreateWebHook(customerID, g.refType, integrationID, proj.RefID)
		if err != nil {
			fmt.Println("err", err)
			return err
		}
		if err := a.CreateWebhook(url, proj.RefID, concurr); err != nil {
			return err
		}

	}
	os.Exit(1)
	return nil
}

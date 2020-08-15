package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pinpt/agent.next.azure/internal/api"
	"github.com/pinpt/agent.next/sdk"
)

const webhookVersion = "1" // change this to have the webhook uninstalled and reinstalled new

type webookPayload struct {
	EventType string `json:"eventType"`
}

type webhookPayloadCreatedDeleted struct {
	Resource struct {
		ID         int `json:"id"`
		WorkItemID int `json:"workItemId"`
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
	instanceID := webhook.IntegrationInstanceID()

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

	a := api.New(g.logger, client, webhook.State(), customerID, instanceID, g.refType, concurr, basicAuth)

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
		itemID = fmt.Sprint(data.Resource.WorkItemID)
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

func (g *AzureIntegration) registerWebHooks(instance sdk.Instance, concurr int64) error {

	customerID := instance.CustomerID()
	instanceID := instance.IntegrationInstanceID()
	state := instance.State()
	auth := instance.Config().APIKeyAuth

	client := g.manager.HTTPManager().New(auth.URL, nil)
	a := api.New(g.logger, client, instance.State(), customerID, instanceID, g.refType, concurr, sdk.WithBasicAuth("", auth.APIKey))

	// fetch projects
	projects, err := a.FetchProjects()
	if err != nil {
		return fmt.Errorf("error fetching projects. err: %v", err)
	}
	webhookManager := g.manager.WebHookManager()

	for _, proj := range projects {
		if webhookManager.Exists(customerID, instanceID, g.refType, proj.RefID, sdk.WebHookScopeProject) {
			url, err := webhookManager.HookURL(customerID, instanceID, g.refType, proj.RefID, sdk.WebHookScopeProject)
			if err != nil {
				return err
			}
			// check and see if we need to upgrade our webhook
			if strings.Contains(url, "&version="+webhookVersion) {
				sdk.LogInfo(g.logger, "skipping web hook install since already installed")
				continue
			}
			var removed bool
			if removed, err = g.removeWebHook(webhookManager, a, state, customerID, instanceID, proj.RefID); err != nil {
				return err
			}
			if !removed {
				continue
			}
		}

		url, err := webhookManager.Create(customerID, instanceID, g.refType, proj.RefID, sdk.WebHookScopeProject, "version="+webhookVersion)
		if err != nil {
			return err
		}
		// each project creates a bunch of webhooks, we need to store the ids of those in state so that we can delete them later
		ids, err := a.CreateWebhook(url, proj.RefID)
		if err != nil {
			sdk.LogError(g.logger, "error creating webhook", "err", err)
			webhookManager.Errored(customerID, instanceID, g.refType, proj.RefID, sdk.WebHookScopeProject, err)
			continue
		}
		if err := state.Set("webhooks_"+proj.RefID, ids); err != nil {
			return err
		}
	}
	return state.Flush()
}

func (g *AzureIntegration) unregisterWebHooks(instance sdk.Instance, concurr int64) error {
	customerID := instance.CustomerID()
	instanceID := instance.IntegrationInstanceID()
	state := instance.State()
	auth := instance.Config().APIKeyAuth

	client := g.manager.HTTPManager().New(auth.URL, nil)
	a := api.New(g.logger, client, instance.State(), customerID, instanceID, g.refType, concurr, sdk.WithBasicAuth("", auth.APIKey))

	// fetch projects
	projects, err := a.FetchProjects()
	if err != nil {
		return fmt.Errorf("error fetching projects. err: %v", err)
	}
	webhookManager := g.manager.WebHookManager()

	for _, proj := range projects {
		if _, err := g.removeWebHook(webhookManager, a, state, customerID, instanceID, proj.RefID); err != nil {
			return err
		}
	}

	return nil
}

func (g *AzureIntegration) removeWebHook(webhookManager sdk.WebHookManager, a *api.API, state sdk.State, customerID, instanceID, projid string) (bool, error) {
	var ids []string
	if ok, err := state.Get("webhooks_"+projid, &ids); ok {
		if err := a.DeleteWebhooks(ids); err != nil {
			sdk.LogError(g.logger, "error removing webhook", "err", err)
			webhookManager.Errored(customerID, instanceID, g.refType, projid, sdk.WebHookScopeProject, err)
			return false, nil
		}
	} else if err != nil {
		return false, err
	}
	if err := state.Delete("webhooks_" + projid); err != nil {
		return false, err
	}
	if err := webhookManager.Delete(customerID, instanceID, g.refType, projid, sdk.WebHookScopeProject); err != nil {
		return false, err
	}
	return true, nil
}

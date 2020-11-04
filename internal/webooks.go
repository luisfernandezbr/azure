package internal

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pinpt/agent/v4/sdk"
	"github.com/pinpt/azure/internal/api"
)

const webhookVersion = "1" // change this to have the webhook uninstalled and reinstalled new

type webookPayload struct {
	EventType string `json:"eventType"`
}

type webhookWorkPayloadCreatedDeleted struct {
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

type webhookSourcecodePayload struct {
	Resource api.PullRequestResponse `json:"resource"`
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *AzureIntegration) WebHook(webhook sdk.WebHook) error {
	var payload webookPayload
	if err := json.Unmarshal(webhook.Bytes(), &payload); err != nil {
		return err
	}
	logger := webhook.Logger()
	pipe := webhook.Pipe()
	config := webhook.Config()
	integrationID := webhook.IntegrationInstanceID()

	ok, concurr := config.GetInt("concurrency")
	if !ok {
		concurr = 10
	}
	url, creds, err := g.getHTTPCredOpts(logger, config)
	if err != nil {
		return err
	}
	client := g.manager.HTTPManager().New(url, nil)
	customerID := webhook.CustomerID()
	rawPayload := webhook.Bytes()

	a := api.New(logger, client, webhook.State(), webhook.Pipe(), customerID, integrationID, g.refType, concurr, creds)

	if strings.HasPrefix(payload.EventType, "workitem.") {
		return g.handleWorkWebHooks(customerID, webhook.IntegrationInstanceID(), payload.EventType, rawPayload, pipe, a)
	}
	return g.handleSourceCodeWebHooks(logger, payload.EventType, rawPayload, pipe, a)
}
func (g *AzureIntegration) handleSourceCodeWebHooks(logger sdk.Logger, eventType string, rawPayload []byte, pipe sdk.Pipe, a *api.API) error {
	var data webhookSourcecodePayload
	if err := json.Unmarshal(rawPayload, &data); err != nil {
		return err
	}
	if eventType == "git.pullrequest.merged" || eventType == "git.pullrequest.updated" {
		return a.ProcessPullRequests(
			[]api.PullRequestResponse{data.Resource},
			data.Resource.Repository.Project.ID,
			data.Resource.Repository.ID,
			data.Resource.Repository.Name, time.Time{},
		)

	}
	if eventType == "git.pullrequest.created" {
		return a.FetchPullRequests(
			data.Resource.Repository.Project.ID,
			data.Resource.Repository.ID,
			data.Resource.Repository.Name, time.Time{},
		)
	}
	sdk.LogInfo(logger, "webhook type not handled", "type", eventType)
	return nil
}

func (g *AzureIntegration) handleWorkWebHooks(customerID, intID, eventType string, rawPayload []byte, pipe sdk.Pipe, a *api.API) error {

	var data webhookWorkPayloadCreatedDeleted
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

	var itemID string
	if eventType == "workitem.created" {
		itemID = fmt.Sprint(data.Resource.ID)
	} else {
		itemID = fmt.Sprint(data.Resource.WorkItemID)
	}
	projectID := data.ResourceContainers.Project.ID
	return a.FetchIssues(projectID, []string{itemID})
}

func (g *AzureIntegration) registerWebHooks(instance sdk.Instance, concurr int64) error {
	return nil
	logger := instance.Logger()
	customerID := instance.CustomerID()
	integrationID := instance.IntegrationInstanceID()
	state := instance.State()
	pipe := instance.Pipe()
	config := instance.Config()

	var a *api.API
	if config.APIKeyAuth != nil {
		auth := config.APIKeyAuth
		client := g.manager.HTTPManager().New(auth.URL, nil)
		a = api.New(logger, client, state, pipe, customerID, integrationID, g.refType, concurr, sdk.WithBasicAuth("", auth.APIKey))
	} else {
		auth := config.OAuth2Auth
		client := g.manager.HTTPManager().New(auth.URL, nil)
		a = api.New(logger, client, state, pipe, customerID, integrationID, g.refType, concurr, sdk.WithOAuth2Refresh(g.manager, g.refType, auth.AccessToken, *auth.RefreshToken))
	}

	// fetch projects
	projects, err := a.FetchProjects()
	if err != nil {
		return fmt.Errorf("error fetching projects. err: %v", err)
	}
	webhookManager := g.manager.WebHookManager()

	for _, proj := range projects {
		if webhookManager.Exists(customerID, integrationID, g.refType, proj.RefID, sdk.WebHookScopeProject) {
			url, err := webhookManager.HookURL(customerID, integrationID, g.refType, proj.RefID, sdk.WebHookScopeProject)
			if err != nil {
				return err
			}
			// check and see if we need to upgrade our webhook
			if strings.Contains(url, "&version="+webhookVersion) {
				sdk.LogInfo(logger, "skipping web hook install since already installed")
				continue
			}
			var removed bool
			if removed, err = g.removeWebHook(logger, webhookManager, a, state, customerID, integrationID, proj.RefID); err != nil {
				return err
			}
			if !removed {
				continue
			}
		}

		url, err := webhookManager.Create(customerID, integrationID, g.refType, proj.RefID, sdk.WebHookScopeProject, "version="+webhookVersion)
		if err != nil {
			return err
		}
		// each project creates a bunch of webhooks, we need to store the ids of those in state so that we can delete them later
		ids, err := a.CreateWebhook(url, proj.RefID)
		if err != nil {
			sdk.LogError(logger, "error creating webhook", "err", err)
			webhookManager.Errored(customerID, integrationID, g.refType, proj.RefID, sdk.WebHookScopeProject, err)
			continue
		}
		if err := state.Set("webhooks_"+proj.RefID, ids); err != nil {
			return err
		}
	}
	return state.Flush()
}

func (g *AzureIntegration) unregisterWebHooks(instance sdk.Instance, concurr int64) error {
	logger := instance.Logger()
	customerID := instance.CustomerID()
	integrationID := instance.IntegrationInstanceID()
	state := instance.State()
	pipe := instance.Pipe()
	config := instance.Config()

	url, creds, err := g.getHTTPCredOpts(logger, config)
	if err != nil {
		return err
	}

	client := g.manager.HTTPManager().New(url, nil)
	a := api.New(logger, client, state, pipe, customerID, integrationID, g.refType, concurr, creds)

	// fetch projects
	projects, err := a.FetchProjects()
	if err != nil {
		return fmt.Errorf("error fetching projects. err: %v", err)
	}
	webhookManager := g.manager.WebHookManager()

	for _, proj := range projects {
		if _, err := g.removeWebHook(logger, webhookManager, a, state, customerID, integrationID, proj.RefID); err != nil {
			return err
		}
	}

	return nil
}

func (g *AzureIntegration) removeWebHook(logger sdk.Logger, webhookManager sdk.WebHookManager, a *api.API, state sdk.State, customerID, integrationID, projid string) (bool, error) {
	var ids []string
	if ok, err := state.Get("webhooks_"+projid, &ids); ok {
		if err := a.DeleteWebhooks(ids); err != nil {
			sdk.LogError(logger, "error removing webhook", "err", err)
			webhookManager.Errored(customerID, integrationID, g.refType, projid, sdk.WebHookScopeProject, err)
			return false, nil
		}
	} else if err != nil {
		return false, err
	}
	if err := state.Delete("webhooks_" + projid); err != nil {
		return false, err
	}
	if err := webhookManager.Delete(customerID, integrationID, g.refType, projid, sdk.WebHookScopeProject); err != nil {
		return false, err
	}
	return true, nil
}

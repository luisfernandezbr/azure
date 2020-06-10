package internal

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/pinpt/agent.next.azure/internal/api"
	"github.com/pinpt/agent.next/sdk"
)

// AzureIntegration is an integration for Azure
type AzureIntegration struct {
	logger  sdk.Logger
	config  sdk.Config
	manager sdk.Manager
	refType string

	graphQL    sdk.GraphQLClient
	httpClient sdk.HTTPClient
}

var _ sdk.Integration = (*AzureIntegration)(nil)

// Start is called when the integration is starting up
func (g *AzureIntegration) Start(logger sdk.Logger, config sdk.Config, manager sdk.Manager) error {
	g.logger = sdk.LogWith(logger, "pkg", "azure")
	g.config = config
	g.manager = manager
	g.refType = "azure"
	sdk.LogInfo(g.logger, "starting")
	return nil
}

// Enroll is called when a new integration instance is added
func (g *AzureIntegration) Enroll(instance sdk.Instance) error {
	sdk.LogInfo(g.logger, "enroll not implemented")
	return nil
}

// Dismiss is called when an existing integration instance is removed
func (g *AzureIntegration) Dismiss(instance sdk.Instance) error {
	sdk.LogInfo(g.logger, "dismiss not implemented")
	return nil
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *AzureIntegration) WebHook(webhook sdk.WebHook) error {
	sdk.LogInfo(g.logger, "webhook not implemented")
	return nil
}

// Stop is called when the integration is shutting down for cleanup
func (g *AzureIntegration) Stop() error {
	sdk.LogInfo(g.logger, "stopping")
	return nil
}

// Export is called to tell the integration to run an export
func (g *AzureIntegration) Export(export sdk.Export) error {
	sdk.LogInfo(g.logger, "export started")

	// Pipe must be called to begin an export and receive a pipe for sending data
	pipe := export.Pipe()

	// State is a customer specific state object for this integration and customer
	state := export.State()

	// CustomerID will return the customer id for the export
	customerID := export.CustomerID()

	sdk.LogDebug(g.logger, "export starting")
	// Config is any customer specific configuration for this customer
	config := export.Config()
	ok, token := config.GetString("access_token")
	if !ok {
		return errors.New("Missing access_token")
	}
	client := g.manager.HTTPManager().New("https://dev.azure.com/penrique", map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+token)),
	})
	a := api.New(client, customerID, g.refType)
	projects, err := a.FetchProjects()
	if err != nil {
		panic(err)
	}
	usermap := map[string]*sdk.WorkUser{}
	for _, proj := range projects {
		ids, err := a.FetchTeams(proj.RefID)
		if err != nil {
			fmt.Println("1 ERROR", err)
			continue
		}
		err = a.FetchUsers(proj.RefID, ids, usermap)
		if err != nil {
			fmt.Println("2 ERROR", err)
			continue
		}
		sprints, err := a.FetchSprints(proj.RefID, ids)
		if err != nil {
			fmt.Println("2 ERROR", err)
			continue
		}
		for _, spr := range sprints {
			pipe.Write(spr)
		}
		var updated time.Time
		var strTime string
		if ok, _ := state.Get("issues_updated_"+proj.RefID, &strTime); ok {
			updated, _ = time.Parse(time.RFC3339Nano, strTime)
		}
		issues, err := a.FetchIssues(proj.RefID, updated)
		if err != nil {
			fmt.Println(" ERROR", err)
		}
		state.Set("issues_updated_"+proj.RefID, time.Now().Format(time.RFC3339Nano))
		for _, iss := range issues {
			pipe.Write(iss)
		}
		pipe.Write(proj)
	}
	for _, urs := range usermap {
		pipe.Write(urs)
	}

	return nil
}

package internal

import (
	"fmt"
	"os"

	"github.com/pinpt/agent.next.azure/internal/api"
	"github.com/pinpt/agent.next/sdk"
)

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

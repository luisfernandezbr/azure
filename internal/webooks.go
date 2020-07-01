package internal

import (
	"github.com/pinpt/agent.next.azure/internal/api"
	"github.com/pinpt/agent.next/sdk"
)

func (g *AzureIntegration) registerWebhook(org, customerID, token, url string, concurr int64) error {
	client := g.manager.HTTPManager().New("https://dev.azure.com/"+org, nil)
	a := api.New(g.logger, client, customerID, g.refType, concurr, sdk.WithBasicAuth("", token))
	return a.CreateWebhook(url)
}

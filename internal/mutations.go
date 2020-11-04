package internal

import (
	"errors"

	"github.com/pinpt/agent/v4/sdk"
	"github.com/pinpt/azure/internal/api"
)

// Mutation is called when a mutation is received on behalf of the integration
func (g *AzureIntegration) Mutation(mut sdk.Mutation) (*sdk.MutationResponse, error) {

	logger := mut.Logger()
	auth := mut.User().APIKeyAuth
	if auth == nil {
		return nil, errors.New("missing auth")
	}
	customerID := mut.CustomerID()
	integrationID := mut.IntegrationInstanceID()
	basicAuth := sdk.WithBasicAuth("", auth.APIKey)
	client := g.manager.HTTPManager().New(auth.URL, nil)
	a := api.New(logger, client, mut.State(), mut.Pipe(), customerID, integrationID, g.refType, 1, basicAuth)
	switch mut.Action() {
	case sdk.CreateAction:
		switch mu := mut.Payload().(type) {
		case *sdk.WorkIssueCreateMutation:
			// mu.Type.Name should be something like Bug, Epic, Issue, etc.
			if err := a.CreateIssue(mu); err != nil {
				return nil, err
			}
		}
	case sdk.UpdateAction:
		switch mu := mut.Payload().(type) {
		case *sdk.WorkIssueUpdateMutation:
			if err := a.UpdateIssue(mut.ID(), mu); err != nil {
				return nil, err
			}
		case *sdk.SourcecodePullRequestUpdateMutation:
			if err := a.UpdatePullRequest(mut.ID(), mu); err != nil {
				return nil, err
			}
		}

	case sdk.DeleteAction:

	}
	sdk.LogInfo(logger, "mutation not implemented")
	return nil, nil
}

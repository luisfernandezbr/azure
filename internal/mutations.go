package internal

import (
	"errors"

	"github.com/pinpt/agent.next.azure/internal/api"
	"github.com/pinpt/agent.next/sdk"
)

// Mutation is called when a mutation is received on behalf of the integration
func (g *AzureIntegration) Mutation(mut sdk.Mutation) error {

	auth := mut.User().APIKeyAuth
	if auth == nil {
		return errors.New("missing auth")
	}
	customerID := mut.CustomerID()
	basicAuth := sdk.WithBasicAuth("", auth.APIKey)
	client := g.manager.HTTPManager().New(auth.URL, nil)
	a := api.New(g.logger, client, mut.State(), customerID, g.refType, 1, basicAuth)
	_ = a
	switch mut.Action() {
	case sdk.CreateAction:
		switch mu := mut.Payload().(type) {
		case *sdk.WorkIssueCreateMutation:
			// mu.Type.Name should be something like Bug, Epic, Issue, etc.
			if err := a.CreateIssue(mu); err != nil {
				return err
			}
		}
	case sdk.UpdateAction:
		switch mu := mut.Payload().(type) {
		case *sdk.WorkIssueUpdateMutation:
			if err := a.UpdateIssue(mut.ID(), mu); err != nil {
				return err
			}
		case *sdk.SourcecodePullRequestUpdateMutation:
			if err := a.UpdatePullRequest(mut.ID(), mu); err != nil {
				return err
			}
		}

	case sdk.DeleteAction:

	}
	sdk.LogInfo(g.logger, "mutation not implemented")
	return nil
}

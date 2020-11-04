package internal

import (
	"fmt"
	"time"

	"github.com/pinpt/agent/v4/sdk"
	"github.com/pinpt/azure/internal/api"
)

// Export is called to tell the integration to run an export
func (g *AzureIntegration) Export(export sdk.Export) error {
	logger := export.Logger()
	pipe := export.Pipe()
	state := export.State()
	customerID := export.CustomerID()
	integrationID := export.IntegrationInstanceID()

	sdk.LogInfo(logger, "export started")

	config := export.Config()

	url, creds, err := g.getHTTPCredOpts(logger, config)
	if err != nil {
		return err
	}
	client := g.manager.HTTPManager().New(url, nil)
	ok, concurr := config.GetInt("concurrency")
	if !ok {
		concurr = 10
	}

	workUsermap := map[string]*sdk.WorkUser{}
	sourcecodeUsermap := map[string]*sdk.SourceCodeUser{}
	a := api.New(logger, client, state, pipe, customerID, integrationID, g.refType, concurr, creds)
	if err := a.FetchStatuses(); err != nil {
		return err
	}

	projects, err := a.FetchProjects()
	if err != nil {
		return fmt.Errorf("error fetching projects. err: %v", err)
	}

	for _, proj := range projects {

		g.sendCapabilities(pipe, customerID, integrationID, proj.RefID)
		pipe.Write(proj)

		var updated time.Time
		var strTime string
		if ok, _ := state.Get("updated_"+proj.RefID, &strTime); ok {
			updated, _ = time.Parse(time.RFC3339Nano, strTime)
		}
		repos, err := a.FetchRepos(proj.RefID)
		if err != nil {
			return fmt.Errorf("error fetching repos. err: %v", err)
		}
		for _, r := range repos {
			pipe.Write(r)
			if err := a.FetchPullRequests(proj.RefID, r.RefID, r.Name, updated); err != nil {
				return fmt.Errorf("error fetching pull requests repos. err: %v", err)
			}
		}

		ids, err := a.FetchTeams(proj.RefID)
		if err != nil {
			return fmt.Errorf("error fetching teams. err: %v", err)
		}
		if err := a.FetchUsers(proj.RefID, ids, workUsermap, sourcecodeUsermap); err != nil {
			return fmt.Errorf("error fetching users. err: %v", err)
		}
		if err := a.FetchSprints(proj.RefID, ids); err != nil {
			return fmt.Errorf("error fetching sprints. err: %v", err)
		}
		if err := a.FetchAllIssues(proj.RefID, updated); err != nil {
			return fmt.Errorf("error fetching issues. err: %v", err)
		}
		state.Set("updated_"+proj.RefID, time.Now().Format(time.RFC3339Nano))
	}
	async := sdk.NewAsync(2)
	async.Do(func() error {
		for _, urs := range workUsermap {
			if err := pipe.Write(urs); err != nil {
				return err
			}
		}
		return nil
	})
	async.Do(func() error {
		for _, urs := range sourcecodeUsermap {
			if err := pipe.Write(urs); err != nil {
				return err
			}
		}
		return nil
	})
	err = async.Wait()
	if err == nil {
		sdk.LogInfo(logger, "export finished")
	}
	return err
}

func (g *AzureIntegration) sendCapabilities(pipe sdk.Pipe, customerID, integrationID, projid string) {
	pipe.Write(&sdk.WorkProjectCapability{
		Attachments:           false,
		ChangeLogs:            true,
		CustomerID:            customerID,
		DueDates:              true,
		Epics:                 false,
		InProgressStates:      true,
		IntegrationInstanceID: &integrationID,
		KanbanBoards:          false,
		LinkedIssues:          false,
		Parents:               false,
		Priorities:            true,
		ProjectID:             sdk.NewWorkProjectID(customerID, projid, g.refType),
		RefID:                 projid,
		RefType:               g.refType,
		Resolutions:           true,
		Sprints:               true,
		StoryPoints:           true,
	})
}

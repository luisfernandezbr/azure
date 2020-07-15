package internal

import (
	"errors"
	"fmt"
	"time"

	"github.com/pinpt/agent.next.azure/internal/api"
	"github.com/pinpt/agent.next/sdk"
)

// AzureIntegration is an integration for Azure
type AzureIntegration struct {
	logger     sdk.Logger
	config     sdk.Config
	manager    sdk.Manager
	refType    string
	customerID string
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
	config := instance.Config()
	if config.APIKeyAuth == nil {
		return errors.New("Missing --apikey_auth")
	}
	ok, concurr := config.GetInt("concurrency")
	if !ok {
		concurr = 10
	}
	return g.registerWebHooks(instance, concurr)
}

// Dismiss is called when an existing integration instance is removed
func (g *AzureIntegration) Dismiss(instance sdk.Instance) error {
	config := instance.Config()
	if config.APIKeyAuth == nil {
		return errors.New("Missing --apikey_auth")
	}
	ok, concurr := config.GetInt("concurrency")
	if !ok {
		concurr = 10
	}
	return g.unregisterWebHooks(instance, concurr)
}

// Stop is called when the integration is shutting down for cleanup
func (g *AzureIntegration) Stop() error {
	sdk.LogInfo(g.logger, "stopping")
	return nil
}

// Export is called to tell the integration to run an export
func (g *AzureIntegration) Export(export sdk.Export) error {
	sdk.LogInfo(g.logger, "export started")
	pipe := export.Pipe()
	state := export.State()
	customerID := export.CustomerID()
	integrationID := export.IntegrationInstanceID()

	sdk.LogDebug(g.logger, "export starting")

	config := export.Config()
	// instance := sdk.NewInstance(config, state, pipe, customerID, export.IntegrationInstanceID())
	// if err := g.Enroll(*instance); err != nil {
	// 	return err
	// }
	// os.Exit(1)
	auth := config.APIKeyAuth
	if auth == nil {
		return errors.New("Missing --apikey_auth")
	}
	client := g.manager.HTTPManager().New(auth.URL, nil)
	ok, concurr := config.GetInt("concurrency")
	if !ok {
		concurr = 10
	}

	prsChannel := make(chan *sdk.SourceCodePullRequest, concurr)
	go func() {
		for each := range prsChannel {
			pipe.Write(each)
		}
	}()
	prCommitsChannel := make(chan *sdk.SourceCodePullRequestCommit, concurr)
	go func() {
		for each := range prCommitsChannel {
			pipe.Write(each)
		}
	}()
	prCommentsChannel := make(chan *sdk.SourceCodePullRequestComment, concurr)
	go func() {
		for each := range prCommentsChannel {
			pipe.Write(each)
		}
	}()
	prReviewsChannel := make(chan *sdk.SourceCodePullRequestReview, concurr)
	go func() {
		for each := range prReviewsChannel {
			pipe.Write(each)
		}
	}()

	issueChannel := make(chan *sdk.WorkIssue, concurr)
	go func() {
		for each := range issueChannel {
			pipe.Write(each)
		}
	}()
	issueCommentChannel := make(chan *sdk.WorkIssueComment, concurr)
	go func() {
		for each := range issueCommentChannel {
			pipe.Write(each)
		}
	}()
	sprintChannel := make(chan *sdk.AgileSprint, concurr)
	go func() {
		for each := range sprintChannel {
			pipe.Write(each)
		}
	}()

	workUsermap := map[string]*sdk.WorkUser{}
	sourcecodeUsermap := map[string]*sdk.SourceCodeUser{}
	a := api.New(g.logger, client, state, customerID, integrationID, g.refType, concurr, sdk.WithBasicAuth("", auth.APIKey))
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
			if err := a.FetchPullRequests(proj.RefID, r.RefID, updated, prsChannel, prCommitsChannel, prCommentsChannel, prReviewsChannel); err != nil {
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
		if err := a.FetchSprints(proj.RefID, ids, sprintChannel); err != nil {
			return fmt.Errorf("error fetching sprints. err: %v", err)
		}
		if err := a.FetchAllIssues(proj.RefID, updated, issueChannel, issueCommentChannel); err != nil {
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
		sdk.LogInfo(g.logger, "export finished")
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

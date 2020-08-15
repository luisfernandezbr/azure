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
	instanceID := export.IntegrationInstanceID()

	sdk.LogDebug(g.logger, "export starting")

	config := export.Config()
	var creds sdk.WithHTTPOption
	var authurl string
	if config.APIKeyAuth != nil {
		creds = sdk.WithBasicAuth("", config.APIKeyAuth.APIKey)
		authurl = config.APIKeyAuth.URL
	} else if config.OAuth2Auth != nil {
		creds = sdk.WithOAuth2Refresh(g.manager, g.refType, config.OAuth2Auth.AccessToken, *config.OAuth2Auth.RefreshToken)
		authurl = "https://dev.azure.com/"
	} else {
		return errors.New("missing auth")
	}
	client := g.manager.HTTPManager().New(authurl, nil)
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

	statusesChannel := make(chan *sdk.WorkIssueStatus, concurr)
	go func() {
		for each := range statusesChannel {
			pipe.Write(each)
		}
	}()

	workUsermap := map[string]*sdk.WorkUser{}
	sourcecodeUsermap := map[string]*sdk.SourceCodeUser{}
	a := api.New(g.logger, client, state, customerID, instanceID, g.refType, concurr, creds)
	workconf, err := a.FetchStatuses(statusesChannel)
	if err != nil {
		return err
	}
	pipe.Write(workconf)

	projects, err := a.FetchProjects()
	if err != nil {
		return fmt.Errorf("error fetching projects. err: %v", err)
	}

	for _, proj := range projects {

		g.sendCapabilities(pipe, customerID, instanceID, proj.RefID)
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
			if err := a.FetchPullRequests(proj.RefID, r.RefID, r.Name, updated, prsChannel, prCommitsChannel, prCommentsChannel, prReviewsChannel); err != nil {
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

func (g *AzureIntegration) sendCapabilities(pipe sdk.Pipe, customerID, instanceID, projid string) {
	pipe.Write(&sdk.WorkProjectCapability{
		Attachments:           false,
		ChangeLogs:            true,
		CustomerID:            customerID,
		DueDates:              true,
		Epics:                 false,
		InProgressStates:      true,
		IntegrationInstanceID: &instanceID,
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

type validateObject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatarUrl"`
	TotalCount  int64  `json:"totalCount"`
	Type        string `json:"type"`
	Public      bool   `json:"public"`
}

func (g *AzureIntegration) Validate(validate sdk.Validate) (map[string]interface{}, error) {

	result := make(map[string]interface{})
	customerID := validate.CustomerID()
	instanceID := validate.IntegrationInstanceID()
	refType := validate.RefType()
	config := validate.Config()

	var creds sdk.WithHTTPOption
	var authurl string
	if config.APIKeyAuth != nil {
		creds = sdk.WithBasicAuth("", config.APIKeyAuth.APIKey)
		authurl = config.APIKeyAuth.URL
	} else if config.OAuth2Auth != nil {
		creds = sdk.WithOAuth2Refresh(g.manager, refType, config.OAuth2Auth.AccessToken, *config.OAuth2Auth.RefreshToken)
		authurl = "https://dev.azure.com/"
	} else {
		return nil, errors.New("missing auth")
	}
	client := g.manager.HTTPManager().New(authurl, nil)
	a := api.New(g.logger, client, nil, customerID, instanceID, refType, 10, creds)
	projs, err := a.FetchProjects()
	if err != nil {
		return nil, err
	}
	for _, prj := range projs {
		repos, err := a.FetchRepos(prj.RefID)
		if err != nil {
			return nil, err
		}
		result[prj.RefID] = validateObject{
			ID:          prj.RefID,
			Name:        prj.Name,
			Description: *prj.Description,
			TotalCount:  int64(len(repos)),
			Type:        "ORG",
			Public:      false,
		}
	}
	return result, nil
}

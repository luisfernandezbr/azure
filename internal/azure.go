package internal

import (
	"errors"

	"github.com/pinpt/agent/v4/sdk"
	"github.com/pinpt/azure/internal/api"
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
	if config.APIKeyAuth != nil {
		if config.APIKeyAuth.APIKey == "" {
			return errors.New("Missing --apikey_auth")
		}
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

func (g *AzureIntegration) getHTTPCredOpts(config sdk.Config) (string, sdk.WithHTTPOption, error) {
	if auth := config.APIKeyAuth; auth != nil {
		sdk.LogInfo(g.logger, "using basic auth")
		return auth.URL, sdk.WithBasicAuth("", auth.APIKey), nil
	}
	if auth := config.OAuth2Auth; auth != nil {
		sdk.LogInfo(g.logger, "using oauth2")
		return auth.URL, sdk.WithOAuth2Refresh(
			g.manager, g.refType,
			auth.AccessToken,
			*auth.RefreshToken,
		), nil
	}
	return "", nil, errors.New("missing auth")
}

func (g *AzureIntegration) fetchAccounts(customerID, integrationID string, config sdk.Config) (*sdk.Config, error) {
	url, creds, err := g.getHTTPCredOpts(config)
	if err != nil {
		return nil, err
	}
	ok, concurr := config.GetInt("concurrency")
	if !ok {
		concurr = 10
	}
	client := g.manager.HTTPManager().New(url, nil)
	a := api.New(g.logger, client, nil, nil, customerID, integrationID, g.refType, concurr, creds)
	projects, err := a.FetchProjects()
	if err != nil {
		return nil, err
	}
	var accounts []*sdk.ConfigAccount
	for _, proj := range projects {
		repos, err := a.FetchRepos(proj.RefID)
		if err != nil {
			return nil, err
		}
		count := int64(len(repos))
		accounts = append(accounts, &sdk.ConfigAccount{
			ID:          proj.RefID,
			Name:        &proj.Name,
			Description: proj.Description,
			TotalCount:  &count,
			Type:        sdk.ConfigAccountTypeOrg,
			Public:      proj.Visibility == sdk.WorkProjectVisibilityPublic,
			Selected:    sdk.BoolPointer(true),
		})
	}

	res := sdk.ConfigAccounts{}
	for _, account := range accounts {
		res[account.ID] = account
	}
	config.Accounts = &res
	return &config, nil
}

// AutoConfigure is called when a cloud integration has requested to be auto configured
func (g *AzureIntegration) AutoConfigure(autoconfig sdk.AutoConfigure) (*sdk.Config, error) {
	customerID := autoconfig.CustomerID()
	integrationID := autoconfig.IntegrationInstanceID()
	config := autoconfig.Config()
	return g.fetchAccounts(customerID, integrationID, config)
}

// Stop is called when the integration is shutting down for cleanup
func (g *AzureIntegration) Stop() error {
	sdk.LogInfo(g.logger, "stopping")
	return nil
}

func (g *AzureIntegration) Validate(validate sdk.Validate) (result map[string]interface{}, err error) {

	customerID := validate.CustomerID()
	integrationID := validate.IntegrationInstanceID()
	config := validate.Config()

	conf, err := g.fetchAccounts(customerID, integrationID, config)
	if err != nil {
		return nil, err
	}
	var accts []*sdk.ConfigAccount
	for _, each := range *conf.Accounts {
		accts = append(accts, each)
	}

	return map[string]interface{}{
		"accounts": accts,
	}, nil

}

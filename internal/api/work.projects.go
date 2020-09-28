package api

import (
	"net/url"
	"sync"

	"github.com/pinpt/agent/sdk"
)

// map[project]process
var projectProcessMap = map[string]string{}

// FetchProjects gets the projects and sends them to the projchan channel
func (a *API) FetchProjects() ([]*sdk.WorkProject, error) {

	sdk.LogInfo(a.logger, "fetching initial projects")
	mu := sync.Mutex{}
	endpoint := "_apis/projects"
	params := url.Values{}
	params.Set("stateFilter", "all")
	var out struct {
		Value []projectResponse `json:"value"`
	}
	if _, err := a.get(endpoint, params, &out); err != nil {
		return nil, err
	}
	async := sdk.NewAsync(a.concurrency)
	var projects []*sdk.WorkProject
	for _, proj := range out.Value {
		projects = append(projects, &sdk.WorkProject{
			Active:                proj.State == "wellFormed",
			CustomerID:            a.customerID,
			Description:           &proj.Description,
			IntegrationInstanceID: &a.integrationID,
			Identifier:            proj.Name,
			Name:                  proj.Name,
			RefID:                 proj.ID,
			RefType:               a.refType,
			URL:                   proj.URL,
		})
		projid := proj.ID
		async.Do(func() error {
			params := url.Values{}
			params.Set("api-version", "5.1-preview.1")
			var out projectDetailResponse
			if _, err := a.get("_apis/projects/"+projid, params, &out); err != nil {
				return err
			}
			mu.Lock()
			projectProcessMap[projid] = out.Capabilities.ProcessTemplate.TemplateTypeID
			mu.Unlock()
			return nil
		})
	}
	if err := async.Wait(); err != nil {
		return nil, err
	}
	return projects, nil
}

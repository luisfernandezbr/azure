package api

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

// FetchProjects gets the projects and sends them to the projchan channel
func (a *API) FetchProjects() ([]*sdk.WorkProject, error) {

	sdk.LogInfo(a.logger, "fetching initial projects")

	endpoint := "_apis/projects"
	params := url.Values{}
	params.Set("stateFilter", "all")
	var out struct {
		Value []projectResponse `json:"value"`
	}
	if _, err := a.get(endpoint, params, &out); err != nil {
		return nil, err
	}
	var projects []*sdk.WorkProject
	for _, proj := range out.Value {
		projects = append(projects, &sdk.WorkProject{
			Active:      proj.State == "wellFormed",
			CustomerID:  a.customerID,
			Description: &proj.Description,
			Identifier:  proj.Name,
			Name:        proj.Name,
			RefID:       fmt.Sprint(proj.ID),
			RefType:     a.refType,
			URL:         proj.URL,
		})
	}
	return projects, nil
}

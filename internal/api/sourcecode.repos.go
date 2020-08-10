package api

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/agent.next/sdk"
)

// FetchRepos gets the repos from a project
func (a *API) FetchRepos(projid string) ([]*sdk.SourceCodeRepo, error) {

	sdk.LogInfo(a.logger, "fetching repos", "project_id", projid)

	endpoint := fmt.Sprintf(`%s/_apis/git/repositories`, url.PathEscape(projid))
	var out struct {
		Value []reposResponse `json:"value"`
	}
	_, err := a.get(endpoint, nil, &out)
	if err != nil {
		return nil, err
	}

	var allRepos []*sdk.SourceCodeRepo
	for _, repo := range out.Value {
		var reponame string
		if strings.HasPrefix(repo.Name, repo.Project.Name) {
			reponame = repo.Name
		} else {
			reponame = repo.Project.Name + "/" + repo.Name
		}
		allRepos = append(allRepos, &sdk.SourceCodeRepo{
			Active:                true,
			CustomerID:            a.customerID,
			DefaultBranch:         strings.TrimPrefix(repo.DefaultBranch, "refs/heads/"),
			IntegrationInstanceID: &a.integrationID,
			Name:                  reponame,
			RefID:                 repo.ID,
			RefType:               a.refType,
			URL:                   repo.RemoteURL,
		})
	}

	return allRepos, nil
}

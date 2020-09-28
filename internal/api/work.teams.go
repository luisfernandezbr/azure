package api

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent/v4/sdk"
)

func (a *API) FetchTeams(projid string) ([]string, error) {

	sdk.LogInfo(a.logger, "fetching teams", "project_id", projid)

	endpoint := fmt.Sprintf(`_apis/projects/%s/teams`, url.PathEscape(projid))
	var out struct {
		Value []teamsResponse `json:"value"`
	}
	_, err := a.get(endpoint, nil, &out)
	if err != nil {
		return nil, err
	}
	var res []string
	for _, team := range out.Value {
		res = append(res, team.ID)
	}
	return res, nil
}

package api

import (
	"fmt"
	"net/url"
)

func (a *API) FetchTeams(projid string) ([]string, error) {

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

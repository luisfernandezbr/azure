package api

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/pinpt/agent.next/sdk"
)

var doubleSlashRegex = regexp.MustCompile(`^(.*?)\\\\`)

// FetchUsers gets all users from all the teams from a single project
func (a *API) FetchUsers(projid string, teamids []string, usermap map[string]*sdk.WorkUser) error {
	rawusers, err := a.fetchAllUsers(projid, teamids)
	if err != nil {
		return err
	}
	for _, u := range rawusers {
		usermap[u.ID] = &sdk.WorkUser{
			AvatarURL:  sdk.StringPointer(u.ImageURL),
			CustomerID: a.customerID,
			Name:       doubleSlashRegex.ReplaceAllString(u.DisplayName, ""),
			RefID:      u.ID,
			RefType:    a.refType,
			Username:   doubleSlashRegex.ReplaceAllString(u.UniqueName, ""),
		}
	}
	return nil
}

func (a *API) fetchAllUsers(projid string, teamids []string) ([]usersResponse, error) {
	usersmap := make(map[string]usersResponse)
	for _, teamid := range teamids {
		users, err := a.fetchUsers(projid, teamid)
		if err != nil {
			return nil, nil
		}
		for _, u := range users {
			usersmap[u.ID] = u
		}
	}
	var users []usersResponse
	for _, u := range usersmap {
		users = append(users, u)
	}
	return users, nil
}

func (a *API) fetchUsers(projid string, teamid string) ([]usersResponse, error) {

	endpoint := fmt.Sprintf(`_apis/projects/%s/teams/%s/members`, url.PathEscape(projid), url.PathEscape(teamid))
	var out struct {
		Value []usersResponseAzure `json:"value"`
	}
	if _, err := a.get(endpoint, nil, &out); err != nil {
		return nil, err
	}
	var users []usersResponse
	for _, r := range out.Value {
		users = append(users, r.Identity)
	}
	return users, nil
}

package api

import (
	"errors"

	"github.com/pinpt/agent.next/sdk"
)

type issueProjectRefs struct {
	IssueID   int    `json:"issue_id"`
	ProjectID string `json:"project_id"`
}
type pullRequestRepoProjectRefs struct {
	ProjectID     string `json:"project_id"`
	RepoID        string `json:"repo_id"`
	PullRequestID int    `json:"pull_request_id"`
}

func (a *API) createIssueID(projid string, issueid int) string {
	newrefid := sdk.Hash(a.customerID, projid, issueid)
	if !a.state.Exists(newrefid) {
		a.state.Set(newrefid, issueProjectRefs{issueid, projid})
	}
	return newrefid
}

func (a *API) createPullRequestID(projid string, repoid string, prid int) string {
	newrefid := sdk.Hash(a.customerID, projid, repoid, prid)
	if !a.state.Exists(newrefid) {
		a.state.Set(newrefid, pullRequestRepoProjectRefs{projid, repoid, prid})
	}
	return newrefid
}

// FetchPullRequestRepoProjectRefs gets the project_ref_id and the issue_ref_id from a hashed ref_id
func (a *API) FetchPullRequestRepoProjectRefs(hashed string) (projectID string, repoID string, prID int, _ error) {
	var ref pullRequestRepoProjectRefs
	exists, err := a.state.Get(hashed, &ref)
	if err != nil {
		return "", "", 0, err
	}
	if exists {
		return ref.ProjectID, ref.RepoID, ref.PullRequestID, nil
	}
	return "", "", 0, errors.New("project id, repo id, and pr id not found")
}

// FetchIssueProjectRefs gets the project_ref_id and the issue_ref_id from a hashed ref_id
func (a *API) FetchIssueProjectRefs(hashed string) (projectID string, issueID int, _ error) {
	var ref issueProjectRefs
	exists, err := a.state.Get(hashed, &ref)
	if err != nil {
		return "", 0, err
	}
	if exists {
		return ref.ProjectID, ref.IssueID, nil
	}
	return "", 0, errors.New("project id and issue id not found")
}

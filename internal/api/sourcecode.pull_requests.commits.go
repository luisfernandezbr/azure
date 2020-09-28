package api

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent/sdk"
)

func (a *API) sendPullRequestCommit(projid string, repoRefID string, p PullRequestResponseWithShas) error {
	sha := p.commitSHAs[len(p.commitSHAs)-1]
	endpoint := fmt.Sprintf(`_apis/git/repositories/%s/commits/%s`, url.PathEscape(p.Repository.ID), url.PathEscape(sha))
	var out singleCommitResponse
	params := url.Values{}
	params.Set("changeCount", "1")

	if _, err := a.get(endpoint, params, &out); err != nil {
		return err
	}
	prrefid := a.createPullRequestID(projid, repoRefID, p.PullRequestID)
	commit := &sdk.SourceCodePullRequestCommit{
		Active:                true,
		Additions:             out.ChangeCounts.Add,
		AuthorRefID:           out.Push.PushedBy.ID,
		BranchID:              sdk.NewSourceCodeBranchID(a.customerID, repoRefID, a.refType, p.SourceBranch, p.commitSHAs[0]),
		CommitterRefID:        out.Push.PushedBy.ID,
		CustomerID:            a.customerID,
		Deletions:             out.ChangeCounts.Delete,
		IntegrationInstanceID: &a.integrationID,
		Message:               out.Comment,
		PullRequestID:         sdk.NewSourceCodePullRequestID(a.customerID, prrefid, a.refType, repoRefID),
		RefID:                 sha,
		RefType:               a.refType,
		RepoID:                sdk.NewSourceCodeRepoID(a.customerID, repoRefID, a.refType),
		Sha:                   sha,
		URL:                   out.RemoteURL,
	}
	sdk.ConvertTimeToDateModel(out.Push.Date, &commit.CreatedDate)
	return a.pipe.Write(commit)
}

func (a *API) sendPullRequestCommits(projid string, reponame string, pr PullRequestResponseWithShas) error {
	endpoint := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/commits`, url.PathEscape(pr.Repository.ID), pr.PullRequestID)
	var out struct {
		Value []commitsResponseLight `json:"value"`
	}
	params := url.Values{}
	params.Set("$top", "10000") // there is no pagination, so make this number BIG
	if _, err := a.get(endpoint, params, &out); err != nil {
		return err
	}
	if len(out.Value) > 0 { // pr without commits? this should never be 0
		for _, commit := range out.Value {
			pr.commitSHAs = append(pr.commitSHAs, commit.CommitID)
			pr := pr
			if err := a.sendPullRequestCommit(projid, pr.Repository.ID, pr); err != nil {
				return err
			}
		}
		return a.sendPullRequest(projid, reponame, pr.Repository.ID, pr)
	}

	return nil
}

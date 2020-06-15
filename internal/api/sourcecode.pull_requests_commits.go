package api

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

func (a *API) sendPullRequestCommit(repoRefID string, p pullRequestResponseWithShas, prCommitsChannel chan<- *sdk.SourceCodePullRequestCommit) error {
	sha := p.commitSHAs[len(p.commitSHAs)-1]
	c, err := a.fetchSingleCommit(p.Repository.ID, sha)
	if err != nil {
		return err
	}
	commit := &sdk.SourceCodePullRequestCommit{
		Additions: c.ChangeCounts.Add,
		// AuthorRefID: not provided

		BranchID: sdk.NewSourceCodeBranchID(a.customerID, repoRefID, a.refType, p.SourceBranch, p.commitSHAs[0]),
		// CommitterRefID: not provided
		CustomerID:    a.customerID,
		Deletions:     c.ChangeCounts.Delete,
		Message:       c.Comment,
		PullRequestID: sdk.NewSourceCodePullRequestID(a.customerID, fmt.Sprint(p.PullRequestID), a.refType, repoRefID),
		RefID:         sha,
		RefType:       a.refType,
		RepoID:        repoRefID,
		Sha:           sha,
		URL:           c.RemoteURL,
	}
	sdk.ConvertTimeToDateModel(c.Push.Date, &commit.CreatedDate)
	prCommitsChannel <- commit
	return nil
}

func (a *API) fetchPullRequestCommits(repoid string, prid int64) ([]commitsResponseLight, error) {
	endpoint := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/commits`, url.PathEscape(repoid), prid)
	var out struct {
		Value []commitsResponseLight `json:"value"`
	}
	params := url.Values{}
	params.Set("$top", "1000")
	_, err := a.get(endpoint, params, &out)
	return out.Value, err
}

func (a *API) fetchSingleCommit(repoid string, commitid string) (singleCommitResponse, error) {
	endpoint := fmt.Sprintf(`_apis/git/repositories/%s/commits/%s`, url.PathEscape(repoid), url.PathEscape(commitid))
	var out singleCommitResponse
	params := url.Values{}
	params.Set("$changeCount", "1000")
	_, err := a.get(endpoint, params, &out)
	return out, err
}

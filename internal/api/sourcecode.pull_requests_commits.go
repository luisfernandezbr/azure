package api

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

func (a *API) sendPullRequestCommit(repoRefID string, p pullRequestResponseWithShas, prCommitsChannel chan<- *sdk.SourceCodePullRequestCommit) error {
	sha := p.commitSHAs[len(p.commitSHAs)-1]
	endpoint := fmt.Sprintf(`_apis/git/repositories/%s/commits/%s`, url.PathEscape(p.Repository.ID), url.PathEscape(sha))
	var out singleCommitResponse
	params := url.Values{}
	params.Set("$changeCount", "1000")
	if _, err := a.get(endpoint, params, &out); err != nil {
		return err
	}
	commit := &sdk.SourceCodePullRequestCommit{
		Additions: out.ChangeCounts.Add,
		// AuthorRefID: not provided

		BranchID: sdk.NewSourceCodeBranchID(a.customerID, repoRefID, a.refType, p.SourceBranch, p.commitSHAs[0]),
		// CommitterRefID: not provided
		CustomerID:    a.customerID,
		Deletions:     out.ChangeCounts.Delete,
		Message:       out.Comment,
		PullRequestID: sdk.NewSourceCodePullRequestID(a.customerID, fmt.Sprint(p.PullRequestID), a.refType, repoRefID),
		RefID:         sha,
		RefType:       a.refType,
		RepoID:        repoRefID,
		Sha:           sha,
		URL:           out.RemoteURL,
	}
	sdk.ConvertTimeToDateModel(out.Push.Date, &commit.CreatedDate)
	prCommitsChannel <- commit
	return nil
}

func (a *API) sendPullRequestCommits(pr pullRequestResponseWithShas, prsChannel chan<- *sdk.SourceCodePullRequest, prCommitsChannel chan<- *sdk.SourceCodePullRequestCommit) error {
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
			if err := a.sendPullRequestCommit(pr.Repository.ID, pr, prCommitsChannel); err != nil {
				return err
			}
		}
		a.sendPullRequest(pr.Repository.ID, pr, prsChannel)
	}

	return nil
}

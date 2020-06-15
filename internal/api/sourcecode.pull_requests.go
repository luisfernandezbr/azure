package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

// FetchPullRequests calls the pull request api and processes the reponse sending each object to the corresponding channel async
// sdk.SourceCodePullRequest, sdk.SourceCodePullRequestReview, sdk.SourceCodePullRequestComment, and sdk.SourceCodePullRequestCommit
func (a *API) FetchPullRequests(projid string, repoid string, updated time.Time,
	prsChannel chan<- *sdk.SourceCodePullRequest,
	prCommitsChannel chan<- *sdk.SourceCodePullRequestCommit,
	prCommentsChannel chan<- *sdk.SourceCodePullRequestComment,
	prReviewsChannel chan<- *sdk.SourceCodePullRequestReview,
) error {
	sdk.LogInfo(a.logger, "fetching pull requests", "project_id", projid, "repo_id", repoid)

	endpoint := fmt.Sprintf(`%s/_apis/git/repositories/%s/pullrequests`, url.PathEscape(projid), url.PathEscape(repoid))
	var out struct {
		Value []pullRequestResponse `json:"value"`
	}
	params := url.Values{}
	params.Set("$top", "1000")
	params.Set("status", "all")
	_, err := a.get(endpoint, params, &out)
	if err != nil {
		return err
	}
	historical := updated.IsZero()
	var pullrequests []pullRequestResponse
	var pullrequestcomments []pullRequestResponse
	for _, p := range out.Value {

		// modify the url to show the ui instead of api call
		p.URL = strings.ToLower(p.URL)
		p.URL = strings.Replace(p.URL, "_apis/git/repositories", "_git", 1)
		p.URL = strings.Replace(p.URL, "/pullrequests/", "/pullrequest/", 1)

		if historical {
			pullrequests = append(pullrequests, p)
			pullrequestcomments = append(pullrequestcomments, p)
		} else if p.Status == "active" || p.CreationDate.After(updated) {
			//  only fetch the comments if this pr is still opened or was closed after the last processed date
			pullrequestcomments = append(pullrequestcomments, p)
		}

	}

	async := NewAsync(10)
	for _, p := range pullrequests {
		pr := pullRequestResponseWithShas{}
		pr.pullRequestResponse = p
		async.Do(func() error {
			pr.SourceBranch = strings.TrimPrefix(p.SourceBranch, "refs/heads/")
			pr.TargetBranch = strings.TrimPrefix(p.TargetBranch, "refs/heads/")
			commits, err := a.fetchPullRequestCommits(pr.Repository.ID, pr.PullRequestID)
			if err != nil {
				return fmt.Errorf("error fetching commits for PR, skipping pr_id:%v repo_id:%v err:%v", pr.PullRequestID, pr.Repository.ID, err)
			}
			// pr without commits? this should never be 0
			if len(commits) > 0 {
				for _, commit := range commits {
					pr.commitSHAs = append(pr.commitSHAs, commit.CommitID)
					pr := pr
					a.sendPullRequestCommit(repoid, pr, prCommitsChannel)
				}
				a.sendPullRequest(repoid, pr, prsChannel)
			}
			return nil
		})
	}
	if err = async.Wait(); err != nil {
		return err
	}

	async = NewAsync(10)
	for _, p := range pullrequestcomments {
		pr := p
		async.Do(func() error {
			return a.sendPullRequestComment(repoid, pr, prCommentsChannel, prReviewsChannel)
		})
	}
	err = async.Wait()
	return err

}

func (a *API) sendPullRequest(repoRefID string, p pullRequestResponseWithShas, prsChannel chan<- *sdk.SourceCodePullRequest) {

	pr := &sdk.SourceCodePullRequest{
		BranchName:     p.SourceBranch,
		CreatedByRefID: p.CreatedBy.ID,
		CustomerID:     a.customerID,
		Description:    p.Description,
		RefID:          fmt.Sprintf("%d", p.PullRequestID),
		RefType:        a.refType,
		RepoID:         repoRefID,
		Title:          p.Title,
		URL:            p.URL,
		CommitShas:     p.commitSHAs,
		Identifier:     fmt.Sprintf("#%d", p.PullRequestID), // format for displaying the PR in app
	}
	if p.commitSHAs != nil {
		pr.BranchID = sdk.NewSourceCodeBranchID(a.customerID, repoRefID, a.refType, p.SourceBranch, p.commitSHAs[0])
		for _, sha := range p.commitSHAs {
			pr.CommitIds = append(pr.CommitIds, sdk.NewSourceCodeCommitID(a.customerID, sha, a.refType, repoRefID))
		}
	}
	sdk.ConvertTimeToDateModel(p.ClosedDate, &pr.ClosedDate)
	sdk.ConvertTimeToDateModel(p.CreationDate, &pr.CreatedDate)

	switch p.Status {
	case "completed":
		pr.Status = sdk.SourceCodePullRequestStatusMerged
		pr.MergeSha = p.LastMergeCommit.CommidID
		pr.MergeCommitID = sdk.NewSourceCodeCommitID(a.customerID, pr.MergeSha, a.refType, repoRefID)
		sdk.ConvertTimeToDateModel(p.CompletionQueueTime, &pr.MergedDate)
	case "active":
		pr.Status = sdk.SourceCodePullRequestStatusOpen
	case "abandoned":
		pr.Status = sdk.SourceCodePullRequestStatusClosed
	}
	for _, r := range p.Reviewers {
		switch r.Vote {
		case -10:
			pr.ClosedByRefID = r.ID
		case 10:
			if pr.ClosedByRefID == "" {
				pr.ClosedByRefID = r.ID
			}
			pr.MergedByRefID = r.ID
		}
	}
	prsChannel <- pr
}

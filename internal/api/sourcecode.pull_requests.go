package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pinpt/agent/sdk"
)

// FetchPullRequests calls the pull request api and processes the reponse writing each object to the pipeline
func (a *API) FetchPullRequests(projid string, repoid string, reponame string, updated time.Time) error {
	sdk.LogInfo(a.logger, "fetching pull requests", "project_id", projid, "repo_id", repoid)

	endpoint := fmt.Sprintf(`%s/_apis/git/repositories/%s/pullrequests`, url.PathEscape(projid), url.PathEscape(repoid))

	params := url.Values{}
	params.Set("$top", "1000")
	params.Set("status", "all")
	// ===========================================
	out := make(chan objects, 1)
	errochan := make(chan error, 1)
	go func() {
		for object := range out {
			value := []PullRequestResponse{}
			if err := object.Unmarshal(&value); err != nil {
				errochan <- err
			}
			err := a.ProcessPullRequests(value, projid, repoid, reponame, updated)
			if err != nil {
				errochan <- err
				return
			}
		}
		errochan <- nil
	}()
	// ===========================================
	func() {
		if err := a.paginate(endpoint, params, out); err != nil {
			errochan <- err
		}
	}()
	return <-errochan
}

// UpdatePullRequest updates a PR, the fields supported are title and description
func (a *API) UpdatePullRequest(refid string, v *sdk.SourcecodePullRequestUpdateMutation) error {
	projid, repoid, prid, err := a.FetchPullRequestRepoProjectRefs(refid)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf(`%s/_apis/git/repositories/%s/pullrequests/%s`,
		url.PathEscape(projid),
		url.PathEscape(repoid),
		url.PathEscape(fmt.Sprint(prid)))
	var payload struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	payload.Title = *v.Set.Title
	payload.Description = *v.Set.Description
	var out interface{}
	if _, err := a.patch(endpoint, payload, nil, &out); err != nil {
		return err
	}
	return nil
}

func (a *API) ProcessPullRequests(value []PullRequestResponse, projid string, repoid string, reponame string, updated time.Time) error {

	historical := updated.IsZero()
	var pullrequests []PullRequestResponse
	var pullrequestcomments []PullRequestResponse

	for _, p := range value {
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

	// =================== Commits ===================
	async := sdk.NewAsync(a.concurrency)
	for _, p := range pullrequests {
		pr := PullRequestResponseWithShas{}
		pr.PullRequestResponse = p
		async.Do(func() error {
			pr.SourceBranch = strings.TrimPrefix(p.SourceBranch, "refs/heads/")
			pr.TargetBranch = strings.TrimPrefix(p.TargetBranch, "refs/heads/")
			if err := a.sendPullRequestCommits(projid, reponame, pr); err != nil {
				return fmt.Errorf("error fetching commits for PR, skipping pr_id:%v repo_id:%v err:%v", pr.PullRequestID, pr.Repository.ID, err)
			}
			return nil
		})
	}
	if err := async.Wait(); err != nil {
		return err
	}

	// =================== Comments ===================
	async = sdk.NewAsync(a.concurrency)
	for _, p := range pullrequestcomments {
		pr := p
		async.Do(func() error {
			return a.sendPullRequestComment(projid, repoid, pr)
		})
	}
	return async.Wait()
}

func (a *API) sendPullRequest(projid string, reponame string, repoRefID string, p PullRequestResponseWithShas) error {
	prrefid := a.createPullRequestID(projid, repoRefID, p.PullRequestID)
	pr := &sdk.SourceCodePullRequest{
		Active:                true,
		BranchName:            p.SourceBranch,
		CreatedByRefID:        p.CreatedBy.ID,
		CustomerID:            a.customerID,
		Description:           `<div class="source-azure">` + p.Description + "</div>",
		IntegrationInstanceID: &a.integrationID,
		RefID:                 prrefid,
		RefType:               a.refType,
		RepoID:                sdk.NewSourceCodeRepoID(a.customerID, repoRefID, a.refType),
		Title:                 p.Title,
		URL:                   p.URL,
		CommitShas:            p.commitSHAs,
		Identifier:            fmt.Sprintf("%s#%d", reponame, p.PullRequestID),
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

	pr.Labels = make([]string, 0)
	for _, lbl := range p.Labels {
		pr.Labels = append(pr.Labels, lbl.Name)
	}
	return a.pipe.Write(pr)
}

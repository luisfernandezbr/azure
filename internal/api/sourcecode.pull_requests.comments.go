package api

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/pinpt/agent.next/sdk"
)

var pullRequestCommentVotedReg = regexp.MustCompile(`(.+?)( voted )(-10|-5|0|5|10.*)`)

func (a *API) sendPullRequestComment(projid string, repoRefID string, pr pullRequestResponse) error {

	endpoint := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/threads`, url.PathEscape(pr.Repository.ID), pr.PullRequestID)
	var out struct {
		Value []threadsReponse `json:"value"`
	}
	if _, err := a.get(endpoint, nil, &out); err != nil {
		return fmt.Errorf("error fetching threads for PR, skipping. pr_id: %v. repo_id: %v. err: %v", pr.PullRequestID, pr.Repository.ID, err)
	}
	prrefid := a.createPullRequestID(projid, repoRefID, pr.PullRequestID)
	for _, thread := range out.Value {
		for _, comment := range thread.Comments {
			// comment type "text" means it's a real user instead of system
			if comment.CommentType == "text" {
				refid := fmt.Sprintf("%d_%d", thread.ID, comment.ID)

				c := &sdk.SourceCodePullRequestComment{
					Active:                true,
					Body:                  comment.Content,
					CustomerID:            a.customerID,
					IntegrationInstanceID: &a.integrationID,
					PullRequestID:         sdk.NewSourceCodePullRequestID(a.customerID, a.refType, prrefid, repoRefID),
					RefID:                 refid,
					RefType:               a.refType,
					RepoID:                sdk.NewSourceCodeRepoID(a.customerID, repoRefID, a.refType),
					UserRefID:             comment.Author.ID,
				}
				sdk.ConvertTimeToDateModel(comment.PublishedDate, &c.CreatedDate)
				sdk.ConvertTimeToDateModel(comment.LastUpdatedDate, &c.UpdatedDate)
				if err := a.pipe.Write(c); err != nil {
					return err
				}

				reviewer := &sdk.SourceCodePullRequestReviewRequest{
					Active:                 true,
					CustomerID:             a.customerID,
					IntegrationInstanceID:  &a.integrationID,
					PullRequestID:          c.PullRequestID,
					RefID:                  refid,
					RefType:                a.refType,
					RepoID:                 c.RepoID,
					RequestedReviewerRefID: comment.Author.ID,
					SenderRefID:            pr.CreatedBy.ID,
					URL:                    pr.URL,
				}
				sdk.ConvertTimeToDateModel(comment.PublishedDate, &reviewer.CreatedDate)
				if err := a.pipe.Write(reviewer); err != nil {
					return err
				}

				continue
			}

			if comment.CommentType == "system" {
				if found := pullRequestCommentVotedReg.FindAllStringSubmatch(comment.Content, -1); len(found) > 0 {
					vote := found[0][3]
					var state sdk.SourceCodePullRequestReviewState
					switch vote {
					case "-10":
						state = sdk.SourceCodePullRequestReviewStateDismissed
					case "-5":
						state = sdk.SourceCodePullRequestReviewStateChangesRequested
					case "0":
						state = sdk.SourceCodePullRequestReviewStatePending
					case "5":
						state = sdk.SourceCodePullRequestReviewStateCommented
					case "10":
						state = sdk.SourceCodePullRequestReviewStateApproved
					}
					refid := sdk.Hash(pr.PullRequestID, thread.ID, comment.ID)
					review := &sdk.SourceCodePullRequestReview{
						Active:                true,
						CustomerID:            a.customerID,
						IntegrationInstanceID: &a.integrationID,
						PullRequestID:         sdk.NewSourceCodePullRequestID(a.customerID, prrefid, a.refType, repoRefID),
						RefID:                 refid,
						RefType:               a.refType,
						RepoID:                sdk.NewSourceCodeRepoID(a.customerID, repoRefID, a.refType),
						State:                 state,
						URL:                   pr.URL,
						UserRefID:             thread.Identities["1"].ID,
					}
					sdk.ConvertTimeToDateModel(comment.PublishedDate, &review.CreatedDate)
					if err := a.pipe.Write(review); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

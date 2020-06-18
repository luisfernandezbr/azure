package api

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/pinpt/agent.next/sdk"
)

var pullRequestCommentVotedReg = regexp.MustCompile(`(.+?)( voted )(-10|-5|0|5|10.*)`)

func (a *API) sendPullRequestComment(repoRefID string, pr pullRequestResponse, prCommentsChannel chan<- *sdk.SourceCodePullRequestComment, prReviewsChannel chan<- *sdk.SourceCodePullRequestReview) error {

	endpoint := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/threads`, url.PathEscape(pr.Repository.ID), pr.PullRequestID)
	var out struct {
		Value []threadsReponse `json:"value"`
	}
	if _, err := a.get(endpoint, nil, &out); err != nil {
		return fmt.Errorf("error fetching threads for PR, skipping. pr_id: %v. repo_id: %v. err: %v", pr.PullRequestID, pr.Repository.ID, err)
	}

	for _, thread := range out.Value {
		for _, comment := range thread.Comments {
			// comment type "text" means it's a real user instead of system
			if comment.CommentType == "text" {
				refid := fmt.Sprintf("%d_%d", thread.ID, comment.ID)
				c := &sdk.SourceCodePullRequestComment{
					Body:          comment.Content,
					CustomerID:    a.customerID,
					PullRequestID: sdk.NewSourceCodePullRequestID(a.customerID, a.refType, fmt.Sprint(pr.PullRequestID), repoRefID),
					RefID:         refid,
					RefType:       a.refType,
					RepoID:        repoRefID,
					UserRefID:     comment.Author.ID,
				}
				sdk.ConvertTimeToDateModel(comment.PublishedDate, &c.CreatedDate)
				sdk.ConvertTimeToDateModel(comment.LastUpdatedDate, &c.UpdatedDate)
				prCommentsChannel <- c
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
						CustomerID:    a.customerID,
						PullRequestID: sdk.NewSourceCodePullRequestID(a.customerID, fmt.Sprintf("%d", pr.PullRequestID), a.refType, repoRefID),
						RefID:         refid,
						RefType:       a.refType,
						RepoID:        repoRefID,
						State:         state,
						URL:           pr.URL,
						UserRefID:     thread.Identities["1"].ID,
					}
					sdk.ConvertTimeToDateModel(comment.PublishedDate, &review.CreatedDate)
					prReviewsChannel <- review
				}
			}
		}
	}
	return nil
}

package api

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

func (a *API) fetchComments(projid, issueid string, issueCommentChannel chan<- *sdk.WorkIssueComment) error {

	endpoint := fmt.Sprintf("_apis/wit/workItems/%s/comments", url.PathEscape(issueid))
	params := url.Values{}
	params.Set("$top", "200")
	params.Set("api-version", "5.1-preview")

	out := make(chan objects)
	errochan := make(chan error)
	go func() {
		for object := range out {
			var value []issueCommentReponse
			if err := object.Unmarshal(&value); err != nil {
				errochan <- err
				return
			}
			for _, raw := range value {
				comment := &sdk.WorkIssueComment{
					Body:                  raw.Text,
					CustomerID:            a.customerID,
					IntegrationInstanceID: &a.integrationID,
					IssueID:               sdk.NewWorkIssueID(a.customerID, issueid, a.refType),
					ProjectID:             sdk.NewWorkProjectID(a.customerID, projid, a.refType),
					RefID:                 fmt.Sprint(raw.ID),
					RefType:               a.refType,
					URL:                   raw.URL,
					UserRefID:             raw.CreatedBy.ID,
				}
				sdk.ConvertTimeToDateModel(raw.CreatedDate, &comment.CreatedDate)
				sdk.ConvertTimeToDateModel(raw.ModifiedDate, &comment.UpdatedDate)
				issueCommentChannel <- comment
			}
		}
		errochan <- nil
	}()
	// ===========================================
	go func() {
		err := a.paginate(sdk.JoinURL(projid, endpoint), params, out)
		if err != nil {
			errochan <- err
		}
	}()
	return <-errochan

}

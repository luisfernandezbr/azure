package api

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

const whereDateFormat = `01/02/2006 15:04:05Z`

func (a *API) FetchIssues(projid string, updated time.Time) ([]*sdk.WorkIssue, error) {

	var q struct {
		Query string `json:"query"`
	}
	q.Query = `Select System.ID From WorkItems`
	if !updated.IsZero() {
		q.Query += fmt.Sprintf(` WHERE System.ChangedDate > '%s'`, updated.Format(whereDateFormat))
	}
	params := url.Values{}
	params.Set("timePrecision", "true")

	var out workItemsResponse
	if _, err := a.post(sdk.JoinURL(projid, "_apis/wit/wiql"), q, params, &out); err != nil {
		return nil, nil
	}

	var res []*sdk.WorkIssue
	var items []string
	for i, item := range out.WorkItems {
		if i != 0 && (i%200) == 0 {
			issues, err := a.fetchIssues(projid, items)
			if err != nil {
				return nil, err
			}
			if issues != nil {
				res = append(res, issues...)
			}
			items = []string{}
		}
		items = append(items, fmt.Sprint(item.ID))
	}
	issues, err := a.fetchIssues(projid, items)
	if err != nil {
		return nil, err
	}
	if issues != nil {
		res = append(res, issues...)
	}
	return res, nil
}

type linksResponse struct {
	HTML struct {
		HREF string `json:"href"`
	} `json:"html"`
	// there are more here, fields, self, workItemComments, workItemRevisions, workItemType, and workItemUpdates
}

type fieldsResponse struct {
	AssignedTo     usersResponse `json:"System.AssignedTo"`
	ChangedDate    time.Time     `json:"System.ChangedDate"`
	CreatedDate    time.Time     `json:"System.CreatedDate"`
	CreatedBy      usersResponse `json:"System.CreatedBy"`
	Description    string        `json:"System.Description"`
	DueDate        time.Time     `json:"Microsoft.VSTS.Scheduling.DueDate"` // ??
	IterationPath  string        `json:"System.IterationPath"`
	TeamProject    string        `json:"System.TeamProject"`
	Priority       int           `json:"Microsoft.VSTS.Common.Priority"`
	Reason         string        `json:"System.Reason"`
	ResolvedReason string        `json:"Microsoft.VSTS.Common.ResolvedReason"`
	ResolvedDate   time.Time     `json:"Microsoft.VSTS.Common.ResolvedDate"`
	StoryPoints    float64       `json:"Microsoft.VSTS.Scheduling.StoryPoints"`
	State          string        `json:"System.State"`
	Tags           string        `json:"System.Tags"`
	Title          string        `json:"System.Title"`
	WorkItemType   string        `json:"System.WorkItemType"`
}

func stringEquals(str string, vals ...string) bool {
	for _, v := range vals {
		if str == v {
			return true
		}
	}
	return false
}

func (a *API) fetchIssues(projid string, ids []string) ([]*sdk.WorkIssue, error) {

	if len(ids) == 0 {
		return nil, nil
	}
	fmt.Println("fetching issues from", ids[0], "to", ids[len(ids)-1])
	params := url.Values{}
	params.Set("ids", strings.Join(ids, ","))
	params.Set("$expand", "all")

	endpoint := "_apis/wit/workitems"

	var out struct {
		Value []workItemResponse `json:"value"`
	}
	_, err := a.get(sdk.JoinURL(projid, endpoint), params, &out)
	if err != nil {
		return nil, err
	}
	async := NewAsync(10)
	var mutex sync.Mutex
	var res []*sdk.WorkIssue
	for _, itm := range out.Value {
		// copy the value to a new variable so that it's inside this scope
		item := itm
		async.Do(func() error {

			fields := item.Fields
			// skip these
			if stringEquals(fields.WorkItemType,
				"Microsoft.VSTS.WorkItemTypes.SharedParameter", "SharedParameter", "Shared Parameter",
				"Microsoft.VSTS.WorkItemTypes.SharedStep", "SharedStep", "Shared Step",
				"Microsoft.VSTS.WorkItemTypes.TestCase", "TestCase", "Test Case",
				"Microsoft.VSTS.WorkItemTypes.TestPlan", "TestPlan", "Test Plan",
				"Microsoft.VSTS.WorkItemTypes.TestSuite", "TestSuite", "Test Suite",
			) {
				return nil
			}

			// if this ticket ticket type does NOT have a resolution "allowed value" but it has a
			// completed state, make the reason the resolution - I know, confusion
			if !a.hasResolution(projid, fields.WorkItemType) {
				if a.completedState(projid, fields.WorkItemType, fields.State) {
					fields.ResolvedReason = fields.Reason
				}
			}

			storypoints := fields.StoryPoints
			issue := &sdk.WorkIssue{
				AssigneeRefID: fields.AssignedTo.ID,
				CreatorRefID:  fields.CreatedBy.ID,
				CustomerID:    a.customerID,
				Description:   fields.Description,
				Identifier:    fmt.Sprintf("%s-%d", fields.TeamProject, item.ID),
				Priority:      fmt.Sprint(fields.Priority),
				ProjectID:     sdk.NewWorkProjectID(a.customerID, projid, a.refType),
				RefID:         fmt.Sprint(item.ID),
				RefType:       a.refType,
				ReporterRefID: fields.CreatedBy.ID,
				Resolution:    fields.ResolvedReason, //itemStateName(fields.ResolvedReason, item.Fields.WorkItemType),
				Status:        fields.State,          // itemStateName(fields.State, item.Fields.WorkItemType),
				StoryPoints:   &storypoints,
				Tags:          strings.Split(fields.Tags, "; "),
				Title:         fields.Title,
				Type:          fields.WorkItemType,
				URL:           item.Links.HTML.HREF,
				SprintIds:     []string{sdk.NewWorkSprintID(a.customerID, fields.IterationPath, a.refType)},
			}

			sdk.ConvertTimeToDateModel(fields.CreatedDate, &issue.CreatedDate)
			sdk.ConvertTimeToDateModel(fields.DueDate, &issue.DueDate)

			var updatedDate time.Time
			if issue.ChangeLog, updatedDate, err = a.fetchChangeLog(fields.WorkItemType, projid, issue.RefID); err != nil {
				return err
			}
			// this should only happen if the changelog is empty, which should only happen when an issue is created and not modified,
			if updatedDate.IsZero() {
				updatedDate = fields.ChangedDate
			}
			sdk.ConvertTimeToDateModel(updatedDate, &issue.UpdatedDate)
			mutex.Lock()
			res = append(res, issue)
			mutex.Unlock()
			return nil
		})
	}

	if err := async.Wait(); err != nil {
		return nil, err
	}
	return res, nil
}

var hasResolutions = map[string]bool{}
var hasResolutionsMutex sync.Mutex

func (a *API) hasResolution(projid, refname string) bool {
	hasResolutionsMutex.Lock()
	has, ok := hasResolutions[refname]
	hasResolutionsMutex.Unlock()
	if ok {
		return has
	}
	params := url.Values{}
	params.Set("$expand", "allowedValues")
	endpoint := fmt.Sprintf(`_apis/wit/workitemtypes/%s/fields`, url.PathEscape(refname))

	var out struct {
		Value []resolutionResponse `json:"value"`
	}
	if _, err := a.get(sdk.JoinURL(projid, endpoint), params, &out); err != nil {
		fmt.Println(err)
		return false
	}
	for _, g := range out.Value {
		if len(g.AllowedValues) > 0 && g.ReferenceName == "Microsoft.VSTS.Common.ResolvedReason" {
			hasResolutionsMutex.Lock()
			hasResolutions[refname] = true
			hasResolutionsMutex.Unlock()

			return true
		}
	}
	hasResolutionsMutex.Lock()
	hasResolutions[refname] = false
	hasResolutionsMutex.Unlock()
	return false
}

var completedStates = map[string]string{}
var completedStatesMutex sync.Mutex

func (a *API) completedState(projid string, itemtype string, state string) bool {

	completedStatesMutex.Lock()
	if s, o := completedStates[itemtype]; o {
		completedStatesMutex.Unlock()
		return state == s
	}
	completedStatesMutex.Unlock()

	endpoint := fmt.Sprintf(`_apis/wit/workitemtypes/%s`, url.PathEscape(itemtype))
	var out workConfigResponse
	if _, err := a.get(sdk.JoinURL(projid, endpoint), nil, &out); err != nil {
		fmt.Println(err)
		return false
	}
	for _, r := range out.States {
		if workConfigStatus(r.Category) == workConfigCompletedStatus {
			completedStatesMutex.Lock()
			completedStates[itemtype] = r.Name
			completedStatesMutex.Unlock()
			return state == r.Name
		}
	}
	return false
}

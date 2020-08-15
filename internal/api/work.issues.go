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

// FetchStatuses gets issues from project id
func (a *API) FetchStatuses(issueStatusChannel chan<- *sdk.WorkIssueStatus) (*sdk.WorkConfig, error) {

	mu := sync.Mutex{}

	params := url.Values{}
	params.Set("api-version", "5.1-preview.2")
	var out struct {
		Value []processesResponse `json:"value"`
	}
	if _, err := a.get("_apis/work/processes", params, &out); err != nil {
		return nil, err
	}
	statuses := sdk.WorkConfigStatuses{}
	async := sdk.NewAsync(a.concurrency / 2)
	for _, _val := range out.Value {
		val := _val

		mu.Lock()
		if a.statusesMap[val.TypeID] == nil {
			a.statusesMap[val.TypeID] = map[string]map[string]string{}
		}
		mu.Unlock()

		async.Do(func() error {

			params := url.Values{}
			params.Set("api-version", "5.1-preview.2")
			var out struct {
				Value []itemTypeResponse `json:"value"`
			}
			if _, err := a.get("_apis/work/processes/"+val.TypeID+"/workItemTypes", params, &out); err != nil {
				return err
			}

			async2 := sdk.NewAsync(a.concurrency / 2)
			for _, _v := range out.Value {
				v := _v

				mu.Lock()
				if a.statusesMap[val.TypeID][v.Name] == nil {
					a.statusesMap[val.TypeID][v.Name] = map[string]string{}
				}
				mu.Unlock()

				async2.Do(func() error {

					params := url.Values{}
					params.Set("api-version", "5.1-preview.1")
					var out struct {
						Value []stateResponse `json:"value"`
					}
					if _, err := a.get("_apis/work/processes/"+val.TypeID+"/workItemTypes/"+v.ReferenceName+"/states", params, &out); err != nil {
						return err
					}

					for _, state := range out.Value {
						issueStatusChannel <- &sdk.WorkIssueStatus{
							CustomerID:            a.customerID,
							Description:           state.StateCategory,
							IntegrationInstanceID: &a.instanceID,
							Name:                  state.Name,
							RefID:                 state.ID,
							RefType:               a.refType,
						}

						mu.Lock()
						a.statusesMap[val.TypeID][v.Name][state.Name] = state.ID

						switch state.StateCategory {
						case "InProgress":
							statuses.InProgressStatus = appendUnique(statuses.InProgressStatus, state.Name)
						case "Proposed":
							statuses.OpenStatus = appendUnique(statuses.OpenStatus, state.Name)
						case "Completed", "Removed", "Resolved":
							statuses.ClosedStatus = appendUnique(statuses.ClosedStatus, state.Name)
						}

						mu.Unlock()

					}
					return nil
				})
			}
			return async2.Wait()
		})
	}
	err := async.Wait()
	workconf := &sdk.WorkConfig{
		CustomerID:            a.customerID,
		IntegrationInstanceID: a.instanceID,
		RefType:               a.refType,
		Statuses:              statuses,
	}
	return workconf, err
}

// FetchAllIssues gets issues from project id
func (a *API) FetchAllIssues(projid string, updated time.Time, issueChannel chan<- *sdk.WorkIssue, issueCommentChannel chan<- *sdk.WorkIssueComment) error {

	sdk.LogInfo(a.logger, "fetching issues for project", "project_id", projid)

	var q struct {
		Query string `json:"query"`
	}
	q.Query = `Select [System.ID], [System.Title] From WorkItems ORDER BY System.ChangedDate Desc` // get newest first
	if !updated.IsZero() {
		q.Query += fmt.Sprintf(` WHERE System.ChangedDate > '%s'`, updated.Format(whereDateFormat))
	}
	params := url.Values{}
	params.Set("timePrecision", "true")

	var out workItemsResponse
	if _, err := a.post(sdk.JoinURL(projid, "_apis/wit/wiql"), q, params, &out); err != nil {
		return nil
	}

	var items []string
	for i, item := range out.WorkItems {
		if i != 0 && (i%200) == 0 {
			err := a.FetchIssues(projid, items, issueChannel, issueCommentChannel)
			if err != nil {
				return err
			}
			items = []string{}
		}
		items = append(items, fmt.Sprint(item.ID))
	}
	err := a.FetchIssues(projid, items, issueChannel, issueCommentChannel)
	if err != nil {
		return err
	}
	return nil
}

// FetchIssues gets all the issues from the ids array
func (a *API) FetchIssues(projid string, ids []string, issueChannel chan<- *sdk.WorkIssue, issueCommentChannel chan<- *sdk.WorkIssueComment) error {

	processid := projectProcessMap[projid]
	process := a.statusesMap[processid]

	// flush the data once in a while
	if err := a.state.Flush(); err != nil {
		return err
	}

	sdk.LogInfo(a.logger, "fetching issues", "project_id", projid, "count", len(ids))

	if len(ids) == 0 {
		return nil
	}
	params := url.Values{}
	params.Set("ids", strings.Join(ids, ","))
	params.Set("$expand", "all")

	endpoint := "_apis/wit/workitems"

	var out struct {
		Value []workItemResponse `json:"value"`
	}
	// no need to paginate, this is 200 at most at a time, look at FetchIssues
	_, err := a.get(sdk.JoinURL(projid, endpoint), params, &out)
	if err != nil {
		return err
	}
	async := sdk.NewAsync(a.concurrency)
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
				AssigneeRefID:         fields.AssignedTo.ID,
				CreatorRefID:          fields.CreatedBy.ID,
				CustomerID:            a.customerID,
				Description:           fields.Description,
				IntegrationInstanceID: &a.instanceID,
				Identifier:            fmt.Sprintf("%s-%d", fields.TeamProject, item.ID),
				Priority:              fmt.Sprint(fields.Priority),
				ProjectID:             sdk.NewWorkProjectID(a.customerID, projid, a.refType),
				RefID:                 a.createIssueID(projid, item.ID),
				RefType:               a.refType,
				ReporterRefID:         fields.CreatedBy.ID,
				Resolution:            fields.ResolvedReason,
				Status:                fields.State,
				StatusID:              sdk.NewWorkIssueStatusID(a.customerID, a.refType, process[fields.WorkItemType][fields.State]),
				StoryPoints:           &storypoints,
				Tags:                  strings.Split(fields.Tags, "; "),
				Title:                 fields.Title,
				Type:                  fields.WorkItemType,
				URL:                   item.Links.HTML.HREF,
				SprintIds:             []string{sdk.NewAgileSprintID(a.customerID, fields.IterationPath, a.refType)},
			}
			sdk.ConvertTimeToDateModel(fields.CreatedDate, &issue.CreatedDate)
			sdk.ConvertTimeToDateModel(fields.DueDate, &issue.DueDate)

			var updatedDate time.Time
			if issue.ChangeLog, updatedDate, err = a.fetchChangeLog(fields.WorkItemType, projid, item.ID); err != nil {
				return err
			}
			// this should only happen if the changelog is empty, which should only happen when an issue is created and not modified,
			if updatedDate.IsZero() {
				updatedDate = fields.ChangedDate
			}
			sdk.ConvertTimeToDateModel(updatedDate, &issue.UpdatedDate)
			issueChannel <- issue
			return nil
		})
		async.Do(func() error {
			return a.fetchComments(projid, item.ID, issueCommentChannel)
		})
	}

	if err := async.Wait(); err != nil {
		return err
	}
	return nil
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

// CreateIssue creates an issue
func (a *API) CreateIssue(obj *sdk.WorkIssueCreateMutation) error {
	endpoint := fmt.Sprintf("%s/_apis/wit/workitems/%s", obj.ProjectRefID, *obj.Type.Name)
	type item struct {
		OP    string `json:"op"`
		Path  string `json:"path"`
		Value string `json:"value"`
	}
	var payload []item
	addToPayload := func(path string, value string) {
		payload = append(payload, item{
			OP:    "add",
			Path:  "/fields/" + path,
			Value: value,
		})
	}
	addToPayload("System.Title", obj.Title)
	addToPayload("System.Description", obj.Description)
	if obj.Priority != nil {
		addToPayload("Microsoft.VSTS.Common.Priority", *obj.Priority.Name)
	}
	if obj.AssigneeRefID != nil {
		addToPayload("System.AssignedTo", sdk.Stringify(usersResponse{ID: *obj.AssigneeRefID}))
	}
	if obj.Labels != nil {
		addToPayload("System.Tags", strings.Join(obj.Labels, "; "))
	}
	var out interface{}
	if _, err := a.post(endpoint, payload, nil, &out); err != nil {
		return err
	}
	return nil
}

// UpdateIssue updates an issue
func (a *API) UpdateIssue(refid string, obj *sdk.WorkIssueUpdateMutation) error {
	projectid, issueid, err := a.FetchIssueProjectRefs(refid)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/_apis/wit/workitems/%s", projectid, fmt.Sprint(issueid))
	type item struct {
		OP    string `json:"op"`
		Path  string `json:"path"`
		Value string `json:"value"`
	}
	var payload []item

	if title := obj.Set.Title; title != nil {
		payload = append(payload, item{
			OP:    "add",
			Path:  "/fields/System.Title",
			Value: *title,
		})
	}
	if status := obj.Set.Status; status != nil {
		payload = append(payload, item{
			OP:    "add",
			Path:  "/fields/System.State",
			Value: *status.Name,
		})

	}
	if priority := obj.Set.Priority; priority != nil {
		payload = append(payload, item{
			OP:    "add",
			Path:  "/fields/Microsoft.VSTS.Common.Priority",
			Value: *priority.Name,
		})
	}
	if resolution := obj.Set.Resolution; resolution != nil {
		payload = append(payload, item{
			OP:    "add",
			Path:  "/fields/Microsoft.VSTS.Common.ResolvedReason",
			Value: *resolution.Name,
		})
	}
	if assigned := obj.Set.AssigneeRefID; assigned != nil {
		payload = append(payload, item{
			OP:    "add",
			Path:  "/fields/System.AssignedTo",
			Value: sdk.Stringify(usersResponse{ID: *assigned}),
		})
	}
	if obj.Unset.Assignee {
		payload = append(payload, item{
			OP:   "remove",
			Path: "/fields/System.AssignedTo",
		})
	}
	var out interface{}
	if _, err := a.post(endpoint, payload, nil, &out); err != nil {
		return err
	}
	return nil
}

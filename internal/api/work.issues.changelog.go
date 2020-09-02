package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

func (a *API) fetchChangeLog(itemtype, projid string, issueid int) ([]sdk.WorkIssueChangeLog, time.Time, error) {

	params := url.Values{}
	params.Set("$top", "200")
	endpoint := fmt.Sprintf("_apis/wit/workItems/%s/updates", url.PathEscape(fmt.Sprint(issueid)))

	var changelogs []sdk.WorkIssueChangeLog
	var latestChange time.Time

	out := make(chan objects)
	errochan := make(chan error)
	go func() {
		for object := range out {
			var value []changelogResponse
			if err := object.Unmarshal(&value); err != nil {
				errochan <- err
				return
			}
			changelogs = append(changelogs, a.processChangelogs(value)...)
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
	if err := <-errochan; err != nil {
		return nil, time.Time{}, err
	}
	sort.Slice(changelogs, func(i int, j int) bool {
		return changelogs[i].CreatedDate.Epoch < changelogs[j].CreatedDate.Epoch
	})
	if len(changelogs) > 0 {
		last := changelogs[len(changelogs)-1]
		latestChange = sdk.DateFromEpoch(last.CreatedDate.Epoch)
	}
	return changelogs, latestChange, nil
}

func (a *API) processChangelogs(response []changelogResponse) []sdk.WorkIssueChangeLog {
	var changelogs []sdk.WorkIssueChangeLog
	previousState := ""
	for i, changelog := range response {
		if changelog.Fields == nil {
			continue
		}
		// check if there is a parent
		changelogCreateParentField(&changelog)
		// get the created date, if any. Some changelogs don't have this
		createdDate := changeLogExtractCreatedDate(changelog)
		for field, values := range changelog.Fields {
			if extractor, ok := changelogFields[field]; ok {
				if i == 0 && changelogToString(values.OldValue) == "" {
					continue
				}
				values.customerID = a.customerID
				values.refType = a.refType
				name, from, to := extractor(values)
				if from == "" && to == "" {
					continue
				}

				if name == sdk.WorkIssueChangeLogFieldStatus {
					if to == "" {
						previousState = from
						continue
					}
					if from == "" && previousState != "" {
						from = previousState
						previousState = ""
					}
					if to == from {
						continue
					}
				}
				changelogs = append(changelogs, sdk.WorkIssueChangeLog{
					RefID:       fmt.Sprintf("%d", changelog.ID),
					CreatedDate: createdDate,
					Field:       name,
					From:        from,
					FromString:  from,
					Ordinal:     int64(i),
					To:          to,
					ToString:    to,
					UserID:      changelog.RevisedBy.ID,
				})
			}
		}
	}
	return changelogs
}

func changelogCreateParentField(changelog *changelogResponse) {
	if added := changelog.Relations.Added; added != nil {
		for _, each := range added {
			if each.Attributes.Name == "Parent" {
				changelog.Fields["parent"] = changelogField{
					NewValue: filepath.Base(each.URL), // get the work item id from the url
				}
				break
			}
		}
	}
	if removed := changelog.Relations.Removed; removed != nil {
		for _, each := range removed {
			if each.Attributes.Name == "Parent" {
				var field changelogField
				var ok bool
				if field, ok = changelog.Fields["parent"]; ok {
					field.OldValue = filepath.Base(each.URL) // get the work item id from the url
				} else {
					field = changelogField{
						OldValue: filepath.Base(each.URL), // get the work item id from the url
					}
				}
				changelog.Fields["parent"] = field
				break
			}
		}
	}
}

func changelogToString(i interface{}) string {
	if i == nil {
		return ""
	}
	if s, o := i.(string); o {
		return s
	}
	if s, o := i.(float64); o {
		return fmt.Sprintf("%f", s)
	}
	return fmt.Sprintf("%v", i)
}

func changeLogExtractCreatedDate(changelog changelogResponse) sdk.WorkIssueChangeLogCreatedDate {
	var createdDate sdk.WorkIssueChangeLogCreatedDate
	// This field is always there
	// System.ChangedDate is the created date if there is only one changelog
	if field, ok := changelog.Fields["System.ChangedDate"]; ok {
		created, err := time.Parse(time.RFC3339, fmt.Sprint(field.NewValue))
		if err == nil {
			sdk.ConvertTimeToDateModel(created, &createdDate)
		}
	} else {
		sdk.ConvertTimeToDateModel(changelog.RevisedDate, &createdDate)
	}
	if createdDate.Epoch < 0 {
		return sdk.WorkIssueChangeLogCreatedDate{}
	}
	return createdDate
}

type changeLogFieldExtractor func(item changelogField) (name sdk.WorkIssueChangeLogField, from string, to string)

func extractUsers(item changelogField) (from string, to string) {

	toUser := func(in interface{}, out *usersResponse) error {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		err = json.Unmarshal(b, out)
		return err
	}

	if item.OldValue != nil {
		var user usersResponse
		toUser(item.OldValue, &user)
		from = user.ID
	}
	if item.NewValue != nil {
		var user usersResponse
		toUser(item.NewValue, &user)
		to = user.ID
	}
	return
}

var changelogFields = map[string]changeLogFieldExtractor{
	"System.State": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		return sdk.WorkIssueChangeLogFieldStatus, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"Microsoft.VSTS.Common.ResolvedReason": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		return sdk.WorkIssueChangeLogFieldResolution, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.AssignedTo": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		from, to := extractUsers(item)
		return sdk.WorkIssueChangeLogFieldAssigneeRefID, from, to
	},
	"System.CreatedBy": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		from, to := extractUsers(item)
		return sdk.WorkIssueChangeLogFieldReporterRefID, from, to
	},
	"System.Title": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		return sdk.WorkIssueChangeLogFieldTitle, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	// convert to date
	"Microsoft.VSTS.Scheduling.DueDate": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		return sdk.WorkIssueChangeLogFieldDueDate, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.WorkItemType": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		return sdk.WorkIssueChangeLogFieldType, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.Tags": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		from := changelogToString(item.OldValue)
		to := changelogToString(item.NewValue)
		if from != "" {
			tmp := strings.Split(from, "; ")
			from = strings.Join(tmp, ",")
		}
		if to != "" {
			tmp := strings.Split(from, "; ")
			to = strings.Join(tmp, ",")
		}
		return sdk.WorkIssueChangeLogFieldTags, from, to
	},
	"Microsoft.VSTS.Common.Priority": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		return sdk.WorkIssueChangeLogFieldPriority, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.Id": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		oldvalue := sdk.NewWorkIssueID(item.customerID, changelogToString(item.OldValue), item.refType)
		newvalue := sdk.NewWorkIssueID(item.customerID, changelogToString(item.NewValue), item.refType)
		return sdk.WorkIssueChangeLogFieldProjectID, oldvalue, newvalue
	},
	"System.TeamProject": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		oldvalue := sdk.NewWorkProjectID(item.customerID, changelogToString(item.OldValue), item.refType)
		newvalue := sdk.NewWorkProjectID(item.customerID, changelogToString(item.NewValue), item.refType)
		return sdk.WorkIssueChangeLogFieldIdentifier, oldvalue, newvalue
	},
	"System.IterationPath": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		oldvalue := sdk.NewAgileSprintID(item.customerID, changelogToString(item.OldValue), item.refType)
		newvalue := sdk.NewAgileSprintID(item.customerID, changelogToString(item.NewValue), item.refType)
		return sdk.WorkIssueChangeLogFieldSprintIds, oldvalue, newvalue
	},
	"parent": func(item changelogField) (sdk.WorkIssueChangeLogField, string, string) {
		oldvalue := sdk.NewWorkIssueID(item.customerID, changelogToString(item.OldValue), item.refType)
		newvalue := sdk.NewWorkIssueID(item.customerID, changelogToString(item.NewValue), item.refType)
		return sdk.WorkIssueChangeLogFieldParentID, oldvalue, newvalue
	},
	// "Epic Link": func(item work.IssueChangeLog) (string, interface{}, interface{}) {
	// 	var from, to string
	// 	if item.From != "" {
	// 		from = pw.NewIssueID(item.CustomerID, "jira", item.From)
	// 	}
	// 	if item.To != "" {
	// 		to = pw.NewIssueID(item.CustomerID, "jira", item.To)
	// 	}
	// 	return "epic_id", from, to
	// },
}

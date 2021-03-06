package api

import (
	"fmt"

	"github.com/pinpt/agent/v4/sdk"
)

func (a *API) FetchSprints(projid string, teamids []string) error {

	sdk.LogInfo(a.logger, "fetching sprints", "project_id", projid)

	for _, teamid := range teamids {
		endpoint := fmt.Sprintf("%s/_apis/work/teamsettings/iterations", teamid)

		var out struct {
			Value []sprintsResponse `json:"value"`
		}
		_, err := a.get(sdk.JoinURL(projid, endpoint), nil, &out)
		if err != nil {
			return err
		}
		for _, r := range out.Value {
			sprint := &sdk.AgileSprint{
				Active:     true,
				CustomerID: a.customerID,
				// Goal is missing
				Name:    r.Name,
				RefID:   r.Path, // ID's don't match changelog IDs, use path here and IterationPath there
				RefType: a.refType,
			}
			switch r.Attributes.TimeFrame {
			case "past":
				sprint.Status = sdk.AgileSprintStatusClosed
			case "current":
				sprint.Status = sdk.AgileSprintStatusActive
			case "future":
				sprint.Status = sdk.AgileSprintStatusFuture
			default:
				sprint.Status = sdk.AgileSprintStatus(4) // unset
			}
			sdk.ConvertTimeToDateModel(r.Attributes.StartDate, &sprint.StartedDate)
			sdk.ConvertTimeToDateModel(r.Attributes.FinishDate, &sprint.EndedDate)
			sdk.ConvertTimeToDateModel(r.Attributes.FinishDate, &sprint.CompletedDate)
			if err := a.pipe.Write(sprint); err != nil {
				return err
			}
		}
	}
	return nil
}

package api

import (
	"encoding/json"
	"time"
)

type pageResponse struct {
	Count int64   `json:"count"`
	Value objects `json:"value"`
	// comments don't have a "value" property, oy vey
	Comments objects `json:"comments"`
}

type objects []map[string]interface{}

func (o objects) Unmarshal(out interface{}) error {
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

type workItemResponse struct {
	Links struct {
		HTML struct {
			HREF string `json:"href"`
		} `json:"html"`
		// there are more here, fields, self, workItemComments, workItemRevisions, workItemType, and workItemUpdates
	} `json:"_links"`
	Fields struct {
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
	} `json:"fields"`
	Relations []struct {
		Rel string `json:"rel"`
		URL string `json:"url"`
	} `json:"relations"`
	ID  int    `json:"id"`
	URL string `json:"url"`
}

type usersResponse struct {
	Descriptor  string `json:"descriptor"`
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
	ImageURL    string `json:"imageUrl"`
	UniqueName  string `json:"uniqueName"`
	URL         string `json:"url"`
}
type resolutionResponse struct {
	AllowedValues []string `json:"allowedValues"`
	Name          string   `json:"name"`
	ReferenceName string   `json:"referenceName"`
}
type workConfigResponse struct {
	Name          string `json:"name"`
	ReferenceName string `json:"referenceName"`
	States        []struct {
		Category string `json:"category"`
		Name     string `json:"name"`
	} `json:"states"`
	FieldInstances []struct {
		ReferenceName string `json:"referenceName"`
	} `json:"fieldInstances"`
	Fields []struct {
		ReferenceName string `json:"referenceName"`
	} `json:"fields"`
}

type workConfigStatus string

// These seem to be the default statuses
const workConfigCompletedStatus = workConfigStatus("Completed")
const workConfigInProgressStatus = workConfigStatus("InProgress")
const workConfigProposedStatus = workConfigStatus("Proposed")
const workConfigRemovedStatus = workConfigStatus("Removed")
const workConfigResolvedStatus = workConfigStatus("Resolved")

type changelogField struct {
	NewValue   interface{} `json:"newValue"`
	OldValue   interface{} `json:"oldvalue"`
	customerID string
	refType    string
}

type changelogResponse struct {
	Fields      map[string]changelogField `json:"fields"`
	ID          int64                     `json:"id"`
	RevisedDate time.Time                 `json:"revisedDate"`
	URL         string                    `json:"url"`
	Relations   struct {
		Added []struct {
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
			URL string `json:"url"`
		} `json:"added"`
		Removed []struct {
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
			URL string `json:"url"`
		} `json:"removed"`
	} `json:"relations"`
	RevisedBy usersResponse `json:"revisedBy"`
}

type sprintsResponse struct {
	Attributes struct {
		FinishDate time.Time `json:"finishDate"`
		StartDate  time.Time `json:"startDate"`
		TimeFrame  string    `json:"timeFrame"` // past, current, future
	} `json:"attributes"`
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url"`
}

type teamsResponse struct {
	Description string `json:"description"`
	ID          string `json:"id"`
	IdentityURL string `json:"identityUrl"`
	Name        string `json:"name"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	URL         string `json:"url"`
}

type workItemsResponse struct {
	AsOf    time.Time `json:"asOf"`
	Columns []struct {
		Name          string `json:"name"`
		ReferenceName string `json:"referenceName"`
		URL           string `json:"url"`
	} `json:"columns"`
	QueryResultType string `json:"queryResultType"`
	QueryType       string `json:"queryType"`
	SortColumns     []struct {
		Descending bool `json:"descending"`
		Field      struct {
			Name          string `json:"name"`
			ReferenceName string `json:"referenceName"`
			URL           string `json:"url"`
		} `json:"field"`
	} `json:"sortColumns"`
	WorkItems []struct {
		ID  int64  `json:"id"`
		URL string `json:"url"`
	} `json:"workItems"`
}

type projectResponseLight struct {
	ID             string `json:"id"`
	LastUpdateTime string `json:"lastUpdateTime"` // not in TFS
	Name           string `json:"name"`
	State          string `json:"state"`
}

type projectResponse struct {
	projectResponseLight
	Revision    int64  `json:"revision"`
	State       string `json:"state"`
	URL         string `json:"url"`
	Visibility  string `json:"visibility"`
	Description string `json:"description"`
}

type usersResponseAzure struct {
	Identity usersResponse `json:"identity"`
}

type reposResponseLight struct {
	ID      string               `json:"id"`
	Name    string               `json:"name"`
	Project projectResponseLight `json:"project"`
	URL     string               `json:"url"`
}

// used in src_repos.go - fetchRepos
type reposResponse struct {
	reposResponseLight
	DefaultBranch string          `json:"defaultBranch"`
	Project       projectResponse `json:"project"`
	RemoteURL     string          `json:"remoteUrl"`
	Size          int64           `json:"size"`
	SSHURL        string          `json:"sshUrl"`
	WebURL        string          `json:"webUrl"`
}

type pullRequestResponse struct {
	ClosedDate          time.Time     `json:"closedDate"`
	CodeReviewID        int64         `json:"codeReviewId"`
	CreatedBy           usersResponse `json:"createdBy"`
	CreationDate        time.Time     `json:"creationDate"`
	CompletionQueueTime time.Time     `json:"completionQueueTime"`
	Description         string        `json:"description"`
	IsDraft             bool          `json:"isDraft"`
	LastMergeCommit     struct {
		CommidID string `json:"commitId"`
		URL      string `json:"url"`
	} `json:"lastMergeCommit"`
	LastMergeSourceCommit struct {
		CommidID string `json:"commitId"`
		URL      string `json:"url"`
	} `json:"lastMergeSourceCommit"`
	LastMergeTargetCommit struct {
		CommidID string `json:"commitId"`
		URL      string `json:"url"`
	} `json:"lastMergeTargetCommit"`
	MergeID       string             `json:"mergeId"`
	MergeStatus   string             `json:"mergeStatus"`
	PullRequestID int                `json:"pullRequestId"`
	Repository    reposResponseLight `json:"repository"`
	Reviewers     []struct {
		DisplayName string `json:"displayName"`
		ID          string `json:"id"`
		ImageURL    string `json:"imageUrl"`
		IsFlagged   bool   `json:"isFlagged"`
		ReviewerURL string `json:"reviewerUrl"`
		UniqueName  string `json:"uniqueName"`
		URL         string `json:"url"`
		Vote        int64  `json:"vote"`
	} `json:"reviewers"`
	SourceBranch       string `json:"sourceRefName"`
	Status             string `json:"status"`
	SupportsIterations bool   `json:"supportsIterations"`
	TargetBranch       string `json:"targetRefName"`
	Title              string `json:"title"`
	URL                string `json:"url"`
}

type pullRequestResponseWithShas struct {
	pullRequestResponse
	commitSHAs []string
}

type commitsResponseLight struct {
	Author struct {
		Date  time.Time `json:"date"`
		Email string    `json:"email"`
		Name  string    `json:"name"`
	} `json:"author"`
	Comment   string `json:"comment"`
	CommitID  string `json:"commitId"`
	Committer struct {
		Date  time.Time `json:"date"`
		Email string    `json:"email"`
		Name  string    `json:"name"`
	} `json:"committer"`
	URL          string `json:"url"`
	ChangeCounts struct {
		Add    int `json:"Add"`
		Delete int `json:"Delete"`
		Edit   int `json:"Edit"`
	} `json:"changeCounts"`
}

// used in src_commit_users.go - fetchCommits
type commitsResponse struct {
	commitsResponseLight
	RemoteURL    string `json:"remoteUrl"`
	ChangeCounts struct {
		Add    int64 `json:"Add"`
		Delete int64 `json:"Delete"`
		Edit   int64 `json:"Edit"`
	} `json:"changeCounts"`
}

type singleCommitResponse struct {
	Author struct {
		Date     time.Time `json:"date"`
		Email    string    `json:"email"`
		ImageURL string    `json:"imageUrl"`
		Name     string    `json:"name"`
	} `json:"author"`
	ChangeCounts struct {
		Add    int64 `json:"Add"`
		Delete int64 `json:"Delete"`
		Edit   int64 `json:"Edit"`
	} `json:"changeCounts"`
	Comment   string `json:"comment"`
	Committer struct {
		Date     time.Time `json:"date"`
		Email    string    `json:"email"`
		ImageURL string    `json:"imageUrl"`
		Name     string    `json:"name"`
	} `json:"committer"`
	Push struct {
		Date     time.Time     `json:"date"`
		PushedBy usersResponse `json:"pushedBy"`
	} `json:"push"`
	RemoteURL string `json:"remoteUrl"`
}
type threadsReponse struct {
	Comments []struct {
		Author                 usersResponse `json:"author"`
		CommentType            string        `json:"commentType"`
		Content                string        `json:"content"`
		ID                     int64         `json:"id"`
		LastContentUpdatedDate time.Time     `json:"lastContentUpdatedDate"`
		LastUpdatedDate        time.Time     `json:"lastUpdatedDate"`
		ParentCommentID        int64         `json:"parentCommentId"`
		PublishedDate          time.Time     `json:"publishedDate"`
	} `json:"comments"`
	ID              int64                    `json:"id"`
	Identities      map[string]usersResponse `json:"identities"`
	IsDeleted       bool                     `json:"isDeleted"`
	LastUpdatedDate time.Time                `json:"lastUpdatedDate"`
	PublishedDate   time.Time                `json:"publishedDate"`
}

type webhookPayload struct {
	ID               string `json:"id,omitempty"` // readonly
	ConsumerActionID string `json:"consumerActionId"`
	ConsumerID       string `json:"consumerId"`
	ConsumerInputs   struct {
		URL string `json:"url"`
	} `json:"consumerInputs"`
	EventType       string `json:"eventType"`
	PublisherID     string `json:"publisherId"`
	PublisherInputs struct {
		ProjectID string `json:"projectId"`
	} `json:"publisherInputs"`
	ResourceVersion string `json:"resourceVersion"`
	Scope           int    `json:"scope"`
}
type issueCommentReponse struct {
	CreatedBy struct {
		Links struct {
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"_links"`
		Descriptor  string `json:"descriptor"`
		DisplayName string `json:"displayName"`
		ID          string `json:"id"`
		ImageURL    string `json:"imageUrl"`
		UniqueName  string `json:"uniqueName"`
		URL         string `json:"url"`
	} `json:"createdBy"`
	CreatedDate time.Time `json:"createdDate"`
	ID          int       `json:"id"`
	ModifiedBy  struct {
		Links struct {
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"_links"`
		Descriptor  string `json:"descriptor"`
		DisplayName string `json:"displayName"`
		ID          string `json:"id"`
		ImageURL    string `json:"imageUrl"`
		UniqueName  string `json:"uniqueName"`
		URL         string `json:"url"`
	} `json:"modifiedBy"`
	ModifiedDate time.Time `json:"modifiedDate"`
	Text         string    `json:"text"`
	URL          string    `json:"url"`
	Version      int       `json:"version"`
	WorkItemID   int       `json:"workItemId"`
}

package export

import (
	tasks "google.golang.org/api/tasks/v1"
)

// SchemaVersion identifies the layout of the exported JSON document. Bump it
// when the envelope shape changes in a backwards-incompatible way.
const SchemaVersion = "1.0"

// File is the top-level exported document. Unlike the raw google.golang.org/api
// task types, none of the envelope structs use `omitempty`: every field is
// always emitted so that "false", "", null and empty-array values are preserved
// verbatim. This is the whole point of the tool — capturing every field,
// including the metadata the Google Tasks UI never surfaces.
type File struct {
	SchemaVersion string   `json:"schemaVersion"`
	Export        Summary  `json:"export"`
	List          TaskList `json:"list"`
	Tasks         []Task   `json:"tasks"`
}

// Summary describes how and when the export was produced.
type Summary struct {
	GeneratedAt string `json:"generatedAt"`
	Tool        string `json:"tool"`
	ToolVersion string `json:"toolVersion"`
	Scope       string `json:"scope"`
	ListID      string `json:"listId"`
	ListTitle   string `json:"listTitle"`
	Counts      Counts `json:"counts"`
}

// Counts is a breakdown of the tasks captured in an export. The buckets are not
// mutually exclusive (a task can be both completed and hidden, for example).
type Counts struct {
	Total       int `json:"total"`
	NeedsAction int `json:"needsAction"`
	Completed   int `json:"completed"`
	Deleted     int `json:"deleted"`
	Hidden      int `json:"hidden"`
	Assigned    int `json:"assigned"`
	Subtasks    int `json:"subtasks"`
	TopLevel    int `json:"topLevel"`
}

// TaskList mirrors tasks.TaskList with every field always present.
type TaskList struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Etag     string `json:"etag"`
	Title    string `json:"title"`
	Updated  string `json:"updated"`
	SelfLink string `json:"selfLink"`
}

// Task mirrors tasks.Task with every field always present.
//
// Completed is a pointer so that a task which has never been completed marshals
// as JSON null (honest) rather than being dropped. AssignmentInfo, DriveResource
// and SpaceInfo are likewise nil -> null when absent. Links is always an array,
// never null.
type Task struct {
	Kind           string          `json:"kind"`
	ID             string          `json:"id"`
	Etag           string          `json:"etag"`
	Title          string          `json:"title"`
	Notes          string          `json:"notes"`
	Status         string          `json:"status"`
	Updated        string          `json:"updated"`
	SelfLink       string          `json:"selfLink"`
	Parent         string          `json:"parent"`
	Position       string          `json:"position"`
	Due            string          `json:"due"`
	Completed      *string         `json:"completed"`
	Deleted        bool            `json:"deleted"`
	Hidden         bool            `json:"hidden"`
	Links          []Link          `json:"links"`
	WebViewLink    string          `json:"webViewLink"`
	AssignmentInfo *AssignmentInfo `json:"assignmentInfo"`
}

// Link mirrors tasks.TaskLinks.
type Link struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Link        string `json:"link"`
}

// AssignmentInfo mirrors tasks.AssignmentInfo (present for tasks assigned from
// Google Docs or Chat Spaces).
type AssignmentInfo struct {
	LinkToTask        string             `json:"linkToTask"`
	SurfaceType       string             `json:"surfaceType"`
	DriveResourceInfo *DriveResourceInfo `json:"driveResourceInfo"`
	SpaceInfo         *SpaceInfo         `json:"spaceInfo"`
}

// DriveResourceInfo mirrors tasks.DriveResourceInfo.
type DriveResourceInfo struct {
	DriveFileID string `json:"driveFileId"`
	ResourceKey string `json:"resourceKey"`
}

// SpaceInfo mirrors tasks.SpaceInfo.
type SpaceInfo struct {
	Space string `json:"space"`
}

// fromAPITaskList copies every field of a tasks.TaskList into the envelope.
func fromAPITaskList(tl *tasks.TaskList) TaskList {
	if tl == nil {
		return TaskList{}
	}

	return TaskList{
		Kind:     tl.Kind,
		ID:       tl.Id,
		Etag:     tl.Etag,
		Title:    tl.Title,
		Updated:  tl.Updated,
		SelfLink: tl.SelfLink,
	}
}

// fromAPITask copies every field of a tasks.Task into the envelope.
func fromAPITask(t *tasks.Task) Task {
	return Task{
		Kind:           t.Kind,
		ID:             t.Id,
		Etag:           t.Etag,
		Title:          t.Title,
		Notes:          t.Notes,
		Status:         t.Status,
		Updated:        t.Updated,
		SelfLink:       t.SelfLink,
		Parent:         t.Parent,
		Position:       t.Position,
		Due:            t.Due,
		Completed:      copyStringPtr(t.Completed),
		Deleted:        t.Deleted,
		Hidden:         t.Hidden,
		Links:          fromAPILinks(t.Links),
		WebViewLink:    t.WebViewLink,
		AssignmentInfo: fromAPIAssignmentInfo(t.AssignmentInfo),
	}
}

// fromAPILinks always returns a non-nil slice so the JSON is `[]`, not `null`.
func fromAPILinks(in []*tasks.TaskLinks) []Link {
	out := make([]Link, 0, len(in))
	for _, l := range in {
		if l == nil {
			continue
		}

		out = append(out, Link{
			Type:        l.Type,
			Description: l.Description,
			Link:        l.Link,
		})
	}

	return out
}

// fromAPIAssignmentInfo copies assignment metadata; nil stays nil (-> null).
func fromAPIAssignmentInfo(a *tasks.AssignmentInfo) *AssignmentInfo {
	if a == nil {
		return nil
	}

	out := &AssignmentInfo{
		LinkToTask:  a.LinkToTask,
		SurfaceType: a.SurfaceType,
	}
	if a.DriveResourceInfo != nil {
		out.DriveResourceInfo = &DriveResourceInfo{
			DriveFileID: a.DriveResourceInfo.DriveFileId,
			ResourceKey: a.DriveResourceInfo.ResourceKey,
		}
	}

	if a.SpaceInfo != nil {
		out.SpaceInfo = &SpaceInfo{Space: a.SpaceInfo.Space}
	}

	return out
}

func copyStringPtr(s *string) *string {
	if s == nil {
		return nil
	}

	return new(*s) // pointer to a copy of the string value (Go 1.26 new(expr))
}

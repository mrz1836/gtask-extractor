package tasksclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	tasks "google.golang.org/api/tasks/v1"
)

// newTestClient builds a real *tasks.Service pointed at an in-process httptest
// server. option.WithHTTPClient makes the API library use srv.Client() verbatim
// (no ADC discovery) and option.WithEndpoint overrides the base URL.
func newTestClient(t *testing.T, srv *httptest.Server, maxPages int) *Client {
	t.Helper()

	svc, err := tasks.NewService(context.Background(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	return &Client{svc: svc, maxPages: maxPages}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

func TestListTaskListsPagination(t *testing.T) {
	var (
		paths  []string
		tokens []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)

		tokens = append(tokens, r.URL.Query().Get("pageToken"))
		switch r.URL.Query().Get("pageToken") {
		case "":
			writeJSON(t, w, tasks.TaskLists{
				Items:         []*tasks.TaskList{{Id: "a", Title: "A"}, {Id: "b", Title: "B"}},
				NextPageToken: "page2",
			})
		case "page2":
			writeJSON(t, w, tasks.TaskLists{
				Items: []*tasks.TaskList{{Id: "c", Title: "C"}},
			})
		default:
			t.Errorf("unexpected pageToken %q", r.URL.Query().Get("pageToken"))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv, 10)

	lists, err := c.ListTaskLists(context.Background())
	if err != nil {
		t.Fatalf("ListTaskLists: %v", err)
	}

	gotIDs := ids(lists)
	if want := []string{"a", "b", "c"}; !equal(gotIDs, want) {
		t.Errorf("ids = %v, want %v", gotIDs, want)
	}

	for _, p := range paths {
		if p != "/tasks/v1/users/@me/lists" {
			t.Errorf("path = %q, want /tasks/v1/users/@me/lists", p)
		}
	}

	if want := []string{"", "page2"}; !equal(tokens, want) {
		t.Errorf("pageTokens sent = %v, want %v", tokens, want)
	}
}

func TestListTasksSetsAllShowFlags(t *testing.T) {
	var gotQuery, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery

		writeJSON(t, w, tasks.Tasks{Items: []*tasks.Task{{Id: "t1"}}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, 10)

	got, err := c.ListTasks(context.Background(), "LIST123")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if len(got) != 1 || got[0].Id != "t1" {
		t.Errorf("tasks = %+v, want one task t1", got)
	}

	if gotPath != "/tasks/v1/lists/LIST123/tasks" {
		t.Errorf("path = %q, want /tasks/v1/lists/LIST123/tasks", gotPath)
	}

	for _, want := range []string{
		"showCompleted=true", "showHidden=true", "showDeleted=true",
		"showAssigned=true", "maxResults=100",
	} {
		if !contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestListTasksPaginationOrdering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("pageToken") {
		case "":
			writeJSON(t, w, tasks.Tasks{
				Items:         []*tasks.Task{{Id: "t1"}, {Id: "t2"}},
				NextPageToken: "next",
			})
		case "next":
			writeJSON(t, w, tasks.Tasks{Items: []*tasks.Task{{Id: "t3"}}})
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv, 10)

	got, err := c.ListTasks(context.Background(), "L")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	var ids []string
	for _, tk := range got {
		ids = append(ids, tk.Id)
	}

	if want := []string{"t1", "t2", "t3"}; !equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestEmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, tasks.Tasks{})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, 10)

	got, err := c.ListTasks(context.Background(), "L")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("expected no tasks, got %d", len(got))
	}
}

func TestHTTPErrorsSurface(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":{"message":"nope"}}`, code)
		}))
		c := newTestClient(t, srv, 10)
		_, err := c.ListTasks(context.Background(), "L")

		srv.Close()

		if err == nil {
			t.Fatalf("code %d: expected error, got nil", code)
		}

		if gerr, ok := errors.AsType[*googleapi.Error](err); !ok || gerr.Code != code {
			t.Errorf("code %d: error = %v, want googleapi.Error with that code", code, err)
		}
	}
}

func TestListTaskListsHTTPErrorSurfaces(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":{"message":"nope"}}`, code)
		}))
		c := newTestClient(t, srv, 10)
		_, err := c.ListTaskLists(context.Background())

		srv.Close()

		if gerr, ok := errors.AsType[*googleapi.Error](err); !ok || gerr.Code != code {
			t.Errorf("code %d: error = %v, want googleapi.Error with that code", code, err)
		}
	}
}

func TestPaginationLoopGuard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Always return the same token -> a loop.
		writeJSON(t, w, tasks.Tasks{
			Items:         []*tasks.Task{{Id: "x"}},
			NextPageToken: "same-token",
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, 1000)

	_, err := c.ListTasks(context.Background(), "L")
	if !errors.Is(err, errPaginationLoop) {
		t.Fatalf("err = %v, want errPaginationLoop", err)
	}
}

func TestMaxPagesGuard(t *testing.T) {
	// Each page has a unique token so the loop guard never trips; the page cap
	// must stop us instead.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := r.URL.Query().Get("pageToken")
		writeJSON(t, w, tasks.Tasks{
			Items:         []*tasks.Task{{Id: "x"}},
			NextPageToken: tok + "x",
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, 3)

	_, err := c.ListTasks(context.Background(), "L")
	if !errors.Is(err, errTooManyPages) {
		t.Fatalf("err = %v, want errTooManyPages", err)
	}

	if !contains(err.Error(), "max 3 pages") {
		t.Errorf("err = %v, want it to mention the page cap", err)
	}
}

func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, tasks.Tasks{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	c := newTestClient(t, srv, 10)
	if _, err := c.ListTasks(ctx, "L"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if _, err := c.ListTaskLists(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// --- small local helpers (no external deps) ---

func ids(lists []*tasks.TaskList) []string {
	out := make([]string, 0, len(lists))
	for _, l := range lists {
		out = append(out, l.Id)
	}

	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

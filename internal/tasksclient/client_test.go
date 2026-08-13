package tasksclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	return &Client{svc: svc, maxPages: maxPages}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")

	require.NoError(t, json.NewEncoder(w).Encode(v))
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
	require.NoError(t, err)

	gotIDs := ids(lists)
	assert.Equal(t, []string{"a", "b", "c"}, gotIDs)

	for _, p := range paths {
		assert.Equal(t, "/tasks/v1/users/@me/lists", p)
	}

	assert.Equal(t, []string{"", "page2"}, tokens)
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
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "t1", got[0].Id)

	assert.Equal(t, "/tasks/v1/lists/LIST123/tasks", gotPath)

	for _, want := range []string{
		"showCompleted=true", "showHidden=true", "showDeleted=true",
		"showAssigned=true", "maxResults=100",
	} {
		assert.Contains(t, gotQuery, want)
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
	require.NoError(t, err)

	var ids []string
	for _, tk := range got {
		ids = append(ids, tk.Id)
	}

	assert.Equal(t, []string{"t1", "t2", "t3"}, ids)
}

func TestEmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, tasks.Tasks{})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, 10)

	got, err := c.ListTasks(context.Background(), "L")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestHTTPErrorsSurface(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusForbidden} {
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, `{"error":{"message":"nope"}}`, code)
			}))
			defer srv.Close()

			c := newTestClient(t, srv, 10)

			_, err := c.ListTasks(context.Background(), "L")
			require.Error(t, err)

			gerr, ok := errors.AsType[*googleapi.Error](err)
			require.True(t, ok)
			assert.Equal(t, code, gerr.Code)
		})
	}
}

func TestListTaskListsHTTPErrorSurfaces(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusForbidden} {
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, `{"error":{"message":"nope"}}`, code)
			}))
			defer srv.Close()

			c := newTestClient(t, srv, 10)

			_, err := c.ListTaskLists(context.Background())

			gerr, ok := errors.AsType[*googleapi.Error](err)
			require.True(t, ok)
			assert.Equal(t, code, gerr.Code)
		})
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
	require.ErrorIs(t, err, errPaginationLoop)
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
	require.ErrorIs(t, err, errTooManyPages)
	assert.Contains(t, err.Error(), "max 3 pages")
}

func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, tasks.Tasks{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	c := newTestClient(t, srv, 10)

	_, err := c.ListTasks(ctx, "L")
	require.ErrorIs(t, err, context.Canceled)

	_, err = c.ListTaskLists(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

// --- small local helpers (no external deps) ---

func ids(lists []*tasks.TaskList) []string {
	out := make([]string, 0, len(lists))
	for _, l := range lists {
		out = append(out, l.Id)
	}

	return out
}

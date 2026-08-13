// Package tasksclient wraps the Google Tasks API behind a small interface so
// the rest of the program (and its tests) never touch the network directly.
package tasksclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/api/option"
	tasks "google.golang.org/api/tasks/v1"
)

const (
	// pageSize is the per-request page size (the API caps this at 100).
	pageSize = 100
	// maxPages bounds pagination as a defense against a misbehaving server
	// that never stops returning a next-page token.
	maxPages = 1000
)

// Sentinel errors for pagination failures.
var (
	errPaginationLoop = errors.New("pagination loop detected")
	errTooManyPages   = errors.New("exceeded maximum page count")
)

// Client is a thin, paginating wrapper around *tasks.Service.
type Client struct {
	svc      *tasks.Service
	maxPages int
}

// New builds a Client from an already-authorized *http.Client. Because it uses
// option.WithHTTPClient, the API library uses the client verbatim and performs
// no ambient credential (ADC) discovery.
func New(ctx context.Context, httpClient *http.Client, userAgent string) (*Client, error) {
	svc, err := tasks.NewService(ctx,
		option.WithHTTPClient(httpClient),
		option.WithUserAgent(userAgent),
	)
	if err != nil {
		return nil, fmt.Errorf("initializing tasks service: %w", err)
	}

	return &Client{svc: svc, maxPages: maxPages}, nil
}

// ListTaskLists returns every task list in the account, following pagination.
func (c *Client) ListTaskLists(ctx context.Context) ([]*tasks.TaskList, error) {
	return paginate(ctx, c.maxPages, "listing task lists",
		func(ctx context.Context, token string) (pageResult[*tasks.TaskList], error) {
			call := c.svc.Tasklists.List().MaxResults(pageSize).Context(ctx)
			if token != "" {
				call = call.PageToken(token)
			}

			resp, err := call.Do()
			if err != nil {
				return pageResult[*tasks.TaskList]{}, fmt.Errorf("listing task lists: %w", err)
			}

			return pageResult[*tasks.TaskList]{items: resp.Items, next: resp.NextPageToken}, nil
		})
}

// ListTasks returns every task in a list. All show* flags are set so completed,
// hidden, deleted and assigned tasks are all captured — that is the whole point
// of the export.
func (c *Client) ListTasks(ctx context.Context, listID string) ([]*tasks.Task, error) {
	what := fmt.Sprintf("listing tasks for list %q", listID)

	return paginate(ctx, c.maxPages, what,
		func(ctx context.Context, token string) (pageResult[*tasks.Task], error) {
			call := c.svc.Tasks.List(listID).
				ShowCompleted(true).
				ShowHidden(true).
				ShowDeleted(true).
				ShowAssigned(true).
				MaxResults(pageSize).
				Context(ctx)
			if token != "" {
				call = call.PageToken(token)
			}

			resp, err := call.Do()
			if err != nil {
				return pageResult[*tasks.Task]{}, fmt.Errorf("%s: %w", what, err)
			}

			return pageResult[*tasks.Task]{items: resp.Items, next: resp.NextPageToken}, nil
		})
}

// pageResult is one page returned by a fetch function: its items and the token
// for the next page ("" when there are no more pages).
type pageResult[T any] struct {
	items []T
	next  string
}

// paginate repeatedly calls fetch, following next-page tokens, concatenating
// items in order. It checks ctx between pages, caps the number of pages, and
// guards against a server that keeps returning the same next-page token.
func paginate[T any](
	ctx context.Context,
	maxPages int,
	what string,
	fetch func(ctx context.Context, token string) (pageResult[T], error),
) ([]T, error) {
	var out []T

	seen := make(map[string]struct{})
	token := ""

	for range maxPages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		page, err := fetch(ctx, token)
		if err != nil {
			return nil, err
		}

		out = append(out, page.items...)
		if page.next == "" {
			return out, nil
		}

		if _, ok := seen[page.next]; ok {
			return nil, fmt.Errorf("%s: %w", what, errPaginationLoop)
		}

		seen[page.next] = struct{}{}
		token = page.next
	}

	return nil, fmt.Errorf("%s: %w (max %d pages)", what, errTooManyPages, maxPages)
}

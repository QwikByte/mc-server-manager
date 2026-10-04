package operation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

func spec() Spec {
	return Spec{Kind: "server.create", Subject: "Lobby", Steps: []string{"image", "container"}, Status: http.StatusCreated,
		Timeout: time.Minute, Visible: func(access.Grants) bool { return false }}
}

func run(ops *Operations, task Task) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	ops.Run(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil), spec(), task)
	return rec
}

// Operations that end right away are answered like ordinary requests.
func TestQuick(t *testing.T) {
	ops := New(time.Minute)
	rec := run(ops, func(context.Context) (any, error) { return map[string]string{"id": "s1"}, nil })
	if rec.Code != http.StatusCreated || rec.Body.String() != "{\"id\":\"s1\"}\n" {
		t.Fatalf("answer %d: %s", rec.Code, rec.Body)
	}
	rec = run(ops, func(context.Context) (any, error) {
		return nil, httpapi.Errorf(http.StatusConflict, "Port 25565 is used.")
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("answer %d: %s", rec.Code, rec.Body)
	}
	if err := ops.CheckIdle(); err != nil {
		t.Fatal(err)
	}
}

// Others are answered with 202 Accepted, and go on with their steps and progress; steps
// that weren't planned are added.
func TestBackground(t *testing.T) {
	ops := New(0)
	proceed := make(chan struct{})
	rec := run(ops, func(ctx context.Context) (any, error) {
		Step(ctx, "image")
		Count(ctx, 50, 100, "bytes")
		Step(ctx, "plugins")
		Target(ctx, "n1", "s1")
		<-proceed
		return "done", nil
	})
	var op Operation
	if err := json.Unmarshal(rec.Body.Bytes(), &op); err != nil || rec.Code != http.StatusAccepted || op.FinishedAt != nil {
		t.Fatalf("answer %d: %s", rec.Code, rec.Body)
	}
	if ops.CheckIdle() == nil {
		t.Fatal("an operation in progress allows restarting")
	}
	waitFor(t, ops, func(op Operation) bool { return op.ServerID == "s1" })
	got := ops.List(0, access.Grants{})[0]
	if !slices.Equal(got.Steps, []string{"image", "container", "plugins"}) || got.Step != 2 || got.Done != 0 || got.Unit != "" {
		t.Fatalf("operation = %+v", got)
	}
	close(proceed)
	got = waitFor(t, ops, func(op Operation) bool { return op.FinishedAt != nil })
	if got.Result != "done" || got.Error != "" || got.User != "" {
		t.Fatalf("operation = %+v", got)
	}
}

// Users see the operations they started, and those of others they may see.
func TestList(t *testing.T) {
	ops := New(time.Minute)
	run(ops, func(context.Context) (any, error) { return nil, nil })
	if len(ops.List(1, access.Grants{})) != 0 {
		t.Fatal("another user sees the operation")
	}
	if len(ops.List(1, access.Admin())) != 0 {
		t.Fatal("the operation isn't visible to others, yet listed")
	}
	if len(ops.List(0, access.Grants{})) != 1 {
		t.Fatal("the user doesn't see the own operation")
	}
}

func waitFor(t *testing.T, ops *Operations, ok func(Operation) bool) Operation {
	t.Helper()
	for range 500 {
		if list := ops.List(0, access.Grants{}); len(list) > 0 && ok(list[0]) {
			return list[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the operation didn't get there")
	return Operation{}
}

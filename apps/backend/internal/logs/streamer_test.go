package logs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/despliegue-facilito/backend/internal/build"
	"github.com/despliegue-facilito/backend/internal/middleware"
	"github.com/despliegue-facilito/backend/internal/projects"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// --- Mocks ---

type mockProjectStore struct {
	project *projects.Project
	err     error
}

func (m *mockProjectStore) Create(ctx context.Context, userID string, project projects.Project) error {
	return nil
}

func (m *mockProjectStore) ListByUser(ctx context.Context, userID string) ([]projects.Project, error) {
	return nil, nil
}

func (m *mockProjectStore) GetByID(ctx context.Context, projectID string) (*projects.Project, error) {
	return m.project, m.err
}

func (m *mockProjectStore) GetByRepoURL(ctx context.Context, repoURL string) (*projects.Project, error) {
	return nil, nil
}

func (m *mockProjectStore) UpdateStatus(ctx context.Context, projectID string, status string, reason string) error {
	return nil
}

type mockBuildService struct {
	status build.BuildStatus
	err    error
	calls  int
}

func (m *mockBuildService) TriggerBuild(ctx context.Context, project projects.Project, token string) (string, error) {
	return "", nil
}

func (m *mockBuildService) GetBuildStatus(ctx context.Context, buildID string) (build.BuildStatus, error) {
	m.calls++
	return m.status, m.err
}

type mockCWLogs struct {
	events    []cwltypes.OutputLogEvent
	nextToken *string
	err       error
}

func (m *mockCWLogs) GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &cloudwatchlogs.GetLogEventsOutput{
		Events:          m.events,
		NextForwardToken: m.nextToken,
	}, nil
}

// --- Tests ---

func TestHandleLogStream_Unauthorized(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{},
	}

	// Request without user in context
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/logs", nil)
	rec := httptest.NewRecorder()

	h.HandleLogStream(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandleLogStream_ProjectNotFound(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{project: nil},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/logs", nil)
	ctx := middleware.SetUserIDInContext(req.Context(), "user-1")

	// Set chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("project_id", "proj-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleLogStream(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleLogStream_ForbiddenUser(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{
			project: &projects.Project{
				ProjectID: "proj-1",
				UserID:    "other-user",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/logs", nil)
	ctx := middleware.SetUserIDInContext(req.Context(), "user-1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("project_id", "proj-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleLogStream(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestHandleLogStream_NoBuild(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{
			project: &projects.Project{
				ProjectID:      "proj-1",
				UserID:         "user-1",
				CurrentBuildID: nil,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/logs", nil)
	ctx := middleware.SetUserIDInContext(req.Context(), "user-1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("project_id", "proj-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleLogStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestStreamLogs_SendsLogsAndCompletion(t *testing.T) {
	buildID := "build-123"
	ts := int64(1700000000000)

	cwMock := &mockCWLogs{
		events: []cwltypes.OutputLogEvent{
			{Timestamp: &ts, Message: aws.String("Installing deps...")},
			{Timestamp: &ts, Message: aws.String("Build succeeded")},
		},
		nextToken: aws.String("token-1"),
	}

	buildMock := &mockBuildService{
		status: build.BuildStatusSucceeded,
	}

	h := &Handler{
		Store: &mockProjectStore{
			project: &projects.Project{
				ProjectID:      "proj-1",
				UserID:         "user-1",
				CurrentBuildID: &buildID,
			},
		},
		BuildService: buildMock,
		CWLogs:       cwMock,
		LogGroupName: "/aws/codebuild/test",
		PollInterval: 50 * time.Millisecond,
	}

	// Create a test server with the WebSocket handler
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.SetUserIDInContext(r.Context(), "user-1")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Get("/api/v1/projects/{project_id}/logs", h.HandleLogStream)

	server := httptest.NewServer(router)
	defer server.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/projects/proj-1/logs"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer ws.Close()

	// Read messages
	var messages []map[string]interface{}
	for {
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		messages = append(messages, msg)

		// Stop reading after completion message
		if msg["type"] == "complete" {
			break
		}
	}

	// Verify we received log messages and a completion message
	if len(messages) < 3 {
		t.Fatalf("expected at least 3 messages (2 logs + completion), got %d", len(messages))
	}

	// Check first log message
	firstLog := messages[0]
	if firstLog["type"] != "log" {
		t.Errorf("expected first message type 'log', got '%s'", firstLog["type"])
	}
	if firstLog["message"] != "Installing deps..." {
		t.Errorf("unexpected first log message: %v", firstLog["message"])
	}

	// Check completion message (last)
	lastMsg := messages[len(messages)-1]
	if lastMsg["type"] != "complete" {
		t.Errorf("expected last message type 'complete', got '%s'", lastMsg["type"])
	}
	if lastMsg["status"] != "succeeded" {
		t.Errorf("expected status 'succeeded', got '%s'", lastMsg["status"])
	}
}

func TestStreamLogs_SendsErrorOnCWFailure(t *testing.T) {
	buildID := "build-456"

	cwMock := &mockCWLogs{
		err: context.DeadlineExceeded,
	}

	// First call returns in_progress with CW error, second call returns succeeded
	callCount := 0
	buildMock := &mockBuildService{
		status: build.BuildStatusInProgress,
	}

	h := &Handler{
		Store: &mockProjectStore{
			project: &projects.Project{
				ProjectID:      "proj-1",
				UserID:         "user-1",
				CurrentBuildID: &buildID,
			},
		},
		BuildService: buildMock,
		CWLogs:       cwMock,
		LogGroupName: "/aws/codebuild/test",
		PollInterval: 50 * time.Millisecond,
	}

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.SetUserIDInContext(r.Context(), "user-1")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Get("/api/v1/projects/{project_id}/logs", h.HandleLogStream)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/projects/proj-1/logs"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer ws.Close()

	// After first poll we should get an error message
	// Then make the build finish on second poll
	go func() {
		time.Sleep(80 * time.Millisecond)
		buildMock.status = build.BuildStatusFailed
		cwMock.err = nil // Clear error for final fetch
		callCount++ // suppress unused var warning
	}()

	var gotError bool
	var gotComplete bool
	for i := 0; i < 20; i++ {
		ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]interface{}
		json.Unmarshal(msgBytes, &msg)

		if msg["type"] == "error" {
			gotError = true
		}
		if msg["type"] == "complete" {
			gotComplete = true
			break
		}
	}

	if !gotError {
		t.Error("expected to receive an error message from CloudWatch failure")
	}
	if !gotComplete {
		t.Error("expected to receive a completion message after build finishes")
	}
	_ = callCount
}

func TestFetchLogEvents_ReturnsChunks(t *testing.T) {
	ts1 := int64(1700000000000)
	ts2 := int64(1700000001000)

	cwMock := &mockCWLogs{
		events: []cwltypes.OutputLogEvent{
			{Timestamp: &ts1, Message: aws.String("line 1")},
			{Timestamp: &ts2, Message: aws.String("line 2")},
		},
		nextToken: aws.String("next-token"),
	}

	h := &Handler{
		CWLogs:       cwMock,
		LogGroupName: "/aws/codebuild/test",
	}

	chunks, token, err := h.fetchLogEvents(context.Background(), "stream-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	if chunks[0].Type != "log" {
		t.Errorf("expected type 'log', got '%s'", chunks[0].Type)
	}
	if chunks[0].Message != "line 1" {
		t.Errorf("expected message 'line 1', got '%s'", chunks[0].Message)
	}
	if chunks[1].Message != "line 2" {
		t.Errorf("expected message 'line 2', got '%s'", chunks[1].Message)
	}

	if token == nil || *token != "next-token" {
		t.Errorf("expected next token 'next-token', got %v", token)
	}
}

func TestFetchLogEvents_Error(t *testing.T) {
	cwMock := &mockCWLogs{
		err: context.DeadlineExceeded,
	}

	h := &Handler{
		CWLogs:       cwMock,
		LogGroupName: "/aws/codebuild/test",
	}

	_, _, err := h.fetchLogEvents(context.Background(), "stream-1", nil)
	if err == nil {
		t.Error("expected error from CloudWatch, got nil")
	}
}

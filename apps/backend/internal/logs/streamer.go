package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/despliegue-facilito/backend/internal/build"
	"github.com/despliegue-facilito/backend/internal/middleware"
	"github.com/despliegue-facilito/backend/internal/projects"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// CloudWatchLogsAPI is a subset of the CloudWatch Logs SDK client used for testability.
type CloudWatchLogsAPI interface {
	GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error)
}

// LogChunkMessage is sent for each log line retrieved from CloudWatch.
type LogChunkMessage struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// LogCompletionMessage is sent when the build finishes.
type LogCompletionMessage struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// LogErrorMessage is sent when a CloudWatch retrieval error occurs.
type LogErrorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Handler handles WebSocket log streaming requests.
type Handler struct {
	Store        projects.ProjectStore
	BuildService build.BuildService
	CWLogs       CloudWatchLogsAPI
	LogGroupName string
	PollInterval time.Duration
}

// NewHandler creates a new log streaming Handler.
func NewHandler(store projects.ProjectStore, buildSvc build.BuildService, cwLogs CloudWatchLogsAPI) *Handler {
	logGroup := os.Getenv("CODEBUILD_LOG_GROUP")
	if logGroup == "" {
		logGroup = "/aws/codebuild/despliegue-facilito-builder"
	}

	return &Handler{
		Store:        store,
		BuildService: buildSvc,
		CWLogs:       cwLogs,
		LogGroupName: logGroup,
		PollInterval: 2 * time.Second,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now; tighten in production
	},
}

// HandleLogStream upgrades the HTTP connection to WebSocket and streams build logs.
func (h *Handler) HandleLogStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	projectID := chi.URLParam(r, "project_id")
	if projectID == "" {
		http.Error(w, "missing project_id", http.StatusBadRequest)
		return
	}

	// Verify user owns the project
	project, err := h.Store.GetByID(ctx, projectID)
	if err != nil {
		http.Error(w, "failed to retrieve project", http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if project.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Check that the project has an active build
	if project.CurrentBuildID == nil || *project.CurrentBuildID == "" {
		http.Error(w, "no active build for this project", http.StatusBadRequest)
		return
	}
	buildID := *project.CurrentBuildID

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("logs: websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Stream logs
	h.streamLogs(ctx, conn, buildID)
}

// streamLogs polls CloudWatch Logs and pushes log chunks to the WebSocket client.
func (h *Handler) streamLogs(ctx context.Context, conn *websocket.Conn, buildID string) {
	logStreamName := buildID
	var nextToken *string

	ticker := time.NewTicker(h.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Poll CloudWatch Logs
			chunks, token, err := h.fetchLogEvents(ctx, logStreamName, nextToken)
			if err != nil {
				// Send error message and retry on next cycle
				errMsg := LogErrorMessage{
					Type:    "error",
					Message: fmt.Sprintf("failed to retrieve logs: %v", err),
				}
				if writeErr := writeJSON(conn, errMsg); writeErr != nil {
					log.Printf("logs: failed to send error message: %v", writeErr)
					return
				}
			} else {
				nextToken = token
				// Send log chunks to client
				for _, chunk := range chunks {
					if writeErr := writeJSON(conn, chunk); writeErr != nil {
						log.Printf("logs: failed to send log chunk: %v", writeErr)
						return
					}
				}
			}

			// Check build status
			status, err := h.BuildService.GetBuildStatus(ctx, buildID)
			if err != nil {
				log.Printf("logs: failed to get build status: %v", err)
				continue
			}

			if status == build.BuildStatusSucceeded || status == build.BuildStatusFailed {
				// Do one final log fetch to capture any remaining logs
				finalChunks, _, fetchErr := h.fetchLogEvents(ctx, logStreamName, nextToken)
				if fetchErr == nil {
					for _, chunk := range finalChunks {
						if writeErr := writeJSON(conn, chunk); writeErr != nil {
							log.Printf("logs: failed to send final log chunk: %v", writeErr)
							return
						}
					}
				}

				// Send completion message
				completionMsg := LogCompletionMessage{
					Type:   "complete",
					Status: string(status),
				}
				if writeErr := writeJSON(conn, completionMsg); writeErr != nil {
					log.Printf("logs: failed to send completion message: %v", writeErr)
				}
				return
			}
		}
	}
}

// fetchLogEvents retrieves log events from CloudWatch Logs.
func (h *Handler) fetchLogEvents(ctx context.Context, logStreamName string, nextToken *string) ([]LogChunkMessage, *string, error) {
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(h.LogGroupName),
		LogStreamName: aws.String(logStreamName),
		StartFromHead: aws.Bool(true),
	}
	if nextToken != nil {
		input.NextToken = nextToken
	}

	output, err := h.CWLogs.GetLogEvents(ctx, input)
	if err != nil {
		return nil, nextToken, err
	}

	var chunks []LogChunkMessage
	for _, event := range output.Events {
		msg := ""
		if event.Message != nil {
			msg = *event.Message
		}
		ts := time.UnixMilli(0)
		if event.Timestamp != nil {
			ts = time.UnixMilli(*event.Timestamp)
		}
		chunks = append(chunks, LogChunkMessage{
			Type:      "log",
			Timestamp: ts.UTC().Format(time.RFC3339),
			Message:   msg,
		})
	}

	return chunks, output.NextForwardToken, nil
}

// writeJSON marshals the given value and writes it as a WebSocket text message.
func writeJSON(conn *websocket.Conn, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

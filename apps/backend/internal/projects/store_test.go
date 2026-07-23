package projects

import (
	"testing"
	"time"
)

func TestProject_FieldMapping(t *testing.T) {
	t.Run("Project struct has all required fields", func(t *testing.T) {
		now := time.Now().UTC()
		reason := "build timeout"
		buildID := "build-123"

		p := Project{
			ProjectID:        "proj-abc",
			UserID:           "user-123",
			RepoURL:          "https://github.com/user/repo",
			Framework:        "react",
			BuildCommand:     "npm run build",
			OutputDir:        "dist",
			DeploymentStatus: string(StatusPending),
			FailureReason:    &reason,
			CurrentBuildID:   &buildID,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if p.ProjectID != "proj-abc" {
			t.Errorf("expected ProjectID 'proj-abc', got '%s'", p.ProjectID)
		}
		if p.UserID != "user-123" {
			t.Errorf("expected UserID 'user-123', got '%s'", p.UserID)
		}
		if p.RepoURL != "https://github.com/user/repo" {
			t.Errorf("expected RepoURL 'https://github.com/user/repo', got '%s'", p.RepoURL)
		}
		if p.Framework != "react" {
			t.Errorf("expected Framework 'react', got '%s'", p.Framework)
		}
		if p.BuildCommand != "npm run build" {
			t.Errorf("expected BuildCommand 'npm run build', got '%s'", p.BuildCommand)
		}
		if p.OutputDir != "dist" {
			t.Errorf("expected OutputDir 'dist', got '%s'", p.OutputDir)
		}
		if p.DeploymentStatus != "pending" {
			t.Errorf("expected DeploymentStatus 'pending', got '%s'", p.DeploymentStatus)
		}
		if p.FailureReason == nil || *p.FailureReason != "build timeout" {
			t.Errorf("expected FailureReason 'build timeout', got %v", p.FailureReason)
		}
		if p.CurrentBuildID == nil || *p.CurrentBuildID != "build-123" {
			t.Errorf("expected CurrentBuildID 'build-123', got %v", p.CurrentBuildID)
		}
		if p.CreatedAt != now {
			t.Errorf("expected CreatedAt %v, got %v", now, p.CreatedAt)
		}
		if p.UpdatedAt != now {
			t.Errorf("expected UpdatedAt %v, got %v", now, p.UpdatedAt)
		}
	})

	t.Run("nullable fields can be nil", func(t *testing.T) {
		p := Project{
			ProjectID:        "proj-xyz",
			UserID:           "user-456",
			RepoURL:          "https://github.com/user/app",
			Framework:        "vue",
			BuildCommand:     "npm run build",
			OutputDir:        "dist",
			DeploymentStatus: string(StatusDeployed),
			FailureReason:    nil,
			CurrentBuildID:   nil,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}

		if p.FailureReason != nil {
			t.Errorf("expected FailureReason nil, got %v", p.FailureReason)
		}
		if p.CurrentBuildID != nil {
			t.Errorf("expected CurrentBuildID nil, got %v", p.CurrentBuildID)
		}
	})
}

func TestDeploymentStatus_Constants(t *testing.T) {
	tests := []struct {
		name     string
		status   DeploymentStatus
		expected string
	}{
		{"pending status", StatusPending, "pending"},
		{"building status", StatusBuilding, "building"},
		{"deployed status", StatusDeployed, "deployed"},
		{"failed status", StatusFailed, "failed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.status) != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, string(tc.status))
			}
		})
	}
}

func TestNewFirestoreProjectStore(t *testing.T) {
	// NewFirestoreProjectStore should not panic with nil client
	// (nil is valid for testing purposes; actual operations would fail)
	store := NewFirestoreProjectStore(nil)
	if store == nil {
		t.Error("expected non-nil store")
	}
	if store.client != nil {
		t.Error("expected nil client in store")
	}
}

func TestProjectStore_InterfaceCompliance(t *testing.T) {
	// Verify that FirestoreProjectStore implements the ProjectStore interface.
	var _ ProjectStore = (*FirestoreProjectStore)(nil)
}

func TestProject_FrameworkValues(t *testing.T) {
	// Test that all valid framework values can be assigned
	frameworks := []struct {
		name         string
		buildCommand string
		outputDir    string
	}{
		{"react", "npm run build", "dist"},
		{"angular", "npm run build", "dist/{project-name}"},
		{"vue", "npm run build", "dist"},
		{"astro", "npm run build", "dist"},
		{"html", "", "."},
	}

	for _, fw := range frameworks {
		t.Run(fw.name, func(t *testing.T) {
			p := Project{
				Framework:    fw.name,
				BuildCommand: fw.buildCommand,
				OutputDir:    fw.outputDir,
			}
			if p.Framework != fw.name {
				t.Errorf("expected framework '%s', got '%s'", fw.name, p.Framework)
			}
			if p.BuildCommand != fw.buildCommand {
				t.Errorf("expected build command '%s', got '%s'", fw.buildCommand, p.BuildCommand)
			}
			if p.OutputDir != fw.outputDir {
				t.Errorf("expected output dir '%s', got '%s'", fw.outputDir, p.OutputDir)
			}
		})
	}
}

func TestProject_StatusTransitions(t *testing.T) {
	// Test valid status transition sequences
	tests := []struct {
		name   string
		from   DeploymentStatus
		to     DeploymentStatus
		reason string
	}{
		{"pending to building", StatusPending, StatusBuilding, ""},
		{"building to deployed", StatusBuilding, StatusDeployed, ""},
		{"building to failed", StatusBuilding, StatusFailed, "build timeout"},
		{"failed to building (retry)", StatusFailed, StatusBuilding, ""},
		{"deployed to building (redeploy)", StatusDeployed, StatusBuilding, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Project{
				DeploymentStatus: string(tc.from),
			}

			// Simulate transition
			p.DeploymentStatus = string(tc.to)
			if tc.reason != "" {
				p.FailureReason = &tc.reason
			} else {
				p.FailureReason = nil
			}

			if p.DeploymentStatus != string(tc.to) {
				t.Errorf("expected status '%s', got '%s'", string(tc.to), p.DeploymentStatus)
			}
			if tc.reason != "" && (p.FailureReason == nil || *p.FailureReason != tc.reason) {
				t.Errorf("expected failure reason '%s', got %v", tc.reason, p.FailureReason)
			}
			if tc.reason == "" && p.FailureReason != nil {
				t.Errorf("expected nil failure reason, got '%s'", *p.FailureReason)
			}
		})
	}
}

func TestCollectionName(t *testing.T) {
	if collectionName != "projects" {
		t.Errorf("expected collection name 'projects', got '%s'", collectionName)
	}
}

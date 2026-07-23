package projects

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

// DeploymentStatus represents the status of a project deployment.
type DeploymentStatus string

const (
	StatusPending  DeploymentStatus = "pending"
	StatusBuilding DeploymentStatus = "building"
	StatusDeployed DeploymentStatus = "deployed"
	StatusFailed   DeploymentStatus = "failed"
)

const collectionName = "projects"

// Project represents a user-owned deployment project stored in Firestore.
type Project struct {
	ProjectID        string    `firestore:"projectId" json:"projectId"`
	UserID           string    `firestore:"userId" json:"userId"`
	RepoURL          string    `firestore:"repoUrl" json:"repoUrl"`
	Subdomain        string    `firestore:"subdomain" json:"subdomain"`
	DeployURL        string    `firestore:"deployUrl" json:"deployUrl"`
	Framework        string    `firestore:"framework" json:"framework"`
	PackageManager   string    `firestore:"packageManager" json:"packageManager"`
	BuildCommand     string    `firestore:"buildCommand" json:"buildCommand"`
	OutputDir        string    `firestore:"outputDir" json:"outputDir"`
	DeploymentStatus string    `firestore:"deploymentStatus" json:"deploymentStatus"`
	FailureReason    *string   `firestore:"failureReason" json:"failureReason,omitempty"`
	CurrentBuildID   *string   `firestore:"currentBuildId" json:"currentBuildId,omitempty"`
	DistributionID   string    `firestore:"distributionId" json:"distributionId,omitempty"`
	CreatedAt        time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time `firestore:"updatedAt" json:"updatedAt"`
}

// ProjectStore defines the interface for project persistence operations.
type ProjectStore interface {
	Create(ctx context.Context, userID string, project *Project) error
	ListByUser(ctx context.Context, userID string) ([]Project, error)
	GetByID(ctx context.Context, projectID string) (*Project, error)
	GetByRepoURL(ctx context.Context, repoURL string) (*Project, error)
	GetBySubdomain(ctx context.Context, subdomain string) (*Project, error)
	UpdateStatus(ctx context.Context, projectID string, status string, reason string) error
	UpdateDeployInfo(ctx context.Context, projectID, deployURL, distributionID string) error
	Delete(ctx context.Context, projectID string) error
}

// FirestoreProjectStore implements ProjectStore using Google Cloud Firestore.
type FirestoreProjectStore struct {
	client *firestore.Client
}

// NewFirestoreProjectStore creates a new FirestoreProjectStore with the given Firestore client.
func NewFirestoreProjectStore(client *firestore.Client) *FirestoreProjectStore {
	return &FirestoreProjectStore{client: client}
}

// Create stores a new project in Firestore with a generated UUID and timestamps.
func (s *FirestoreProjectStore) Create(ctx context.Context, userID string, project *Project) error {
	project.ProjectID = uuid.New().String()
	project.UserID = userID
	project.DeploymentStatus = string(StatusPending)
	now := time.Now().UTC()
	project.CreatedAt = now
	project.UpdatedAt = now

	_, err := s.client.Collection(collectionName).Doc(project.ProjectID).Set(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	return nil
}

// ListByUser returns all projects belonging to the specified user.
func (s *FirestoreProjectStore) ListByUser(ctx context.Context, userID string) ([]Project, error) {
	iter := s.client.Collection(collectionName).Where("userId", "==", userID).Documents(ctx)
	defer iter.Stop()

	var projects []Project
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate projects: %w", err)
		}

		var p Project
		if err := doc.DataTo(&p); err != nil {
			return nil, fmt.Errorf("failed to decode project: %w", err)
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// GetByID finds a project by its ID. Returns nil if not found.
func (s *FirestoreProjectStore) GetByID(ctx context.Context, projectID string) (*Project, error) {
	doc, err := s.client.Collection(collectionName).Doc(projectID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get project by ID: %w", err)
	}
	if !doc.Exists() {
		return nil, nil
	}

	var p Project
	if err := doc.DataTo(&p); err != nil {
		return nil, fmt.Errorf("failed to decode project: %w", err)
	}

	return &p, nil
}

// GetByRepoURL finds a project by its repository URL. Returns nil if not found.
func (s *FirestoreProjectStore) GetByRepoURL(ctx context.Context, repoURL string) (*Project, error) {
	iter := s.client.Collection(collectionName).Where("repoUrl", "==", repoURL).Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query project by repo URL: %w", err)
	}

	var p Project
	if err := doc.DataTo(&p); err != nil {
		return nil, fmt.Errorf("failed to decode project: %w", err)
	}

	return &p, nil
}

// GetBySubdomain finds a project by its subdomain. Returns nil if not found.
func (s *FirestoreProjectStore) GetBySubdomain(ctx context.Context, subdomain string) (*Project, error) {
	iter := s.client.Collection(collectionName).Where("subdomain", "==", subdomain).Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query project by subdomain: %w", err)
	}

	var p Project
	if err := doc.DataTo(&p); err != nil {
		return nil, fmt.Errorf("failed to decode project: %w", err)
	}

	return &p, nil
}

// UpdateStatus updates the deployment status (and optionally the failure reason) of a project.
func (s *FirestoreProjectStore) UpdateStatus(ctx context.Context, projectID string, status string, reason string) error {
	updates := []firestore.Update{
		{Path: "deploymentStatus", Value: status},
		{Path: "updatedAt", Value: time.Now().UTC()},
	}

	if reason != "" {
		updates = append(updates, firestore.Update{Path: "failureReason", Value: reason})
	} else {
		updates = append(updates, firestore.Update{Path: "failureReason", Value: nil})
	}

	_, err := s.client.Collection(collectionName).Doc(projectID).Update(ctx, updates)
	if err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}
	return nil
}

// Delete removes a project from Firestore by its ID.
func (s *FirestoreProjectStore) Delete(ctx context.Context, projectID string) error {
	_, err := s.client.Collection(collectionName).Doc(projectID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

// UpdateDeployInfo stores the CloudFront deploy URL and distribution ID.
func (s *FirestoreProjectStore) UpdateDeployInfo(ctx context.Context, projectID, deployURL, distributionID string) error {
	updates := []firestore.Update{
		{Path: "deployUrl", Value: deployURL},
		{Path: "distributionId", Value: distributionID},
		{Path: "updatedAt", Value: time.Now().UTC()},
	}
	_, err := s.client.Collection(collectionName).Doc(projectID).Update(ctx, updates)
	if err != nil {
		return fmt.Errorf("failed to update deploy info: %w", err)
	}
	return nil
}

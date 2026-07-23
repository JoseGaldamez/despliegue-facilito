package domains

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

const collectionName = "domains"

// DomainStore defines the interface for domain mapping persistence.
type DomainStore interface {
	Add(ctx context.Context, projectID string, hostname string) (Domain, error)
	ListByProject(ctx context.Context, projectID string) ([]Domain, error)
	Delete(ctx context.Context, projectID string, domainID string) error
	GetByHostname(ctx context.Context, hostname string) (*Domain, error)
}

// FirestoreDomainStore implements DomainStore using Google Cloud Firestore.
type FirestoreDomainStore struct {
	client *firestore.Client
}

// NewFirestoreDomainStore creates a new FirestoreDomainStore with the given Firestore client.
func NewFirestoreDomainStore(client *firestore.Client) *FirestoreDomainStore {
	return &FirestoreDomainStore{client: client}
}

// Add creates a new domain mapping in Firestore.
func (s *FirestoreDomainStore) Add(ctx context.Context, projectID string, hostname string) (Domain, error) {
	domain := Domain{
		DomainID:  uuid.New().String(),
		ProjectID: projectID,
		Hostname:  hostname,
		CreatedAt: time.Now().UTC(),
	}

	_, err := s.client.Collection(collectionName).Doc(domain.DomainID).Set(ctx, domain)
	if err != nil {
		return Domain{}, fmt.Errorf("failed to add domain: %w", err)
	}

	return domain, nil
}

// ListByProject returns all domain mappings for the specified project.
func (s *FirestoreDomainStore) ListByProject(ctx context.Context, projectID string) ([]Domain, error) {
	iter := s.client.Collection(collectionName).Where("projectId", "==", projectID).Documents(ctx)
	defer iter.Stop()

	var domains []Domain
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate domains: %w", err)
		}

		var d Domain
		if err := doc.DataTo(&d); err != nil {
			return nil, fmt.Errorf("failed to decode domain: %w", err)
		}
		domains = append(domains, d)
	}

	return domains, nil
}

// Delete removes a domain mapping by its ID for the given project.
func (s *FirestoreDomainStore) Delete(ctx context.Context, projectID string, domainID string) error {
	_, err := s.client.Collection(collectionName).Doc(domainID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}
	return nil
}

// GetByHostname finds a domain mapping by hostname. Returns nil if not found.
func (s *FirestoreDomainStore) GetByHostname(ctx context.Context, hostname string) (*Domain, error) {
	iter := s.client.Collection(collectionName).Where("hostname", "==", hostname).Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query domain by hostname: %w", err)
	}

	var d Domain
	if err := doc.DataTo(&d); err != nil {
		return nil, fmt.Errorf("failed to decode domain: %w", err)
	}

	return &d, nil
}

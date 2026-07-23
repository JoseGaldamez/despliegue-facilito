package domains

import "time"

// Domain represents a custom domain mapping for a project.
type Domain struct {
	DomainID  string    `firestore:"domainId" json:"domainId"`
	ProjectID string    `firestore:"projectId" json:"projectId"`
	Hostname  string    `firestore:"hostname" json:"hostname"`
	CreatedAt time.Time `firestore:"createdAt" json:"createdAt"`
}

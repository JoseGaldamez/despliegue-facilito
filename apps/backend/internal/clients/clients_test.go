package clients

import (
	"testing"
)

func TestClientsStructHasRequiredFields(t *testing.T) {
	// Verify the Clients struct has all expected fields by instantiating it.
	c := &Clients{}

	if c.Firestore != nil {
		t.Error("expected nil Firestore client on zero-value struct")
	}
	if c.FirebaseAuth != nil {
		t.Error("expected nil FirebaseAuth client on zero-value struct")
	}
	if c.CodeBuild != nil {
		t.Error("expected nil CodeBuild client on zero-value struct")
	}
	if c.S3 != nil {
		t.Error("expected nil S3 client on zero-value struct")
	}
	if c.CloudWatchLogs != nil {
		t.Error("expected nil CloudWatchLogs client on zero-value struct")
	}
}

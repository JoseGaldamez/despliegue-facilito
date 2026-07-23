package clients

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Clients holds all external service clients used by the backend.
type Clients struct {
	Firestore      *firestore.Client
	FirebaseAuth   *auth.Client
	CodeBuild      *codebuild.Client
	S3             *s3.Client
	CloudFront     *cloudfront.Client
	CloudWatchLogs *cloudwatchlogs.Client
}

// Init initializes Firestore, Firebase Auth, and AWS SDK clients.
// It terminates the process with exit(1) if any initialization fails.
func Init(ctx context.Context) *Clients {
	// Initialize Firebase App
	firebaseApp, err := firebase.NewApp(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase app: %v", err)
	}

	// Initialize Firestore client
	firestoreClient, err := firebaseApp.Firestore(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Firestore client: %v", err)
	}

	// Initialize Firebase Auth client
	authClient, err := firebaseApp.Auth(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase Auth client: %v", err)
	}

	// Initialize AWS SDK configuration
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS SDK configuration: %v", err)
	}

	// Create AWS service clients
	codebuildClient := codebuild.NewFromConfig(awsCfg)
	s3Client := s3.NewFromConfig(awsCfg)
	cloudwatchLogsClient := cloudwatchlogs.NewFromConfig(awsCfg)
	cloudfrontClient := cloudfront.NewFromConfig(awsCfg)

	log.Println("All clients initialized successfully")

	return &Clients{
		Firestore:      firestoreClient,
		FirebaseAuth:   authClient,
		CodeBuild:      codebuildClient,
		S3:             s3Client,
		CloudFront:     cloudfrontClient,
		CloudWatchLogs: cloudwatchLogsClient,
	}
}

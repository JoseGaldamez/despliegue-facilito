package cdn

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/despliegue-facilito/backend/internal/projects"
)

// CloudFrontAPI is the subset of the CloudFront client we use.
type CloudFrontAPI interface {
	CreateDistribution(ctx context.Context, params *cloudfront.CreateDistributionInput, optFns ...func(*cloudfront.Options)) (*cloudfront.CreateDistributionOutput, error)
	GetDistribution(ctx context.Context, params *cloudfront.GetDistributionInput, optFns ...func(*cloudfront.Options)) (*cloudfront.GetDistributionOutput, error)
}

// Service manages CloudFront distribution provisioning per project.
type Service struct {
	client   CloudFrontAPI
	store    projects.ProjectStore
	s3Bucket string
	oacID    string // Origin Access Control ID
	region   string
}

// NewService creates a CDN service.
func NewService(client CloudFrontAPI, store projects.ProjectStore) *Service {
	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		s3Bucket = "despliegue-facilito-apps"
	}
	oacID := os.Getenv("CLOUDFRONT_OAC_ID")
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	return &Service{
		client:   client,
		store:    store,
		s3Bucket: s3Bucket,
		oacID:    oacID,
		region:   region,
	}
}

// ProvisionDistribution creates a new CloudFront distribution for a project
// pointing to its S3 folder /apps/{projectId}/.
func (s *Service) ProvisionDistribution(ctx context.Context, project projects.Project) error {
	if project.DistributionID != "" {
		// Already has a distribution, skip
		log.Printf("cdn: project %s already has distribution %s", project.ProjectID, project.DistributionID)
		return nil
	}

	originDomain := fmt.Sprintf("%s.s3.%s.amazonaws.com", s.s3Bucket, s.region)
	originID := fmt.Sprintf("S3-%s-%s", s.s3Bucket, project.ProjectID)
	callerRef := fmt.Sprintf("%s-%d", project.ProjectID, time.Now().Unix())
	originPath := fmt.Sprintf("/apps/%s", project.ProjectID)

	// Build origin - with OAC, S3OriginConfig must be present with empty identity
	origin := cftypes.Origin{
		Id:                    aws.String(originID),
		DomainName:            aws.String(originDomain),
		OriginPath:            aws.String(originPath),
		OriginAccessControlId: aws.String(s.oacID),
		S3OriginConfig: &cftypes.S3OriginConfig{
			OriginAccessIdentity: aws.String(""),
		},
	}

	input := &cloudfront.CreateDistributionInput{
		DistributionConfig: &cftypes.DistributionConfig{
			CallerReference:   aws.String(callerRef),
			Comment:           aws.String(fmt.Sprintf("Despliegue Facilito - %s", project.Subdomain)),
			Enabled:           aws.Bool(true),
			DefaultRootObject: aws.String("index.html"),
			Origins: &cftypes.Origins{
				Quantity: aws.Int32(1),
				Items:    []cftypes.Origin{origin},
			},
			DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
				TargetOriginId:       aws.String(originID),
				ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
				AllowedMethods: &cftypes.AllowedMethods{
					Quantity: aws.Int32(2),
					Items:    []cftypes.Method{cftypes.MethodGet, cftypes.MethodHead},
				},
				ForwardedValues: &cftypes.ForwardedValues{
					QueryString: aws.Bool(false),
					Cookies:     &cftypes.CookiePreference{Forward: cftypes.ItemSelectionNone},
				},
				MinTTL:     aws.Int64(0),
				DefaultTTL: aws.Int64(86400),
				MaxTTL:     aws.Int64(31536000),
				Compress:   aws.Bool(true),
			},
			CustomErrorResponses: &cftypes.CustomErrorResponses{
				Quantity: aws.Int32(2),
				Items: []cftypes.CustomErrorResponse{
					{
						ErrorCode:          aws.Int32(403),
						ResponseCode:       aws.String("200"),
						ResponsePagePath:   aws.String("/index.html"),
						ErrorCachingMinTTL: aws.Int64(10),
					},
					{
						ErrorCode:          aws.Int32(404),
						ResponseCode:       aws.String("200"),
						ResponsePagePath:   aws.String("/index.html"),
						ErrorCachingMinTTL: aws.Int64(10),
					},
				},
			},
		},
	}

	// If we have an OAC, use it (already set in origin above)

	output, err := s.client.CreateDistribution(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create CloudFront distribution: %w", err)
	}

	distributionID := aws.ToString(output.Distribution.Id)
	domainName := aws.ToString(output.Distribution.DomainName)
	deployURL := fmt.Sprintf("https://%s", domainName)

	log.Printf("cdn: created distribution %s (%s) for project %s, waiting for deployment...", distributionID, domainName, project.ProjectID)

	// Update the project with the deploy URL immediately (so UI can show it)
	if err := s.store.UpdateDeployInfo(ctx, project.ProjectID, deployURL, distributionID); err != nil {
		return fmt.Errorf("failed to update project deploy info: %w", err)
	}

	// Poll until distribution is fully deployed
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	timeout := time.After(15 * time.Minute)

	for {
		select {
		case <-ticker.C:
			resp, err := s.client.GetDistribution(ctx, &cloudfront.GetDistributionInput{
				Id: aws.String(distributionID),
			})
			if err != nil {
				log.Printf("cdn: error checking distribution %s status: %v", distributionID, err)
				continue
			}
			status := aws.ToString(resp.Distribution.Status)
			if status == "Deployed" {
				log.Printf("cdn: distribution %s is now deployed for project %s", distributionID, project.ProjectID)
				return nil
			}
			log.Printf("cdn: distribution %s status: %s", distributionID, status)
		case <-timeout:
			log.Printf("cdn: timeout waiting for distribution %s to deploy", distributionID)
			return nil
		}
	}
}

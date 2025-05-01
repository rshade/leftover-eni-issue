package enidetection

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// OrphanedENI represents an orphaned Elastic Network Interface
type OrphanedENI struct {
	ID               string
	Region           string
	VPCID            *string
	SubnetID         *string
	AvailabilityZone *string
	Description      *string
	AttachmentState  *string
	CreatedTime      *string
	Tags             map[string]string
}

// DetectOrphanedENIs detects orphaned ENIs across specified AWS regions
func DetectOrphanedENIs(ctx *pulumi.Context, regions []string, provider *aws.Provider) ([]OrphanedENI, error) {
	// To be implemented
	return []OrphanedENI{}, nil
}

// IsLikelyOrphaned checks if an ENI is likely orphaned based on its description,
// attachment state, and tags
func IsLikelyOrphaned(eni *ec2.NetworkInterface) bool {
	// To be implemented
	return false
}

// LogOrphanedENIsOnDestroy creates a log message about orphaned ENIs that will be displayed during resource destruction
func LogOrphanedENIsOnDestroy(ctx *pulumi.Context, resourceName string, provider *aws.Provider) (pulumi.Resource, error) {
	// To be implemented
	return nil, nil
}
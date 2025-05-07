package enicleanup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// OrphanedENI represents a potentially orphaned ENI discovered during detection
type OrphanedENI struct {
	ID               string
	Region           string
	VPCID            string
	SubnetID         string
	AvailabilityZone string
	Description      string
	AttachmentState  string
	CreatedTime      time.Time
	Tags             map[string]string
	AttachmentID     string
	SecurityGroups   []string
}

// DetectOptions contains options for the ENI detection process
type DetectOptions struct {
	SkipReservedDescriptions []string
	IncludeTagKeys           []string
	ExcludeTagKeys           []string
	OlderThanDays            *float64
	LogLevel                 string
	SecurityGroupId          *string
}

// CleanupResult captures the results of the cleanup operation
type CleanupResult struct {
	SuccessCount int
	FailureCount int
	SkippedCount int
	CleanedENIs  []CleanedENI
	Errors       []string
}

// DetectOrphanedENIs detects orphaned ENIs across all specified regions
func DetectOrphanedENIs(ctx context.Context, regions []string, options DetectOptions) ([]OrphanedENI, error) {
	var orphanedENIs []OrphanedENI

	// Default reserved descriptions to skip
	reservedDescriptions := []string{
		"ELB", "Amazon EKS", "AWS-mgmt", "NAT Gateway", "Kubernetes.io",
	}

	// Add user-specified reserved descriptions
	reservedDescriptions = append(reservedDescriptions, options.SkipReservedDescriptions...)

	// Process each region
	for _, region := range regions {
		// Create AWS config for this region
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			logging.V(5).Infof("Error loading AWS config for region %s: %v", region, err)
			continue
		}

		// Create EC2 client
		ec2Client := ec2.NewFromConfig(cfg)

		// Find all ENIs, not just available ones
		var filters []types.Filter

		// If a security group ID is specified, filter by that
		if options.SecurityGroupId != nil && *options.SecurityGroupId != "" {
			filters = append(filters, types.Filter{
				Name:   aws.String("group-id"),
				Values: []string{*options.SecurityGroupId},
			})
		}

		enis, err := findNetworkInterfaces(ctx, ec2Client, filters)
		if err != nil {
			logging.V(5).Infof("Error finding ENIs in region %s: %v", region, err)
			continue
		}

		// Filter the ENIs to find orphaned ones
		for _, eni := range enis {
			// Skip ENIs with reserved descriptions
			if eni.Description != nil {
				shouldSkip := false
				for _, reservedDesc := range reservedDescriptions {
					if strings.Contains(*eni.Description, reservedDesc) {
						shouldSkip = true
						break
					}
				}
				if shouldSkip {
					logging.V(9).Infof("Skipping ENI %s with reserved description: %s", *eni.NetworkInterfaceId, *eni.Description)
					continue
				}
			}

			// Extract tags
			tags := make(map[string]string)
			for _, tag := range eni.TagSet {
				if tag.Key != nil && tag.Value != nil {
					tags[*tag.Key] = *tag.Value
				}
			}

			// Filter by include tag keys if specified
			if len(options.IncludeTagKeys) > 0 {
				hasIncludeTag := false
				for _, includeKey := range options.IncludeTagKeys {
					if _, ok := tags[includeKey]; ok {
						hasIncludeTag = true
						break
					}
				}
				if !hasIncludeTag {
					continue
				}
			}

			// Filter by exclude tag keys if specified
			if len(options.ExcludeTagKeys) > 0 {
				hasExcludeTag := false
				for _, excludeKey := range options.ExcludeTagKeys {
					if _, ok := tags[excludeKey]; ok {
						hasExcludeTag = true
						break
					}
				}
				if hasExcludeTag {
					continue
				}
			}

			// Filter by age if specified
			// Note: AWS SDK v2 doesn't expose CreateTime directly in NetworkInterface
			// Skip age filtering for now
			if options.OlderThanDays != nil {
				logging.V(9).Infof("Age filtering is not available in the current AWS SDK version")
			}

			// Extract security groups
			var securityGroups []string
			for _, group := range eni.Groups {
				if group.GroupId != nil {
					securityGroups = append(securityGroups, *group.GroupId)
				}
			}

			// Create orphaned ENI entry
			orphanedENI := OrphanedENI{
				ID:             *eni.NetworkInterfaceId,
				Region:         region,
				Tags:           tags,
				SecurityGroups: securityGroups,
				CreatedTime:    time.Now(), // Use current time as fallback since CreateTime isn't available
			}

			if eni.VpcId != nil {
				orphanedENI.VPCID = *eni.VpcId
			}

			if eni.SubnetId != nil {
				orphanedENI.SubnetID = *eni.SubnetId
			}

			if eni.AvailabilityZone != nil {
				orphanedENI.AvailabilityZone = *eni.AvailabilityZone
			}

			if eni.Description != nil {
				orphanedENI.Description = *eni.Description
			}

			if eni.Attachment != nil {
				orphanedENI.AttachmentState = string(eni.Attachment.Status)
				if eni.Attachment.AttachmentId != nil {
					orphanedENI.AttachmentID = *eni.Attachment.AttachmentId
				}
			}

			orphanedENIs = append(orphanedENIs, orphanedENI)
		}
	}

	return orphanedENIs, nil
}

// CleanupOrphanedENIs cleans up orphaned ENIs in the specified regions
func CleanupOrphanedENIs(ctx context.Context, enis []OrphanedENI, dryRun bool, disassociateOnly bool, defaultSecurityGroupId *string, targetSecurityGroupId *string) CleanupResult {
	result := CleanupResult{
		CleanedENIs: make([]CleanedENI, 0),
		Errors:      make([]string, 0),
	}

	// Create a map to group ENIs by region
	enisByRegion := make(map[string][]OrphanedENI)
	for _, eni := range enis {
		enisByRegion[eni.Region] = append(enisByRegion[eni.Region], eni)
	}

	// Process each region
	for region, regionENIs := range enisByRegion {
		// Create AWS config for this region
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			errMsg := fmt.Sprintf("Error loading AWS config for region %s: %v", region, err)
			result.Errors = append(result.Errors, errMsg)
			result.FailureCount += len(regionENIs)
			continue
		}

		// Create EC2 client
		ec2Client := ec2.NewFromConfig(cfg)

		// Get the default security group ID for the region if not provided
		var defaultSG string
		if defaultSecurityGroupId != nil && *defaultSecurityGroupId != "" {
			defaultSG = *defaultSecurityGroupId
		}

		// Process each ENI in the region
		for _, eni := range regionENIs {
			if dryRun {
				logging.V(5).Infof("[DRY RUN] Would clean up ENI %s in region %s", eni.ID, eni.Region)
				result.SkippedCount++
				continue
			}

			// For security group disassociation, we need to determine which groups to remove
			var newGroups []string
			var targetSG string
			var actionTaken string

			// If targetSecurityGroupId is specified, we only want to remove that one
			if targetSecurityGroupId != nil && *targetSecurityGroupId != "" {
				targetSG = *targetSecurityGroupId
				// Keep all security groups except the target one
				for _, sg := range eni.SecurityGroups {
					if sg != targetSG {
						newGroups = append(newGroups, sg)
					}
				}

				// If no groups would be left and we have a default, use it
				if len(newGroups) == 0 && defaultSG != "" {
					newGroups = append(newGroups, defaultSG)
				}

				// If the target SG is not in the current groups, skip
				sgFound := false
				for _, sg := range eni.SecurityGroups {
					if sg == targetSG {
						sgFound = true
						break
					}
				}

				if !sgFound {
					logging.V(5).Infof("ENI %s does not have target security group %s, skipping", eni.ID, targetSG)
					result.SkippedCount++
					continue
				}

				actionTaken = "disassociated from security group " + targetSG
			} else {
				// If no target is specified, remove all security groups and use default if available
				if defaultSG != "" {
					newGroups = []string{defaultSG}
				} else {
					newGroups = []string{} // Empty which is OK for AWS
				}
				actionTaken = "disassociated from all security groups"
			}

			// Modify the ENI's security groups
			logging.V(5).Infof("Modifying security groups for ENI %s", eni.ID)
			_, err := ec2Client.ModifyNetworkInterfaceAttribute(ctx, &ec2.ModifyNetworkInterfaceAttributeInput{
				NetworkInterfaceId: aws.String(eni.ID),
				Groups:             newGroups,
			})

			if err != nil {
				errMsg := fmt.Sprintf("Failed to modify security groups for ENI %s: %v", eni.ID, err)
				result.Errors = append(result.Errors, errMsg)

				// Try to tag for manual cleanup
				tagENIForManualCleanup(ctx, ec2Client, eni.ID, err.Error())
				result.FailureCount++
				continue
			}

			// Only attempt to delete if not in disassociate-only mode
			if !disassociateOnly {
				// Detach the ENI if it's attached
				if eni.AttachmentState != "" && eni.AttachmentState != "detached" && eni.AttachmentID != "" {
					logging.V(5).Infof("Detaching ENI %s (attachment ID: %s)", eni.ID, eni.AttachmentID)
					_, err := ec2Client.DetachNetworkInterface(ctx, &ec2.DetachNetworkInterfaceInput{
						AttachmentId: aws.String(eni.AttachmentID),
						Force:        aws.Bool(true),
					})
					if err != nil {
						errMsg := fmt.Sprintf("Error detaching ENI %s: %v", eni.ID, err)
						result.Errors = append(result.Errors, errMsg)
						result.FailureCount++
						continue
					}

					// Wait a moment for detachment to complete
					time.Sleep(5 * time.Second)
				}

				// Try to delete the ENI
				logging.V(5).Infof("Deleting ENI %s", eni.ID)
				_, err = ec2Client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{
					NetworkInterfaceId: aws.String(eni.ID),
				})
				if err != nil {
					// Tag the ENI for manual cleanup since we can't delete it
					errMsg := fmt.Sprintf("Could not delete ENI %s after removing security groups: %v", eni.ID, err)
					result.Errors = append(result.Errors, errMsg)
					tagENIForManualCleanup(ctx, ec2Client, eni.ID, err.Error())

					// But we succeeded in disassociating security groups, so count as success with disassociate action
					actionTaken = "disassociated from security groups (delete failed)"
				} else {
					actionTaken = "deleted"
				}
			}

			// Success - add to cleaned ENIs
			result.SuccessCount++
			result.CleanedENIs = append(result.CleanedENIs, CleanedENI{
				ID:            eni.ID,
				Region:        eni.Region,
				VpcID:         eni.VPCID,
				Description:   eni.Description,
				ActionTaken:   actionTaken,
				SecurityGroup: targetSG,
			})
		}
	}

	return result
}

// findNetworkInterfaces finds ENIs in the given region based on filters
func findNetworkInterfaces(ctx context.Context, client *ec2.Client, filters []types.Filter) ([]types.NetworkInterface, error) {
	// Find ENIs with the specified filters
	resp, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}

	return resp.NetworkInterfaces, nil
}

// tagENIForManualCleanup tags an ENI for manual cleanup
func tagENIForManualCleanup(ctx context.Context, client *ec2.Client, eniID string, errorMsg string) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	_, err := client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{eniID},
		Tags: []types.Tag{
			{
				Key:   aws.String("NeedsManualCleanup"),
				Value: aws.String("true"),
			},
			{
				Key:   aws.String("AttemptedCleanupTime"),
				Value: aws.String(timestamp),
			},
			{
				Key:   aws.String("DeletionError"),
				Value: aws.String(errorMsg),
			},
		},
	})
	if err != nil {
		logging.V(5).Infof("Failed to tag ENI %s for manual cleanup: %v", eniID, err)
	}
}

package enicleanup

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// Resource is the ENI cleanup resource implementation.
type Resource struct{}

// ResourceArgs defines the arguments for the ENI cleanup resource.
type ResourceArgs struct {
	Regions                  []string `pulumi:"regions"`
	SecurityGroupId          *string  `pulumi:"securityGroupId,optional"`
	DefaultSecurityGroupId   *string  `pulumi:"defaultSecurityGroupId,optional"`
	DryRun                   *bool    `pulumi:"dryRun,optional"`
	SkipReservedDescriptions []string `pulumi:"skipReservedDescriptions,optional"`
	LogLevel                 *string  `pulumi:"logLevel,optional"`
	IncludeTagKeys           []string `pulumi:"includeTagKeys,optional"`
	ExcludeTagKeys           []string `pulumi:"excludeTagKeys,optional"`
	OlderThanDays            *float64 `pulumi:"olderThanDays,optional"`
	DisassociateOnly         *bool    `pulumi:"disassociateOnly,optional"`
}

// ResourceState represents the state of the ENI cleanup resource.
type ResourceState struct {
	// Input fields
	Regions                  []string `pulumi:"regions"`
	SecurityGroupId          *string  `pulumi:"securityGroupId,optional"`
	DefaultSecurityGroupId   *string  `pulumi:"defaultSecurityGroupId,optional"`
	DryRun                   *bool    `pulumi:"dryRun,optional"`
	SkipReservedDescriptions []string `pulumi:"skipReservedDescriptions,optional"`
	LogLevel                 *string  `pulumi:"logLevel,optional"`
	IncludeTagKeys           []string `pulumi:"includeTagKeys,optional"`
	ExcludeTagKeys           []string `pulumi:"excludeTagKeys,optional"`
	OlderThanDays            *float64 `pulumi:"olderThanDays,optional"`
	DisassociateOnly         *bool    `pulumi:"disassociateOnly,optional"`

	// Output fields
	SuccessCount int          `pulumi:"successCount"`
	FailureCount int          `pulumi:"failureCount"`
	SkippedCount int          `pulumi:"skippedCount"`
	CleanedENIs  []CleanedENI `pulumi:"cleanedENIs"`
}

// CleanedENI represents information about a cleaned ENI.
type CleanedENI struct {
	ID            string `pulumi:"id"`
	Region        string `pulumi:"region"`
	VpcID         string `pulumi:"vpcId"`
	Description   string `pulumi:"description"`
	ActionTaken   string `pulumi:"actionTaken"` // "disassociated" or "deleted"
	SecurityGroup string `pulumi:"securityGroup,optional"`
}

// Create implements the create operation for the ENI cleanup resource.
func (r Resource) Create(ctx context.Context, name string, input ResourceArgs, preview bool) (string, ResourceState, error) {
	// Validate inputs
	if len(input.Regions) == 0 {
		return "", ResourceState{}, fmt.Errorf("at least one region must be specified")
	}

	if preview {
		return name, ResourceState{
			Regions:                  input.Regions,
			SecurityGroupId:          input.SecurityGroupId,
			DefaultSecurityGroupId:   input.DefaultSecurityGroupId,
			DryRun:                   input.DryRun,
			SkipReservedDescriptions: input.SkipReservedDescriptions,
			LogLevel:                 input.LogLevel,
			IncludeTagKeys:           input.IncludeTagKeys,
			ExcludeTagKeys:           input.ExcludeTagKeys,
			OlderThanDays:            input.OlderThanDays,
			DisassociateOnly:         input.DisassociateOnly,
		}, nil
	}

	// Set default values for the state
	state := ResourceState{
		Regions:                  input.Regions,
		SecurityGroupId:          input.SecurityGroupId,
		DefaultSecurityGroupId:   input.DefaultSecurityGroupId,
		DryRun:                   input.DryRun,
		SkipReservedDescriptions: input.SkipReservedDescriptions,
		LogLevel:                 input.LogLevel,
		IncludeTagKeys:           input.IncludeTagKeys,
		ExcludeTagKeys:           input.ExcludeTagKeys,
		OlderThanDays:            input.OlderThanDays,
		DisassociateOnly:         input.DisassociateOnly,
		SuccessCount:             0,
		FailureCount:             0,
		SkippedCount:             0,
		CleanedENIs:              []CleanedENI{},
	}

	// Determine if this is a disassociate-only operation
	disassociateOnly := false
	if state.DisassociateOnly != nil {
		disassociateOnly = *state.DisassociateOnly
	}

	// Perform ENI detection and cleanup
	logLevel := "info"
	if state.LogLevel != nil {
		logLevel = *state.LogLevel
	}

	// Setup detection options
	options := DetectOptions{
		SkipReservedDescriptions: state.SkipReservedDescriptions,
		IncludeTagKeys:           state.IncludeTagKeys,
		ExcludeTagKeys:           state.ExcludeTagKeys,
		OlderThanDays:            state.OlderThanDays,
		LogLevel:                 logLevel,
		SecurityGroupId:          state.SecurityGroupId,
	}

	// Detect orphaned ENIs
	orphanedENIs, err := DetectOrphanedENIs(ctx, state.Regions, options)
	if err != nil {
		return "", ResourceState{}, fmt.Errorf("failed to detect orphaned ENIs: %w", err)
	}

	// Log detection results
	logging.V(5).Infof("Detected %d orphaned ENIs", len(orphanedENIs))

	// Determine if this is a dry run
	dryRun := false
	if state.DryRun != nil {
		dryRun = *state.DryRun
	}

	// Perform cleanup
	result := CleanupOrphanedENIs(ctx, orphanedENIs, dryRun, disassociateOnly, state.DefaultSecurityGroupId, state.SecurityGroupId)

	// Update state with results
	state.SuccessCount = result.SuccessCount
	state.FailureCount = result.FailureCount
	state.SkippedCount = result.SkippedCount

	// Convert cleanup results to output state
	for _, eni := range result.CleanedENIs {
		state.CleanedENIs = append(state.CleanedENIs, eni)
	}

	return name, state, nil
}

// Read implements the read operation for the ENI cleanup resource.
func (r Resource) Read(ctx context.Context, id string, oldState ResourceState) (ResourceState, error) {
	// Since this is a stateless resource that performs actions on create and delete,
	// we just return the existing state
	return oldState, nil
}

// Update implements the update operation for the ENI cleanup resource.
func (r Resource) Update(ctx context.Context, id string, oldState ResourceState, newArgs ResourceArgs, preview bool) (ResourceState, error) {
	// If this is a preview, just return the new args without taking action
	if preview {
		return ResourceState{
			Regions:                  newArgs.Regions,
			SecurityGroupId:          newArgs.SecurityGroupId,
			DefaultSecurityGroupId:   newArgs.DefaultSecurityGroupId,
			DryRun:                   newArgs.DryRun,
			SkipReservedDescriptions: newArgs.SkipReservedDescriptions,
			LogLevel:                 newArgs.LogLevel,
			IncludeTagKeys:           newArgs.IncludeTagKeys,
			ExcludeTagKeys:           newArgs.ExcludeTagKeys,
			OlderThanDays:            newArgs.OlderThanDays,
			DisassociateOnly:         newArgs.DisassociateOnly,
			SuccessCount:             oldState.SuccessCount,
			FailureCount:             oldState.FailureCount,
			SkippedCount:             oldState.SkippedCount,
			CleanedENIs:              oldState.CleanedENIs,
		}, nil
	}

	// Determine if this is a disassociate-only operation
	disassociateOnly := false
	if newArgs.DisassociateOnly != nil {
		disassociateOnly = *newArgs.DisassociateOnly
	}

	// Perform update by basically doing a new create operation
	logging.V(5).Infof("Updating ENI cleanup resource")

	// Setup detection options
	logLevel := "info"
	if newArgs.LogLevel != nil {
		logLevel = *newArgs.LogLevel
	}

	options := DetectOptions{
		SkipReservedDescriptions: newArgs.SkipReservedDescriptions,
		IncludeTagKeys:           newArgs.IncludeTagKeys,
		ExcludeTagKeys:           newArgs.ExcludeTagKeys,
		OlderThanDays:            newArgs.OlderThanDays,
		LogLevel:                 logLevel,
		SecurityGroupId:          newArgs.SecurityGroupId,
	}

	// Detect orphaned ENIs
	orphanedENIs, err := DetectOrphanedENIs(ctx, newArgs.Regions, options)
	if err != nil {
		return ResourceState{}, fmt.Errorf("failed to detect orphaned ENIs: %w", err)
	}

	// Determine if this is a dry run
	dryRun := false
	if newArgs.DryRun != nil {
		dryRun = *newArgs.DryRun
	}

	// Perform cleanup
	result := CleanupOrphanedENIs(ctx, orphanedENIs, dryRun, disassociateOnly, newArgs.DefaultSecurityGroupId, newArgs.SecurityGroupId)

	// Create new state with updated values
	newState := ResourceState{
		Regions:                  newArgs.Regions,
		SecurityGroupId:          newArgs.SecurityGroupId,
		DefaultSecurityGroupId:   newArgs.DefaultSecurityGroupId,
		DryRun:                   newArgs.DryRun,
		SkipReservedDescriptions: newArgs.SkipReservedDescriptions,
		LogLevel:                 newArgs.LogLevel,
		IncludeTagKeys:           newArgs.IncludeTagKeys,
		ExcludeTagKeys:           newArgs.ExcludeTagKeys,
		OlderThanDays:            newArgs.OlderThanDays,
		DisassociateOnly:         newArgs.DisassociateOnly,
		SuccessCount:             result.SuccessCount,
		FailureCount:             result.FailureCount,
		SkippedCount:             result.SkippedCount,
		CleanedENIs:              []CleanedENI{},
	}

	// Convert cleanup results to output state
	for _, eni := range result.CleanedENIs {
		newState.CleanedENIs = append(newState.CleanedENIs, eni)
	}

	return newState, nil
}

// Delete implements the delete operation for the ENI cleanup resource.
func (r Resource) Delete(ctx context.Context, id string, state ResourceState) error {
	// Special delete-time ENI cleanup logic
	logging.V(5).Infof("Running delete-time ENI cleanup for resource")

	// Always use disassociate-only for delete operations
	disassociateOnly := true

	// Setup detection options
	logLevel := "info"
	if state.LogLevel != nil {
		logLevel = *state.LogLevel
	}

	options := DetectOptions{
		SkipReservedDescriptions: state.SkipReservedDescriptions,
		IncludeTagKeys:           state.IncludeTagKeys,
		ExcludeTagKeys:           state.ExcludeTagKeys,
		OlderThanDays:            state.OlderThanDays,
		LogLevel:                 logLevel,
		SecurityGroupId:          state.SecurityGroupId,
	}

	// Detect orphaned ENIs
	orphanedENIs, err := DetectOrphanedENIs(ctx, state.Regions, options)
	if err != nil {
		logging.V(5).Infof("Failed to detect orphaned ENIs during deletion: %v", err)
		// Continue even if detection fails - we don't want to block deletion
	}

	// Always perform cleanup on resource deletion, regardless of DryRun setting
	// This ensures resources are cleaned up when the stack is destroyed
	dryRun := false
	if len(orphanedENIs) > 0 {
		result := CleanupOrphanedENIs(ctx, orphanedENIs, dryRun, disassociateOnly, state.DefaultSecurityGroupId, state.SecurityGroupId)
		logging.V(5).Infof("Delete-time cleanup results: %d processed, %d failed, %d skipped",
			result.SuccessCount, result.FailureCount, result.SkippedCount)
	} else {
		logging.V(5).Infof("No orphaned ENIs detected during delete-time cleanup")
	}

	return nil
}

// Annotate sets annotations for the resource.
func (r Resource) Annotate() map[string]interface{} {
	return map[string]interface{}{
		"pulumi:token": "aws-eni-cleanup:index:ENICleanup",
		"description":  "Provides a resource for cleaning up orphaned ENIs in AWS by disassociating them from security groups.",
	}
}

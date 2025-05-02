package enicleanup

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// RegisterENICleanupHandler registers an ENI cleanup handler that runs during resource destruction
// Uses the pulumi-command provider to execute AWS CLI commands that identify and clean up orphaned ENIs
func RegisterENICleanupHandler(
	ctx *pulumi.Context,
	resource pulumi.Resource,
	regions []string,
	logOutput bool,
	dryRun bool,
) (*local.Command, error) {
	// Create a script that will run as part of resource destruction
	cleanupScript := generateCleanupScript(regions, dryRun)

	// Generate a unique name for this cleanup handler
	resourceName := resource.URN().Name()
	cleanupName := fmt.Sprintf("%s-eni-cleanup", resourceName)

	// Create command arguments
	commandArgs := &local.CommandArgs{
		Create:      pulumi.String("echo 'ENI cleanup handler attached'"),
		Delete:      pulumi.String(cleanupScript),
		Interpreter: pulumi.ToStringArray([]string{"/bin/bash", "-c"}),
	}

	// Create command options
	commandOpts := []pulumi.ResourceOption{
		pulumi.Parent(resource),
		// This is crucial: we want this to happen BEFORE the parent resource is destroyed
		pulumi.DeleteBeforeReplace(true),
		// Ensure resource replacement causes the cleanup to run
		pulumi.AdditionalSecretOutputs([]string{"triggers"}),
		pulumi.Trigger("triggers", resource.URN()),
	}

	// Create a command resource that runs during destruction
	cleanupCommand, err := local.NewCommand(ctx, cleanupName, commandArgs, commandOpts...)
	if err != nil {
		return nil, err
	}

	// If we want to see the output, we can export it
	if logOutput {
		cleanupCommand.Stdout.ApplyT(func(stdout string) string {
			if stdout == "" {
				return "No output from ENI cleanup"
			}
			return stdout
		}).(pulumi.StringOutput).ApplyT(func(output string) error {
			outputName := fmt.Sprintf("%s_eni_cleanup", strings.ReplaceAll(strings.ReplaceAll(resource.URN().String(), "::", "_"), "$", "_"))
			ctx.Export(outputName, pulumi.String(output))
			return nil
		})
	}

	return cleanupCommand, nil
}

// generateCleanupScript generates a bash script to cleanup orphaned ENIs
func generateCleanupScript(regions []string, dryRun bool) string {
	regionsStr := ""
	for i, region := range regions {
		if i > 0 {
			regionsStr += " "
		}
		regionsStr += fmt.Sprintf("\"%s\"", region)
	}

	dryRunFlag := ""
	if dryRun {
		dryRunFlag = "--dry-run"
	}

	return fmt.Sprintf(`
#!/bin/bash
set -e

echo "Starting ENI cleanup for regions: %s"

for region in %s; do
    echo "Scanning region: $region for orphaned ENIs"
    
    # Find all ENIs in 'available' state
    echo "Finding available ENIs in $region"
    AVAILABLE_ENIS=$(aws ec2 describe-network-interfaces \
        --region $region \
        --filters "Name=status,Values=available" \
        --query 'NetworkInterfaces[*].{ID:NetworkInterfaceId, VPC:VpcId, Description:Description}' \
        --output json)
    
    # Count them
    ENI_COUNT=$(echo $AVAILABLE_ENIS | jq '. | length')
    
    if [ "$ENI_COUNT" -eq 0 ]; then
        echo "No available ENIs found in $region"
        continue
    fi
    
    echo "Found $ENI_COUNT available ENIs in $region"
    
    # Process each ENI
    echo $AVAILABLE_ENIS | jq -c '.[]' | while read -r eni; do
        ENI_ID=$(echo $eni | jq -r '.ID')
        VPC_ID=$(echo $eni | jq -r '.VPC')
        DESCRIPTION=$(echo $eni | jq -r '.Description')
        
        echo "Processing ENI: $ENI_ID in VPC: $VPC_ID"
        
        # Skip ENIs with reserved descriptions that should not be deleted
        if [[ "$DESCRIPTION" == *"ELB"* || "$DESCRIPTION" == *"Amazon EKS"* || "$DESCRIPTION" == *"AWS-mgmt"* ]]; then
            echo "Skipping ENI $ENI_ID with reserved description: $DESCRIPTION"
            continue
        fi
        
        # Get ENI with additional details
        ENI_DETAILS=$(aws ec2 describe-network-interfaces \
            --region $region \
            --network-interface-ids $ENI_ID \
            --query 'NetworkInterfaces[0]' \
            --output json)
            
        # Check if it has any attachments
        ATTACHMENT_COUNT=$(echo $ENI_DETAILS | jq '.Attachment | length')
        if [ "$ATTACHMENT_COUNT" != "0" ]; then
            # Check if it's detachable
            ATTACH_ID=$(echo $ENI_DETAILS | jq -r '.Attachment.AttachmentId // "none"')
            if [ "$ATTACH_ID" != "none" ]; then
                echo "Detaching ENI $ENI_ID (attachment: $ATTACH_ID)"
                if [ "%s" == "" ]; then
                    aws ec2 detach-network-interface \
                        --region $region \
                        --attachment-id $ATTACH_ID \
                        --force
                    
                    # Wait for detachment to complete
                    echo "Waiting for ENI $ENI_ID to detach completely"
                    sleep 5
                else
                    echo "[DRY RUN] Would detach ENI $ENI_ID (attachment: $ATTACH_ID)"
                fi
            fi
        fi
        
        # Delete the ENI
        echo "Deleting ENI $ENI_ID"
        if [ "%s" == "" ]; then
            # Try to delete the ENI
            if ! aws ec2 delete-network-interface \
                --region $region \
                --network-interface-id $ENI_ID 2>/dev/null; then
                
                echo "Initial deletion failed for ENI $ENI_ID. Trying fallback strategies..."
                
                # Fallback 1: Try removing all security group associations
                echo "Fallback 1: Removing security group associations for ENI $ENI_ID"
                if aws ec2 modify-network-interface-attribute \
                    --region $region \
                    --network-interface-id $ENI_ID \
                    --groups "[]" 2>/dev/null; then
                    
                    echo "Security groups disassociated. Retrying deletion..."
                    sleep 2
                    
                    # Try deleting again
                    if aws ec2 delete-network-interface \
                        --region $region \
                        --network-interface-id $ENI_ID 2>/dev/null; then
                        echo "Successfully deleted ENI $ENI_ID after security group disassociation"
                    else
                        echo "Deletion still failed after removing security groups"
                        
                        # Fallback 2: Tag for manual cleanup
                        echo "Fallback 2: Tagging ENI $ENI_ID for manual cleanup"
                        TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
                        aws ec2 create-tags \
                            --region $region \
                            --resources $ENI_ID \
                            --tags "Key=NeedsManualCleanup,Value=true" "Key=AttemptedCleanupTime,Value=$TIMESTAMP"
                        echo "Tagged ENI $ENI_ID for manual cleanup"
                    fi
                else
                    echo "Failed to modify security groups for ENI $ENI_ID"
                    
                    # Fallback 2: Tag for manual cleanup
                    echo "Fallback 2: Tagging ENI $ENI_ID for manual cleanup"
                    TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
                    aws ec2 create-tags \
                        --region $region \
                        --resources $ENI_ID \
                        --tags "Key=NeedsManualCleanup,Value=true" "Key=AttemptedCleanupTime,Value=$TIMESTAMP"
                    echo "Tagged ENI $ENI_ID for manual cleanup"
                fi
            else
                echo "Successfully deleted ENI $ENI_ID in $region"
            fi
        else
            echo "[DRY RUN] Would delete ENI $ENI_ID in $region"
        fi
    done
done

echo "ENI cleanup completed"
`, strings.Join(regions, ", "), regionsStr, dryRunFlag, dryRunFlag)
}

// generatePythonCleanupScript generates a Python script to cleanup orphaned ENIs
// Used as an alternative when bash might not be available or cross-platform execution is needed
func generatePythonCleanupScript(regions []string, dryRun bool) string {
	regionsJSON, _ := json.Marshal(regions)
	dryRunStr := "False"
	if dryRun {
		dryRunStr = "True"
	}

	return fmt.Sprintf(`
import boto3
import json
import time

regions = %s
dry_run = %s

print(f"Starting ENI cleanup for regions: {', '.join(regions)}")

for region in regions:
    print(f"Scanning region: {region} for orphaned ENIs")
    
    ec2_client = boto3.client('ec2', region_name=region)
    
    # Find all ENIs in 'available' state
    print(f"Finding available ENIs in {region}")
    response = ec2_client.describe_network_interfaces(
        Filters=[{'Name': 'status', 'Values': ['available']}]
    )
    
    available_enis = response.get('NetworkInterfaces', [])
    
    if not available_enis:
        print(f"No available ENIs found in {region}")
        continue
    
    print(f"Found {len(available_enis)} available ENIs in {region}")
    
    # Process each ENI
    for eni in available_enis:
        eni_id = eni['NetworkInterfaceId']
        vpc_id = eni.get('VpcId', 'unknown')
        description = eni.get('Description', '')
        
        print(f"Processing ENI: {eni_id} in VPC: {vpc_id}")
        
        # Skip ENIs with reserved descriptions that should not be deleted
        if any(reserved in description for reserved in ['ELB', 'Amazon EKS', 'AWS-mgmt']):
            print(f"Skipping ENI {eni_id} with reserved description: {description}")
            continue
        
        # Check if it has any attachments
        if 'Attachment' in eni and eni['Attachment']:
            attachment_id = eni['Attachment'].get('AttachmentId')
            if attachment_id:
                print(f"Detaching ENI {eni_id} (attachment: {attachment_id})")
                if not dry_run:
                    try:
                        ec2_client.detach_network_interface(
                            AttachmentId=attachment_id,
                            Force=True
                        )
                        
                        # Wait for detachment to complete
                        print(f"Waiting for ENI {eni_id} to detach completely")
                        time.sleep(5)
                    except Exception as e:
                        print(f"Error detaching ENI {eni_id}: {e}")
                        continue
                else:
                    print(f"[DRY RUN] Would detach ENI {eni_id} (attachment: {attachment_id})")
        
        # Delete the ENI
        print(f"Deleting ENI {eni_id}")
        if not dry_run:
            try:
                # Try to delete the ENI
                ec2_client.delete_network_interface(
                    NetworkInterfaceId=eni_id
                )
                print(f"Successfully deleted ENI {eni_id} in {region}")
            except Exception as initial_error:
                print(f"Initial deletion failed for ENI {eni_id}: {initial_error}")
                print(f"Trying fallback strategies...")
                
                try:
                    # Fallback 1: Try removing all security group associations
                    print(f"Fallback 1: Removing security group associations for ENI {eni_id}")
                    ec2_client.modify_network_interface_attribute(
                        NetworkInterfaceId=eni_id,
                        Groups=[]
                    )
                    
                    print(f"Security groups disassociated. Retrying deletion...")
                    time.sleep(2)
                    
                    # Try deleting again
                    try:
                        ec2_client.delete_network_interface(
                            NetworkInterfaceId=eni_id
                        )
                        print(f"Successfully deleted ENI {eni_id} after security group disassociation")
                    except Exception as second_error:
                        print(f"Deletion still failed after removing security groups: {second_error}")
                        
                        # Fallback 2: Tag for manual cleanup
                        print(f"Fallback 2: Tagging ENI {eni_id} for manual cleanup")
                        timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
                        ec2_client.create_tags(
                            Resources=[eni_id],
                            Tags=[
                                {'Key': 'NeedsManualCleanup', 'Value': 'true'},
                                {'Key': 'AttemptedCleanupTime', 'Value': timestamp},
                                {'Key': 'DeletionError', 'Value': str(second_error)[:255]}
                            ]
                        )
                        print(f"Tagged ENI {eni_id} for manual cleanup")
                except Exception as fallback_error:
                    print(f"Failed to apply fallback strategies: {fallback_error}")
                    
                    # Still try to tag for manual cleanup as last resort
                    try:
                        print(f"Tagging ENI {eni_id} for manual cleanup as last resort")
                        timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
                        ec2_client.create_tags(
                            Resources=[eni_id],
                            Tags=[
                                {'Key': 'NeedsManualCleanup', 'Value': 'true'},
                                {'Key': 'AttemptedCleanupTime', 'Value': timestamp},
                                {'Key': 'DeletionError', 'Value': str(initial_error)[:255]}
                            ]
                        )
                        print(f"Tagged ENI {eni_id} for manual cleanup")
                    except Exception as tag_error:
                        print(f"Failed to tag ENI {eni_id} for manual cleanup: {tag_error}")
        else:
            print(f"[DRY RUN] Would delete ENI {eni_id} in {region}")

print("ENI cleanup completed")
`, regionsJSON, dryRunStr)
}
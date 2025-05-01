import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import * as command from '@pulumi/command';

interface CleanupHandlerOptions {
    logOutput?: boolean;
    dryRun?: boolean;
}

/**
 * Registers an ENI cleanup handler that runs during resource destruction
 * Uses the pulumi-command provider to execute AWS CLI commands that
 * identify and clean up orphaned ENIs.
 */
export function registerENICleanupHandler(
    resource: pulumi.Resource,
    regions: string[],
    options: CleanupHandlerOptions = {}
): command.local.Command {
    const logOutput = options.logOutput ?? true;
    const dryRun = options.dryRun ?? false;
    
    // Create a script that will run as part of resource destruction
    const cleanupScript = generateCleanupScript(regions, dryRun);
    
    // Create a command resource that runs during destruction
    const cleanupCommand = new command.local.Command(`${resource.urn}-eni-cleanup`, {
        create: "echo 'ENI cleanup handler attached'",
        delete: cleanupScript,
        interpreter: ["/bin/bash", "-c"],
    }, {
        parent: resource,
        // This is crucial: we want this to happen BEFORE the parent resource is destroyed
        deleteBeforeReplace: true,
        // Ensure resource replacement causes the cleanup to run
        triggers: [resource.urn],
    });
    
    // If we want to see the output, we can export it
    if (logOutput) {
        const cleanupOutput = cleanupCommand.stdout.apply(stdout => {
            if (!stdout) {
                return "No output from ENI cleanup";
            }
            return stdout;
        });
        
        // Export the output with a name based on the resource
        const outputName = `${resource.getUrn().replace(/[^a-zA-Z0-9]/g, "_")}_eni_cleanup`;
        pulumi.output(cleanupOutput).apply(output => {
            console.log(`[ENI Cleanup for ${resource.urn}]: ${output}`);
        });
    }
    
    return cleanupCommand;
}

/**
 * Generates a bash script to cleanup orphaned ENIs
 */
function generateCleanupScript(regions: string[], dryRun: boolean): string {
    const dryRunFlag = dryRun ? '--dry-run' : '';
    
    return `
#!/bin/bash
set -e

echo "Starting ENI cleanup for regions: ${regions.join(', ')}"

for region in ${regions.map(r => `"${r}"`).join(' ')}; do
    echo "Scanning region: $region for orphaned ENIs"
    
    # Find all ENIs in 'available' state
    echo "Finding available ENIs in $region"
    AVAILABLE_ENIS=$(aws ec2 describe-network-interfaces \\
        --region $region \\
        --filters "Name=status,Values=available" \\
        --query 'NetworkInterfaces[*].{ID:NetworkInterfaceId, VPC:VpcId, Description:Description}' \\
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
        ENI_DETAILS=$(aws ec2 describe-network-interfaces \\
            --region $region \\
            --network-interface-ids $ENI_ID \\
            --query 'NetworkInterfaces[0]' \\
            --output json)
            
        # Check if it has any attachments
        ATTACHMENT_COUNT=$(echo $ENI_DETAILS | jq '.Attachment | length')
        if [ "$ATTACHMENT_COUNT" != "0" ]; then
            # Check if it's detachable
            ATTACH_ID=$(echo $ENI_DETAILS | jq -r '.Attachment.AttachmentId // "none"')
            if [ "$ATTACH_ID" != "none" ]; then
                echo "Detaching ENI $ENI_ID (attachment: $ATTACH_ID)"
                if [ "$dryRunFlag" == "" ]; then
                    aws ec2 detach-network-interface \\
                        --region $region \\
                        --attachment-id $ATTACH_ID \\
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
        if [ "$dryRunFlag" == "" ]; then
            aws ec2 delete-network-interface \\
                --region $region \\
                --network-interface-id $ENI_ID
            echo "Successfully deleted ENI $ENI_ID in $region"
        else
            echo "[DRY RUN] Would delete ENI $ENI_ID in $region"
        fi
    done
done

echo "ENI cleanup completed"
`;
}

/**
 * Generates a Python script to cleanup orphaned ENIs
 * Used as an alternative when bash might not be available or cross-platform execution is needed
 */
function generatePythonCleanupScript(regions: string[], dryRun: boolean): string {
    const dryRunStr = dryRun ? 'True' : 'False';
    
    return `
import boto3
import json
import time

regions = ${JSON.stringify(regions)}
dry_run = ${dryRunStr}

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
                ec2_client.delete_network_interface(
                    NetworkInterfaceId=eni_id
                )
                print(f"Successfully deleted ENI {eni_id} in {region}")
            except Exception as e:
                print(f"Error deleting ENI {eni_id}: {e}")
        else:
            print(f"[DRY RUN] Would delete ENI {eni_id} in {region}")

print("ENI cleanup completed")
`;
}
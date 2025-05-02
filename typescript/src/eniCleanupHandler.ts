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
        
        # Try to delete the ENI
        echo "Deleting ENI $ENI_ID"
        if [ "$dryRunFlag" == "" ]; then
            if aws ec2 delete-network-interface \\
                --region $region \\
                --network-interface-id $ENI_ID; then
                echo "Successfully deleted ENI $ENI_ID in $region"
            else
                echo "Failed to delete ENI $ENI_ID. Attempting fallback methods..."
                
                # Fallback 1: Check for security group associations and try to remove them
                echo "Checking security group associations for ENI $ENI_ID"
                GROUPS=$(echo $ENI_DETAILS | jq -r '.Groups[].GroupId // empty')
                
                if [ ! -z "$GROUPS" ]; then
                    echo "Found security group associations for ENI $ENI_ID. Attempting to modify network interface attribute."
                    # Create a temporary file for groups (empty array)
                    TEMP_FILE=$(mktemp)
                    echo '[]' > $TEMP_FILE
                    
                    # Try to remove all security groups
                    if aws ec2 modify-network-interface-attribute \\
                        --region $region \\
                        --network-interface-id $ENI_ID \\
                        --groups file://$TEMP_FILE; then
                        echo "Successfully removed security group associations from ENI $ENI_ID"
                        
                        # Try deleting again
                        if aws ec2 delete-network-interface \\
                            --region $region \\
                            --network-interface-id $ENI_ID; then
                            echo "Successfully deleted ENI $ENI_ID after removing security groups"
                        else
                            echo "Still failed to delete ENI $ENI_ID after removing security groups"
                        fi
                    else
                        echo "Failed to remove security group associations from ENI $ENI_ID"
                    fi
                    
                    # Clean up temp file
                    rm $TEMP_FILE
                fi
                
                # Fallback 2: If deletion still fails, try to tag it for manual cleanup later
                if aws ec2 describe-network-interfaces \\
                    --region $region \\
                    --network-interface-ids $ENI_ID > /dev/null 2>&1; then
                    echo "ENI $ENI_ID still exists. Tagging it for manual cleanup."
                    aws ec2 create-tags \\
                        --region $region \\
                        --resources $ENI_ID \\
                        --tags Key=NeedsManualCleanup,Value=true Key=AttemptedCleanupTime,Value="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
                    echo "Tagged ENI $ENI_ID for manual cleanup. Please review this ENI later."
                fi
            fi
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
        
        # Try to delete the ENI
        print(f"Deleting ENI {eni_id}")
        if not dry_run:
            try:
                ec2_client.delete_network_interface(
                    NetworkInterfaceId=eni_id
                )
                print(f"Successfully deleted ENI {eni_id} in {region}")
            except Exception as e:
                print(f"Error deleting ENI {eni_id}: {e}")
                print(f"Failed to delete ENI {eni_id}. Attempting fallback methods...")
                
                # Fallback 1: Check for security group associations and try to remove them
                try:
                    # Get current ENI details
                    response = ec2_client.describe_network_interfaces(
                        NetworkInterfaceIds=[eni_id]
                    )
                    if response.get('NetworkInterfaces') and len(response['NetworkInterfaces']) > 0:
                        current_eni = response['NetworkInterfaces'][0]
                        
                        # Check if it has security groups
                        if 'Groups' in current_eni and current_eni['Groups']:
                            print(f"Found security group associations for ENI {eni_id}. Attempting to modify them.")
                            
                            # Try to remove all security groups
                            try:
                                ec2_client.modify_network_interface_attribute(
                                    NetworkInterfaceId=eni_id,
                                    Groups=[]
                                )
                                print(f"Successfully removed security group associations from ENI {eni_id}")
                                
                                # Try deleting again
                                try:
                                    ec2_client.delete_network_interface(
                                        NetworkInterfaceId=eni_id
                                    )
                                    print(f"Successfully deleted ENI {eni_id} after removing security groups")
                                except Exception as e2:
                                    print(f"Still failed to delete ENI {eni_id} after removing security groups: {e2}")
                            except Exception as e3:
                                print(f"Failed to remove security group associations from ENI {eni_id}: {e3}")
                        
                        # Fallback 2: If deletion still fails, try to tag it for manual cleanup later
                        try:
                            # Check if ENI still exists
                            ec2_client.describe_network_interfaces(
                                NetworkInterfaceIds=[eni_id]
                            )
                            # If we get here, ENI still exists, so tag it
                            print(f"ENI {eni_id} still exists. Tagging it for manual cleanup.")
                            import datetime
                            current_time = datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
                            ec2_client.create_tags(
                                Resources=[eni_id],
                                Tags=[
                                    {'Key': 'NeedsManualCleanup', 'Value': 'true'},
                                    {'Key': 'AttemptedCleanupTime', 'Value': current_time}
                                ]
                            )
                            print(f"Tagged ENI {eni_id} for manual cleanup. Please review this ENI later.")
                        except Exception as e4:
                            # Either ENI doesn't exist anymore or tagging failed
                            pass
                except Exception as e5:
                    print(f"Error during fallback cleanup of ENI {eni_id}: {e5}")
        else:
            print(f"[DRY RUN] Would delete ENI {eni_id} in {region}")

print("ENI cleanup completed")
`;
}
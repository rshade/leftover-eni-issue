"""
Module for handling ENI cleanup during resource destruction.
"""

import pulumi
import pulumi_aws as aws
import pulumi_command as command
import json

def register_eni_cleanup_handler(
    resource: pulumi.Resource,
    regions: list,
    log_output: bool = True,
    dry_run: bool = False
) -> command.local.Command:
    """
    Registers an ENI cleanup handler that runs during resource destruction.
    Uses the pulumi-command provider to execute AWS CLI commands that
    identify and clean up orphaned ENIs.
    
    Args:
        resource: The Pulumi resource to attach the handler to
        regions: List of AWS regions to check for orphaned ENIs
        log_output: Whether to log the cleanup output
        dry_run: Whether to run in dry-run mode without making changes
        
    Returns:
        The command resource that will perform the cleanup
    """
    # Create a script that will run as part of resource destruction
    cleanup_script = _generate_cleanup_script(regions, dry_run)
    
    # Generate a unique name for this cleanup handler
    resource_name = resource.urn.apply(lambda urn: urn.split("::")[2])
    cleanup_name = f"{resource_name}-eni-cleanup"
    
    # Create a command resource that runs during destruction
    cleanup_command = command.local.Command(cleanup_name,
        create="echo 'ENI cleanup handler attached'",
        delete=cleanup_script,
        interpreter=["/bin/bash", "-c"],
        opts=pulumi.ResourceOptions(
            parent=resource,
            # This is crucial: we want this to happen BEFORE the parent resource is destroyed
            delete_before_replace=True,
            # Ensure resource replacement causes the cleanup to run
            triggers=[resource.urn]
        )
    )
    
    # If we want to see the output, we can export it
    if log_output:
        cleanup_output = cleanup_command.stdout.apply(lambda stdout: 
            "No output from ENI cleanup" if not stdout else stdout
        )
        
        # Export the output with a name based on the resource
        resource.urn.apply(lambda urn: 
            pulumi.export(f"{urn.replace('::', '_').replace('$', '_')}_eni_cleanup", 
                         cleanup_output)
        )
    
    return cleanup_command

def _generate_cleanup_script(regions: list, dry_run: bool = False) -> str:
    """
    Generates a bash script to cleanup orphaned ENIs.
    
    Args:
        regions: List of AWS regions to check
        dry_run: Whether to run in dry-run mode
        
    Returns:
        The bash script as a string
    """
    # Convert regions list to a space-separated string of quoted region names
    regions_str = ' '.join([f'"{region}"' for region in regions])
    dry_run_flag = '--dry-run' if dry_run else ''
    
    return f"""
#!/bin/bash
set -e

echo "Starting ENI cleanup for regions: {', '.join(regions)}"

for region in {regions_str}; do
    echo "Scanning region: $region for orphaned ENIs"
    
    # Find all ENIs in 'available' state
    echo "Finding available ENIs in $region"
    AVAILABLE_ENIS=$(aws ec2 describe-network-interfaces \\
        --region $region \\
        --filters "Name=status,Values=available" \\
        --query 'NetworkInterfaces[*].{{ID:NetworkInterfaceId, VPC:VpcId, Description:Description}}' \\
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
                if [ "{dry_run_flag}" == "" ]; then
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
        if [ "{dry_run_flag}" == "" ]; then
            # Try to delete the ENI
            if ! aws ec2 delete-network-interface \\
                --region $region \\
                --network-interface-id $ENI_ID 2>/dev/null; then
                
                echo "Initial deletion failed for ENI $ENI_ID. Trying fallback strategies..."
                
                # Fallback 1: Try removing all security group associations
                echo "Fallback 1: Removing security group associations for ENI $ENI_ID"
                if aws ec2 modify-network-interface-attribute \\
                    --region $region \\
                    --network-interface-id $ENI_ID \\
                    --groups "[]" 2>/dev/null; then
                    
                    echo "Security groups disassociated. Retrying deletion..."
                    sleep 2
                    
                    # Try deleting again
                    if aws ec2 delete-network-interface \\
                        --region $region \\
                        --network-interface-id $ENI_ID 2>/dev/null; then
                        echo "Successfully deleted ENI $ENI_ID after security group disassociation"
                    else
                        echo "Deletion still failed after removing security groups"
                        
                        # Fallback 2: Tag for manual cleanup
                        echo "Fallback 2: Tagging ENI $ENI_ID for manual cleanup"
                        TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
                        aws ec2 create-tags \\
                            --region $region \\
                            --resources $ENI_ID \\
                            --tags "Key=NeedsManualCleanup,Value=true" "Key=AttemptedCleanupTime,Value=$TIMESTAMP"
                        echo "Tagged ENI $ENI_ID for manual cleanup"
                    fi
                else
                    echo "Failed to modify security groups for ENI $ENI_ID"
                    
                    # Fallback 2: Tag for manual cleanup
                    echo "Fallback 2: Tagging ENI $ENI_ID for manual cleanup"
                    TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
                    aws ec2 create-tags \\
                        --region $region \\
                        --resources $ENI_ID \\
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
"""

def _generate_python_cleanup_script(regions: list, dry_run: bool = False) -> str:
    """
    Generates a Python script to cleanup orphaned ENIs.
    Used as an alternative when bash might not be available or cross-platform execution is needed.
    
    Args:
        regions: List of AWS regions to check
        dry_run: Whether to run in dry-run mode
        
    Returns:
        The Python script as a string
    """
    regions_json = json.dumps(regions)
    dry_run_str = str(dry_run).lower()
    
    return f"""
import boto3
import json
import time

regions = {regions_json}
dry_run = {dry_run_str}

print(f"Starting ENI cleanup for regions: {{', '.join(regions)}}")

for region in regions:
    print(f"Scanning region: {{region}} for orphaned ENIs")
    
    ec2_client = boto3.client('ec2', region_name=region)
    
    # Find all ENIs in 'available' state
    print(f"Finding available ENIs in {{region}}")
    response = ec2_client.describe_network_interfaces(
        Filters=[{{'Name': 'status', 'Values': ['available']}}]
    )
    
    available_enis = response.get('NetworkInterfaces', [])
    
    if not available_enis:
        print(f"No available ENIs found in {{region}}")
        continue
    
    print(f"Found {{len(available_enis)}} available ENIs in {{region}}")
    
    # Process each ENI
    for eni in available_enis:
        eni_id = eni['NetworkInterfaceId']
        vpc_id = eni.get('VpcId', 'unknown')
        description = eni.get('Description', '')
        
        print(f"Processing ENI: {{eni_id}} in VPC: {{vpc_id}}")
        
        # Skip ENIs with reserved descriptions that should not be deleted
        if any(reserved in description for reserved in ['ELB', 'Amazon EKS', 'AWS-mgmt']):
            print(f"Skipping ENI {{eni_id}} with reserved description: {{description}}")
            continue
        
        # Check if it has any attachments
        if 'Attachment' in eni and eni['Attachment']:
            attachment_id = eni['Attachment'].get('AttachmentId')
            if attachment_id:
                print(f"Detaching ENI {{eni_id}} (attachment: {{attachment_id}})")
                if not dry_run:
                    try:
                        ec2_client.detach_network_interface(
                            AttachmentId=attachment_id,
                            Force=True
                        )
                        
                        # Wait for detachment to complete
                        print(f"Waiting for ENI {{eni_id}} to detach completely")
                        time.sleep(5)
                    except Exception as e:
                        print(f"Error detaching ENI {{eni_id}}: {{e}}")
                        continue
                else:
                    print(f"[DRY RUN] Would detach ENI {{eni_id}} (attachment: {{attachment_id}})")
        
        # Delete the ENI
        print(f"Deleting ENI {{eni_id}}")
        if not dry_run:
            try:
                # Try to delete the ENI
                ec2_client.delete_network_interface(
                    NetworkInterfaceId=eni_id
                )
                print(f"Successfully deleted ENI {{eni_id}} in {{region}}")
            except Exception as initial_error:
                print(f"Initial deletion failed for ENI {{eni_id}}: {{initial_error}}")
                print(f"Trying fallback strategies...")
                
                try:
                    # Fallback 1: Try removing all security group associations
                    print(f"Fallback 1: Removing security group associations for ENI {{eni_id}}")
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
                        print(f"Successfully deleted ENI {{eni_id}} after security group disassociation")
                    except Exception as second_error:
                        print(f"Deletion still failed after removing security groups: {{second_error}}")
                        
                        # Fallback 2: Tag for manual cleanup
                        print(f"Fallback 2: Tagging ENI {{eni_id}} for manual cleanup")
                        timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
                        ec2_client.create_tags(
                            Resources=[eni_id],
                            Tags=[
                                {'Key': 'NeedsManualCleanup', 'Value': 'true'},
                                {'Key': 'AttemptedCleanupTime', 'Value': timestamp},
                                {'Key': 'DeletionError', 'Value': str(second_error)[:255]}
                            ]
                        )
                        print(f"Tagged ENI {{eni_id}} for manual cleanup")
                except Exception as fallback_error:
                    print(f"Failed to apply fallback strategies: {{fallback_error}}")
                    
                    # Still try to tag for manual cleanup as last resort
                    try:
                        print(f"Tagging ENI {{eni_id}} for manual cleanup as last resort")
                        timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
                        ec2_client.create_tags(
                            Resources=[eni_id],
                            Tags=[
                                {'Key': 'NeedsManualCleanup', 'Value': 'true'},
                                {'Key': 'AttemptedCleanupTime', 'Value': timestamp},
                                {'Key': 'DeletionError', 'Value': str(initial_error)[:255]}
                            ]
                        )
                        print(f"Tagged ENI {{eni_id}} for manual cleanup")
                    except Exception as tag_error:
                        print(f"Failed to tag ENI {{eni_id}} for manual cleanup: {{tag_error}}")
        else:
            print(f"[DRY RUN] Would delete ENI {{eni_id}} in {{region}}")

print("ENI cleanup completed")
"""
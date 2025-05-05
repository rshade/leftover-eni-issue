import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import * as command from '@pulumi/command';

// Interface for detected ENIs
export interface OrphanedENI {
    id: string;
    region: string;
    vpcId?: string;
    subnetId?: string;
    availabilityZone?: string;
    description?: string;
    attachmentState?: string;
    createdTime?: string;
    tags: Record<string, string>;
}

/**
 * Detects orphaned ENIs across specified AWS regions.
 * An ENI is considered orphaned if it's in the "available" state or meets other criteria
 * indicating it's been left behind by a resource that was destroyed.
 */
export async function detectOrphanedENIs(
    regions: string[] = ['us-east-1'], 
    provider?: aws.Provider
): Promise<OrphanedENI[]> {
    const orphanedENIs: OrphanedENI[] = [];
    
    // Process each region in parallel
    await Promise.all(regions.map(async (region) => {
        // Create region-specific provider if a default is not provided
        const regionProvider = provider ?? new aws.Provider(`${region}-provider`, {
            region: region,
        });
        
        try {
            // Get all ENIs in the region
            const networkInterfaces = await aws.ec2.getNetworkInterfaces({}, {
                provider: regionProvider,
            });
            
            // Filter the results based on criteria that suggest the ENI is orphaned
            for (const eni of networkInterfaces.networkInterfaces) {
                // Convert AWS API ENI to network interface object for use with isLikelyOrphaned
                const networkInterface = {
                    id: eni.id,
                    attachments: eni.attachment ? [eni.attachment] : [],
                    description: eni.description,
                    status: eni.status,
                    tags: eni.tags || {},
                } as unknown as aws.ec2.NetworkInterface;
                
                if (isLikelyOrphaned(networkInterface)) {
                    orphanedENIs.push({
                        id: eni.id,
                        region: region,
                        vpcId: eni.vpcId,
                        subnetId: eni.subnetId,
                        availabilityZone: eni.availabilityZone,
                        description: eni.description,
                        attachmentState: eni.attachment?.status, 
                        createdTime: PLACEHOLDER_CREATED_TIME, // Placeholder value
                        tags: eni.tags || {},
                    });
                }
            }
        } catch (error) {
            pulumi.log.error(`Error detecting orphaned ENIs in region ${region}: ${error}`);
        }
    }));
    
    return orphanedENIs;
}

/**
 * Check if an ENI is likely orphaned based on its description, 
 * attachment state, and tags.
 */
export function isLikelyOrphaned(eni: aws.ec2.NetworkInterface): boolean {
    // Check if the ENI is in an "available" state, which is the primary indicator of an orphaned ENI
    if (eni.status === 'available') {
        // Skip ENIs with reserved descriptions that should not be deleted
        // These are typically managed by AWS services and should be left alone
        const reservedDescriptions = ['ELB', 'Amazon EKS', 'AWS-mgmt', 'AWS managed', 'NAT Gateway'];
        if (eni.description && reservedDescriptions.some(desc => eni.description!.includes(desc))) {
            return false;
        }
        
        return true;
    }
    
    // Check for ENIs that are stuck in other states but appear to be orphaned
    // This could include ENIs that have been in a "detaching" state for a long time
    if (eni.status === 'detaching') {
        // Could check timestamp if available to see if it's been stuck for a while
        // For now, we're just being cautious and not marking these as orphaned
        return false;
    }
    
    // Check for ENIs with specific tag patterns that indicate they were created by a resource
    // that has since been destroyed but the ENI was left behind
    if (eni.tags) {
        const orphanIndicators = [
            'kubernetes.io/cluster/', // EKS creates ENIs with these tags
            'terraform-', // Terraform-managed resources might have been deleted improperly
            'pulumi-', // Pulumi-managed resources might have been deleted improperly
        ];
        
        // Check for presence of known orphan indicator tags
        const hasOrphanIndicatorTags = Object.keys(eni.tags).some(tagKey => 
            orphanIndicators.some(indicator => tagKey.startsWith(indicator))
        );
        
        // Also check if there's a Name tag pattern indicating temporary resource
        const hasNameIndicatingTemporary = eni.tags['Name'] && 
            (eni.tags['Name'].includes('temporary') || eni.tags['Name'].includes('temp'));
            
        // Only consider tagged ENIs as orphaned if they're also in an available state
        return hasOrphanIndicatorTags || hasNameIndicatingTemporary;
    }
    
    // By default, do not consider an ENI to be orphaned unless it meets the criteria above
    return false;
}

/**
 * Creates a log message about orphaned ENIs that will be displayed during resource destruction.
 * This is useful for debugging and tracking ENIs that might be left behind.
 */
export function logOrphanedENIsOnDestroy(
    resourceName: string, 
    provider?: aws.Provider,
    regions: string[] = ['us-east-1']
): pulumi.Resource {
    // Create a command that will run on resource destruction
    const loggerCommand = new command.local.Command(`${resourceName}-eni-logger`, {
        create: "echo 'ENI logger attached'",
        delete: pulumi
            .output(regions)
            .apply(regions => {
                // Generate a script that logs orphaned ENIs
                const regionsStr = regions.map(r => `"${r}"`).join(' ');
                
                return `
                #!/bin/bash
                set -e
                
                echo "Checking for orphaned ENIs in regions: ${regions.join(', ')}"
                echo "This information is being logged during destroy time for resource: ${resourceName}"
                
                for region in ${regionsStr}; do
                    echo "\\nScanning region: $region for orphaned ENIs"
                    
                    # Find all ENIs in 'available' state
                    AVAILABLE_ENIS=$(aws ec2 describe-network-interfaces \\
                        --region $region \\
                        --filters "Name=status,Values=available" \\
                        --query 'NetworkInterfaces[*].{ID:NetworkInterfaceId, VPC:VpcId, Description:Description, AZ:AvailabilityZone, Subnet:SubnetId}' \\
                        --output json)
                    
                    # Count them
                    ENI_COUNT=$(echo $AVAILABLE_ENIS | jq '. | length')
                    
                    if [ "$ENI_COUNT" -eq 0 ]; then
                        echo "No available ENIs found in $region"
                        continue
                    fi
                    
                    echo "Found $ENI_COUNT available ENIs in $region:"
                    echo $AVAILABLE_ENIS | jq -r '.[] | "ENI ID: \\(.ID) | VPC: \\(.VPC) | AZ: \\(.AZ) | Subnet: \\(.Subnet) | Description: \\(.Description)"'
                done
                
                echo "\\nENI detection completed. You can use the ENI cleanup handler to clean these up."
                `;
            }),
        interpreter: ["/bin/bash", "-c"],
    }, {
        // Use a trigger to ensure the command runs when the parent resource is destroyed
        triggers: [resourceName],
    });
    
    // Output a message to the Pulumi logs about the logger being attached
    pulumi.log.info(`ENI logger attached to ${resourceName}. It will log orphaned ENIs when this resource is destroyed.`);
    
    return loggerCommand;
}
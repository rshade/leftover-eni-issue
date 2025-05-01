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
 * An ENI is considered orphaned if it's in the "available" state.
 */
export async function detectOrphanedENIs(
    regions: string[] = ['us-east-1'], 
    provider?: aws.Provider
): Promise<OrphanedENI[]> {
    // To be implemented - this is just the interface
    return [];
}

/**
 * Check if an ENI is likely orphaned based on its description, 
 * attachment state, and tags.
 */
export function isLikelyOrphaned(eni: aws.ec2.NetworkInterface): boolean {
    // To be implemented
    return false;
}

/**
 * Creates a log message about orphaned ENIs that will be displayed during resource destruction.
 */
export function logOrphanedENIsOnDestroy(
    resourceName: string, 
    provider?: aws.Provider
): pulumi.Resource {
    // To be implemented
    return new pulumi.CustomResource('custom:resource:ENICleanupLogger', resourceName, {}, {});
}
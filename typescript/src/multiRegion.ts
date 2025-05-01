import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';

/**
 * Configuration for multi-region AWS access
 */
export interface RegionConfig {
    region: string;
    profile?: string;
    provider: aws.Provider;
}

/**
 * Creates AWS providers for each specified region
 */
export function configureRegions(
    regions: string[] = ['us-east-1'],
    profile?: string
): Record<string, RegionConfig> {
    const providers: Record<string, RegionConfig> = {};
    
    for (const region of regions) {
        const provider = new aws.Provider(`aws-${region}`, {
            region,
            profile,
        });
        
        providers[region] = {
            region,
            profile,
            provider,
        };
    }
    
    return providers;
}

/**
 * Retrieves a list of all available AWS regions
 */
export async function getAllAwsRegions(provider?: aws.Provider): Promise<string[]> {
    // To be implemented
    return [];
}
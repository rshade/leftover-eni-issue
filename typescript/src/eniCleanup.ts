import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import * as command from '@pulumi/command';
import { OrphanedENI } from './eniDetection';

/**
 * Options for ENI cleanup operations
 */
export interface ENICleanupOptions {
    dryRun?: boolean;
    skipConfirmation?: boolean;
    regions?: string[];
    includeTagKeys?: string[];
    excludeTagKeys?: string[];
    olderThanDays?: number;
    provider?: aws.Provider;
    logLevel?: 'debug' | 'info' | 'warn' | 'error';
}

/**
 * Results of the cleanup operation
 */
export interface CleanupResult {
    successCount: number;
    failureCount: number;
    skippedCount: number;
    errors: Error[];
    cleanedENIs: OrphanedENI[];
    failedENIs: OrphanedENI[];
}

/**
 * Safely detaches and deletes orphaned ENIs
 */
export async function cleanupENIs(
    enis: OrphanedENI[],
    options: ENICleanupOptions = {}
): Promise<CleanupResult> {
    // To be implemented
    return {
        successCount: 0,
        failureCount: 0,
        skippedCount: 0,
        errors: [],
        cleanedENIs: [],
        failedENIs: []
    };
}

/**
 * Creates a pre-destroy hook that attempts to clean up ENIs
 * when a parent resource is destroyed
 */
export function createPreDestroyCleanupHook(
    parentResource: pulumi.Resource,
    options: ENICleanupOptions = {}
): pulumi.Resource {
    // To be implemented
    return new pulumi.CustomResource('custom:resource:ENICleanupHook', 'cleanupHook', {}, {});
}
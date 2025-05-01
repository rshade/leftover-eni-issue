import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import * as command from '@pulumi/command';

// Import internal modules
import { registerENICleanupHandler } from './eniCleanupHandler';

const config = new pulumi.Config();
const regions = config.getObject<string[]>('regions') || ['us-east-1'];

export interface ENICleanupOptions {
    regions?: string[];
    disableCleanup?: boolean;
    logOutput?: boolean;
}

/**
 * ENI Cleanup Component
 * Registers a destroy-time ENI cleanup handler with AWS resources that may
 * create orphaned ENIs during destruction.
 */
export class ENICleanupComponent extends pulumi.ComponentResource {
    constructor(name: string, args: ENICleanupOptions = {}, opts?: pulumi.ComponentResourceOptions) {
        super('awsutil:cleanup:ENICleanupComponent', name, args, opts);
        
        const cleanupRegions = args.regions || regions;
        const disableCleanup = args.disableCleanup || false;
        const logOutput = args.logOutput ?? true;
        
        // Register the cleanup handler with this component resource
        if (!disableCleanup) {
            registerENICleanupHandler(this, cleanupRegions, { logOutput });
        }
        
        this.registerOutputs();
    }
}

/**
 * Creates and attaches an ENI cleanup handler to an AWS resource
 * This will clean up any orphaned ENIs when the resource is destroyed
 */
export function attachENICleanupHandler(
    resource: pulumi.Resource,
    options: ENICleanupOptions = {}
): void {
    const opts = options || {};
    const cleanupRegions = opts.regions || regions;
    const disableCleanup = opts.disableCleanup || false;
    
    if (!disableCleanup) {
        registerENICleanupHandler(resource, cleanupRegions, { logOutput: opts.logOutput ?? true });
    }
}

export { registerENICleanupHandler };
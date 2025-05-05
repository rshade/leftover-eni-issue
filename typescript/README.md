# AWS ENI Cleanup - TypeScript

This Pulumi program creates a destroy-time cleanup mechanism for orphaned Elastic Network Interfaces (ENIs) in AWS accounts. It ensures that ENIs are properly cleaned up when resources are destroyed, preventing failures during `pulumi destroy` operations.

## Prerequisites

- Node.js 14+
- Pulumi CLI
- AWS CLI configured with appropriate permissions
- `jq` utility installed for the bash script execution

## Installation

1. Install dependencies:
   ```
   npm install
   ```

2. Configure your AWS credentials:
   ```
   aws configure
   ```

## Usage

There are two ways to use this module:

### 1. Global Cleanup Component

You can create a global ENI cleanup component that will manage ENIs across multiple regions:

```typescript
import { ENICleanupComponent } from './index';

// Create a global ENI cleanup component
const eniCleanup = new ENICleanupComponent('global', {
    regions: ['us-east-1', 'us-west-2'],
});

// Create AWS resources as children of the cleanup component
const vpc = new aws.ec2.Vpc('example-vpc', {
    cidrBlock: '10.0.0.0/16',
}, { parent: eniCleanup });
```

### 2. Resource-Specific Cleanup Handler

You can attach a cleanup handler to specific resources:

```typescript
import { attachENICleanupHandler } from './index';

// Create a resource
const eksCluster = new aws.eks.Cluster('my-cluster', {
    roleArn: eksRole.arn,
    vpcConfig: {
        subnetIds: [subnet1.id, subnet2.id],
    },
});

// Attach the ENI cleanup handler
attachENICleanupHandler(eksCluster, {
    regions: ['us-east-1'],
    logOutput: true,
});
```

## How It Works

1. The module creates a destroy-time handler using Pulumi Command
2. When a resource is destroyed, the handler runs AWS CLI commands to:
   - Find all available ENIs in the specified regions
   - Detach and delete any orphaned ENIs
   - Log the cleanup process
3. The cleanup happens BEFORE the resource is destroyed, preventing dependency failures

### Fallback Mechanisms

If ENI deletion fails, the handler employs several fallback strategies:

1. **Security Group Disassociation**: If the ENI has security group associations preventing deletion, the handler attempts to remove all security group associations first and then tries deletion again.

2. **Tagging for Manual Review**: If deletion still fails after all automated attempts, the ENI is tagged with:
   - `NeedsManualCleanup: true`
   - `AttemptedCleanupTime: <timestamp>`
   
   This allows for later identification and manual cleanup of problematic ENIs.

3. **Comprehensive Error Handling**: All cleanup operations include detailed error reporting to help identify why an ENI couldn't be deleted.

## Configuration

The following configuration options are available:

- `regions`: List of AWS regions to scan for orphaned ENIs
- `disableCleanup`: Set to true to disable the cleanup (for testing)
- `logOutput`: Set to true to see the cleanup logs

## Testing

Run the tests with:
```
npm test
```
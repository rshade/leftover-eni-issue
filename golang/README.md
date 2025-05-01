# AWS ENI Cleanup - Go

This Pulumi program creates a destroy-time cleanup mechanism for orphaned Elastic Network Interfaces (ENIs) in AWS accounts. It ensures that ENIs are properly cleaned up when resources are destroyed, preventing failures during `pulumi destroy` operations.

## Prerequisites

- Go 1.18+
- Pulumi CLI
- AWS CLI configured with appropriate permissions
- `jq` utility installed for the bash script execution

## Installation

1. Install dependencies:
   ```
   go mod tidy
   ```

2. Configure your AWS credentials:
   ```
   aws configure
   ```

## Usage

There are two ways to use this module:

### 1. Global Cleanup Component

You can create a global ENI cleanup component that will manage ENIs across multiple regions:

```go
import "github.com/organization/eni-cleanup-go/examples"

// Create a global ENI cleanup component
eniCleanup, err := examples.NewENICleanupComponent(ctx, "global", &examples.ENICleanupOptions{
    Regions: []string{"us-east-1", "us-west-2"},
})
if err != nil {
    return err
}

// Create AWS resources as children of the cleanup component
vpc, err := ec2.NewVpc(ctx, "example-vpc", &ec2.VpcArgs{
    CidrBlock: pulumi.String("10.0.0.0/16"),
}, pulumi.Parent(eniCleanup))
if err != nil {
    return err
}
```

### 2. Resource-Specific Cleanup Handler

You can attach a cleanup handler to specific resources:

```go
import "github.com/organization/eni-cleanup-go/examples"

// Create a resource
eksCluster, err := eks.NewCluster(ctx, "my-cluster", &eks.ClusterArgs{
    RoleArn: eksRole.Arn,
    VpcConfig: &eks.ClusterVpcConfigArgs{
        SubnetIds: pulumi.StringArray{subnet1.ID(), subnet2.ID()},
    },
})
if err != nil {
    return err
}

// Attach the ENI cleanup handler
err = examples.AttachENICleanupHandler(ctx, eksCluster, &examples.ENICleanupOptions{
    Regions: []string{"us-east-1"},
})
if err != nil {
    return err
}
```

## How It Works

1. The module creates a destroy-time handler using Pulumi Command
2. When a resource is destroyed, the handler runs AWS CLI commands to:
   - Find all available ENIs in the specified regions
   - Detach and delete any orphaned ENIs
   - Log the cleanup process
3. The cleanup happens BEFORE the resource is destroyed, preventing dependency failures

## Configuration

The following configuration options are available:

- `regions`: List of AWS regions to scan for orphaned ENIs
- `disableCleanup`: Set to true to disable the cleanup (for testing)
- `logOutput`: Set to true (default) to see the cleanup logs

## Testing

Run the tests with:
```
go test ./...
```
# Pulumi AWS ENI Cleanup Provider

This Pulumi provider lets you clean up orphaned Elastic Network Interfaces (ENIs) in AWS as part of your Pulumi infrastructure management. The provider can be used to detect and either disassociate security groups from ENIs or fully remove ENIs that are no longer attached to instances but are still consuming resources and incurring costs.

## Features

- Detect ENIs across multiple AWS regions
- Filter ENIs by age, tags, and description patterns
- Disassociate ENIs from specific security groups
- Optionally assign a default security group as a fallback
- Tag ENIs for manual cleanup when automated processes fail
- Comprehensive error handling with detailed logs
- Works as both a standalone cleanup tool and as a resource that can be parented to other resources

## Building the Provider

To build the provider, use the included Makefile:

```bash
# Build the provider binary
make build

# Install the provider
make install

# Generate the schema and SDKs
make gen_sdk
```

## Using the Provider

First, add the provider to your Pulumi project:

```go
import (
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
    "github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
    "github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
    eni "github.com/organization/aws-eni-cleanup-provider/sdk" // Generated SDK
)
```

Then, create an ENI cleanup resource that disassociates security groups:

```go
// Create a security group for fallback
defaultSG, err := ec2.NewSecurityGroup(ctx, "default-sg", &ec2.SecurityGroupArgs{
    VpcId: vpc.ID(),
    // ... other security group settings
})

// Create the ENI cleanup resource
cleanup, err := eni.NewENICleanup(ctx, "eni-cleanup", &eni.ENICleanupArgs{
    Regions: pulumi.StringArray{
        pulumi.String("us-east-1"),
        pulumi.String("us-west-2"),
    },
    // Target a specific security group to disassociate
    SecurityGroupId: problematicSecurityGroup.ID(),
    // Set a default security group to use as fallback
    DefaultSecurityGroupId: defaultSG.ID(),
    // Only disassociate security groups, don't delete ENIs
    DisassociateOnly: pulumi.Bool(true),
    LogLevel: pulumi.String("info"),
})
if err \!= nil {
    return err
}

// Export outputs
ctx.Export("cleanupSuccessCount", cleanup.SuccessCount)
```

## Configuration Options

The provider supports the following configuration options:

 < /dev/null |  Option | Description | Type | Required |
|--------|-------------|------|----------|
| `regions` | List of AWS regions to scan for ENIs | `[]string` | Yes |
| `securityGroupId` | Target security group ID to disassociate from ENIs | `*string` | No |
| `defaultSecurityGroupId` | Default security group ID to assign if needed | `*string` | No |
| `disassociateOnly` | If true, only disassociate security groups and don't delete ENIs | `*bool` | No |
| `dryRun` | If true, only log what would be done without taking action | `*bool` | No |
| `skipReservedDescriptions` | ENI description patterns to exclude from cleanup | `[]string` | No |
| `logLevel` | Log verbosity level (debug, info, warn, error) | `*string` | No |
| `includeTagKeys` | Only clean ENIs with these tag keys | `[]string` | No |
| `excludeTagKeys` | Skip cleaning ENIs with these tag keys | `[]string` | No |
| `olderThanDays` | Only clean ENIs older than this many days | `*float64` | No |

## Examples

Check the `examples/` directory for complete working examples:

- `examples/main.go`: Basic usage example with multiple regions
- `examples/eni_cleanup_with_fallback.go`: Advanced example showing security group disassociation

## Development

For development, you can use these Make commands:

```bash
# Run linting
make lint

# Format code
make format

# Run tests
make test

# Clean build artifacts
make clean
```

## License

This provider is licensed under the Apache License 2.0.

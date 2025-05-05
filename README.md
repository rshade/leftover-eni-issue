# AWS ENI Cleanup Solution

This repository contains Pulumi components in TypeScript, Python, and Go that address the common issue of orphaned Elastic Network Interfaces (ENIs) in AWS by implementing destroy-time cleanup mechanisms.

## The Problem: Orphaned ENIs in AWS

When working with AWS resources such as EKS clusters, Lambda functions, ECS tasks, and Load Balancers, these services automatically create Elastic Network Interfaces (ENIs) to enable networking functionality. However, when you attempt to destroy these resources with Pulumi or Terraform:

1. The ENIs can sometimes remain in an "available" state despite the parent resource being deleted
2. These orphaned ENIs create dependencies that prevent security groups from being deleted
3. This causes destroy operations to fail with errors like:
   ```
   Error: DependencyViolation: resource sg-XXXX has a dependent object
   ```

As documented in several GitHub issues ([terraform-aws-eks#2048](https://github.com/terraform-aws-modules/terraform-aws-eks/issues/2048), [amazon-vpc-cni-k8s#1447](https://github.com/aws/amazon-vpc-cni-k8s/issues/1447), [amazon-vpc-cni-k8s#1223](https://github.com/aws/amazon-vpc-cni-k8s/issues/1223), [amazon-vpc-cni-k8s#608](https://github.com/aws/amazon-vpc-cni-k8s/issues/608)), this is a common problem with no official comprehensive solution.

The issue causes several other problems:

- Unnecessary costs for unused ENIs remaining in AWS accounts
- VPC resource limits can be reached due to abandoned ENIs
- Subnet IP addresses may be exhausted
- Security risks from unmanaged network interfaces
- Manual cleanup becomes necessary to successfully destroy infrastructure

## Our Solution: Destroy-Time ENI Cleanup

This solution takes a novel approach to the problem:

1. It creates a pre-destroy hook using Pulumi's Command resource that runs **before** the parent resource is destroyed
2. This hook executes AWS CLI commands that identify and clean up any orphaned ENIs
3. By cleaning up ENIs first, the parent resource and its security groups can be destroyed successfully

### Key Technical Innovation

The key innovation is using Pulumi's `deleteBeforeReplace` functionality to ensure ENI cleanup happens at exactly the right time in the resource lifecycle:

```typescript
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
```

This approach eliminates the race conditions that typically occur when AWS resources are being terminated and ensures all network dependencies are properly cleaned up.

## Detailed Implementation

### Cross-Language Support

The solution is implemented in three languages to support the broadest range of Pulumi users:

- **TypeScript**: For JavaScript/TypeScript Pulumi programs
- **Python**: For Python Pulumi programs
- **Go**: For Go Pulumi programs

Each implementation follows language-specific idioms while providing identical functionality.

### Core Components

1. **ENI Cleanup Component**: A reusable Pulumi component resource that can be the parent of AWS resources
2. **Cleanup Handler Function**: A utility function to attach cleanup handlers to specific resources
3. **Cleanup Script Generator**: Creates bash scripts that use AWS CLI to find and clean ENIs
4. **Logging System**: Captures detailed logs of the cleanup process for troubleshooting

### Technical Details of the Cleanup Process

The cleanup script performs these operations:

1. **ENI Detection**: Uses AWS CLI to find all ENIs in the "available" state in specified regions
   ```bash
   aws ec2 describe-network-interfaces --filters "Name=status,Values=available"
   ```

2. **Intelligent Filtering**: Skips ENIs with reserved descriptions to avoid interfering with AWS-managed resources
   ```bash
   if [[ "$DESCRIPTION" == *"ELB"* || "$DESCRIPTION" == *"Amazon EKS"* || "$DESCRIPTION" == *"AWS-mgmt"* ]]; then
       echo "Skipping ENI $ENI_ID with reserved description: $DESCRIPTION"
       continue
   fi
   ```

3. **Safe Detachment**: Handles cases where ENIs might still have attachments
   ```bash
   aws ec2 detach-network-interface --attachment-id $ATTACH_ID --force
   ```

4. **Deletion**: Permanently removes the orphaned ENI
   ```bash
   aws ec2 delete-network-interface --network-interface-id $ENI_ID
   ```

5. **Fallback Strategies**: If deletion fails, implements a series of fallback mechanisms:
   - **Security Group Disassociation**: Attempts to remove security group associations
     ```bash
     aws ec2 modify-network-interface-attribute --network-interface-id $ENI_ID --groups "[]"
     ```
   - **Tagging for Manual Review**: Marks problematic ENIs for later cleanup
     ```bash
     aws ec2 create-tags --resources $ENI_ID --tags Key=NeedsManualCleanup,Value=true
     ```

6. **Comprehensive Logging**: Records each step for debugging and verification

## Usage Patterns

### Option 1: Global Cleanup Component

Create a global cleanup component and make AWS resources children of it:

```typescript
// TypeScript
const eniCleanup = new ENICleanupComponent('global', {
    regions: ['us-east-1', 'us-west-2'],
});

const vpc = new aws.ec2.Vpc('example-vpc', {
    cidrBlock: '10.0.0.0/16',
}, { parent: eniCleanup });
```

```python
# Python
eni_cleanup = ENICleanupComponent('global', ENICleanupOptions(
    regions=['us-east-1', 'us-west-2']
))

vpc = aws.ec2.Vpc('example-vpc', 
    cidr_block='10.0.0.0/16',
    opts=pulumi.ResourceOptions(parent=eni_cleanup)
)
```

```go
// Go
eniCleanup, err := NewENICleanupComponent(ctx, "global", &ENICleanupOptions{
    Regions: []string{"us-east-1", "us-west-2"},
})
if err != nil {
    return err
}

vpc, err := ec2.NewVpc(ctx, "example-vpc", &ec2.VpcArgs{
    CidrBlock: pulumi.String("10.0.0.0/16"),
}, pulumi.Parent(eniCleanup))
if err != nil {
    return err
}
```

### Option 2: Resource-Specific Handlers

Attach handlers directly to resources that might create orphaned ENIs:

```typescript
// TypeScript
const eksCluster = new aws.eks.Cluster('my-cluster', {
    roleArn: eksRole.arn,
    vpcConfig: {
        subnetIds: [subnet1.id, subnet2.id],
    },
});

attachENICleanupHandler(eksCluster, {
    regions: ['us-east-1'],
    logOutput: true,
});
```

```python
# Python
eks_cluster = aws.eks.Cluster('my-cluster',
    role_arn=eks_role.arn,
    vpc_config={
        'subnet_ids': [subnet1.id, subnet2.id],
    }
)

attach_eni_cleanup_handler(eks_cluster, ENICleanupOptions(
    regions=['us-east-1'],
    log_output=True
))
```

```go
// Go
eksCluster, err := eks.NewCluster(ctx, "my-cluster", &eks.ClusterArgs{
    RoleArn: eksRole.Arn,
    VpcConfig: &eks.ClusterVpcConfigArgs{
        SubnetIds: pulumi.StringArray{subnet1.ID(), subnet2.ID()},
    },
})
if err != nil {
    return err
}

err = AttachENICleanupHandler(ctx, eksCluster, &ENICleanupOptions{
    Regions: []string{"us-east-1"},
})
if err != nil {
    return err
}
```

## Advanced Configuration Options

All implementations support these configuration options:

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `regions` | string[] | `["us-east-1"]` | AWS regions to scan for orphaned ENIs |
| `dryRun` | boolean | `false` | If true, only logs actions without making changes |
| `logOutput` | boolean | `true` | Whether to log cleanup operations |
| `disableCleanup` | boolean | `false` | Disable cleanup (for testing) |

## Prerequisites

- **AWS CLI**: Must be installed and configured with appropriate permissions
- **jq**: Required for parsing JSON in bash scripts
- **Pulumi CLI**: Version 3.0.0 or higher
- **Language-specific requirements**:
  - TypeScript: Node.js 14+
  - Python: Python 3.7+
  - Go: Go 1.18+

## Implementation-Specific Details

Each language implementation has its own README with specific details:

- [TypeScript Implementation Details](typescript/README.md)
- [Python Implementation Details](python/README.md)
- [Go Implementation Details](golang/README.md)

## Testing

The solution includes tests to verify that the cleanup process works correctly. Run the tests for your preferred language implementation:

- TypeScript: `cd typescript && npm test`
- Python: `cd python && python -m pytest tests/`
- Go: `cd golang && go test ./...`

## Contributing

Contributions to improve the solution are welcome. Please feel free to submit pull requests or open issues for any bugs or feature requests.

## License

This project is available under the MIT License.

## References

For more information about the ENI cleanup issue, see these GitHub discussions:

- [terraform-aws-eks#2048](https://github.com/terraform-aws-modules/terraform-aws-eks/issues/2048)
- [amazon-vpc-cni-k8s#1447](https://github.com/aws/amazon-vpc-cni-k8s/issues/1447)
- [amazon-vpc-cni-k8s#1223](https://github.com/aws/amazon-vpc-cni-k8s/issues/1223)
- [amazon-vpc-cni-k8s#608](https://github.com/aws/amazon-vpc-cni-k8s/issues/608)
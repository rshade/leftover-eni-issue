# AWS ENI Cleanup - Python

This Pulumi program creates a destroy-time cleanup mechanism for orphaned Elastic Network Interfaces (ENIs) in AWS accounts. It ensures that ENIs are properly cleaned up when resources are destroyed, preventing failures during `pulumi destroy` operations.

## Prerequisites

- Python 3.7+
- Pulumi CLI
- AWS CLI configured with appropriate permissions
- `jq` utility installed for the bash script execution

## Installation

1. Install dependencies:
   ```
   pip install -r requirements.txt
   ```

2. Configure your AWS credentials:
   ```
   aws configure
   ```

## Usage

There are two ways to use this module:

### 1. Global Cleanup Component

You can create a global ENI cleanup component that will manage ENIs across multiple regions:

```python
from __main__ import ENICleanupComponent, ENICleanupOptions

# Create a global ENI cleanup component
eni_cleanup = ENICleanupComponent('global', ENICleanupOptions(
    regions=['us-east-1', 'us-west-2']
))

# Create AWS resources as children of the cleanup component
vpc = aws.ec2.Vpc('example-vpc',
    cidr_block='10.0.0.0/16',
    opts=pulumi.ResourceOptions(parent=eni_cleanup)
)
```

### 2. Resource-Specific Cleanup Handler

You can attach a cleanup handler to specific resources:

```python
from __main__ import attach_eni_cleanup_handler, ENICleanupOptions

# Create a resource
eks_cluster = aws.eks.Cluster('my-cluster',
    role_arn=eks_role.arn,
    vpc_config={
        'subnet_ids': [subnet1.id, subnet2.id],
    }
)

# Attach the ENI cleanup handler
attach_eni_cleanup_handler(eks_cluster, ENICleanupOptions(
    regions=['us-east-1'],
    log_output=True
))
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
- `disable_cleanup`: Set to true to disable the cleanup (for testing)
- `log_output`: Set to true to see the cleanup logs

## Testing

Run the tests with:
```
python -m pytest tests/
```
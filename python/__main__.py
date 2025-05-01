"""
Main entry point for the Pulumi program to cleanup orphaned AWS ENIs during destroy.
"""

import pulumi
import pulumi_aws as aws
from src.eni_cleanup_handler import register_eni_cleanup_handler

# Get config values
config = pulumi.Config()
regions = config.get_object("regions") or ["us-east-1"]

class ENICleanupOptions:
    """Options for the ENI cleanup handler."""
    
    def __init__(self, regions=None, disable_cleanup=False, log_output=True):
        self.regions = regions
        self.disable_cleanup = disable_cleanup
        self.log_output = log_output

class ENICleanupComponent(pulumi.ComponentResource):
    """
    ENI Cleanup Component
    Registers a destroy-time ENI cleanup handler with AWS resources that may
    create orphaned ENIs during destruction.
    """
    
    def __init__(self, name, args=None, opts=None):
        super().__init__('awsutil:cleanup:ENICleanupComponent', name, args, opts)
        
        if args is None:
            args = ENICleanupOptions()
        
        cleanup_regions = args.regions or regions
        disable_cleanup = args.disable_cleanup or False
        log_output = args.log_output if args.log_output is not None else True
        
        # Register the cleanup handler with this component resource
        if not disable_cleanup:
            register_eni_cleanup_handler(self, cleanup_regions, log_output=log_output)
        
        self.register_outputs({})

def attach_eni_cleanup_handler(resource, options=None):
    """
    Creates and attaches an ENI cleanup handler to an AWS resource.
    This will clean up any orphaned ENIs when the resource is destroyed.
    
    Args:
        resource: The Pulumi resource to attach the handler to
        options: Optional ENICleanupOptions
    """
    if options is None:
        options = ENICleanupOptions()
    
    cleanup_regions = options.regions or regions
    disable_cleanup = options.disable_cleanup or False
    log_output = options.log_output if options.log_output is not None else True
    
    if not disable_cleanup:
        register_eni_cleanup_handler(resource, cleanup_regions, log_output=log_output)

# Example usage (commented out)
"""
# Create a global cleanup component
eni_cleanup = ENICleanupComponent('global', ENICleanupOptions(
    regions=['us-east-1', 'us-west-2']
))

# Create a VPC with the cleanup component as its parent
vpc = aws.ec2.Vpc('example-vpc', 
    cidr_block='10.0.0.0/16',
    tags={'Name': 'example-vpc'},
    opts=pulumi.ResourceOptions(parent=eni_cleanup)
)

# Or attach a cleanup handler to a specific resource
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
"""

# Export the supported regions
pulumi.export("regions", regions)
"""
Examples for using the ENI cleanup handler.
"""

import pulumi
import pulumi_aws as aws
from __main__ import ENICleanupComponent, ENICleanupOptions, attach_eni_cleanup_handler

def global_cleanup_example():
    """Example using the ENICleanupComponent for global cleanup."""
    
    # Create a global ENI cleanup component that will handle ENIs across multiple regions
    eni_cleanup = ENICleanupComponent('global', ENICleanupOptions(
        regions=['us-east-1', 'us-west-2']
    ))
    
    # Create a VPC with cleanup component as parent
    vpc = aws.ec2.Vpc('example-vpc', 
        cidr_block='10.0.0.0/16',
        tags={'Name': 'example-vpc'},
        opts=pulumi.ResourceOptions(parent=eni_cleanup)
    )
    
    return vpc

def resource_specific_cleanup_example():
    """Example using the attachENICleanupHandler function for resource-specific cleanup."""
    
    # Create a VPC resource
    vpc = aws.ec2.Vpc('example-vpc',
        cidr_block='10.0.0.0/16',
        tags={'Name': 'example-vpc'},
    )
    
    # Attach an ENI cleanup handler to the VPC
    attach_eni_cleanup_handler(vpc, ENICleanupOptions(
        regions=['us-east-1'],
        log_output=True
    ))
    
    return vpc

def eks_cluster_cleanup_example():
    """Example using with EKS Cluster."""
    
    # Create a VPC for the EKS cluster
    vpc = aws.ec2.Vpc('eks-vpc',
        cidr_block='10.0.0.0/16',
        tags={'Name': 'eks-vpc'},
    )
    
    # Create subnets
    subnet1 = aws.ec2.Subnet('eks-subnet-1',
        vpc_id=vpc.id,
        cidr_block='10.0.1.0/24',
        availability_zone='us-east-1a',
        tags={'Name': 'eks-subnet-1'},
    )
    
    subnet2 = aws.ec2.Subnet('eks-subnet-2',
        vpc_id=vpc.id,
        cidr_block='10.0.2.0/24',
        availability_zone='us-east-1b',
        tags={'Name': 'eks-subnet-2'},
    )
    
    # Create an IAM role for the EKS cluster
    eks_role = aws.iam.Role('eks-role',
        assume_role_policy=json.dumps({
            'Version': '2012-10-17',
            'Statement': [{
                'Action': 'sts:AssumeRole',
                'Effect': 'Allow',
                'Principal': {
                    'Service': 'eks.amazonaws.com',
                },
            }],
        }),
    )
    
    # Attach required policies
    role_attachment = aws.iam.RolePolicyAttachment('eks-policy-attachment',
        role=eks_role.name,
        policy_arn='arn:aws:iam::aws:policy/AmazonEKSClusterPolicy',
    )
    
    # Create EKS cluster
    eks_cluster = aws.eks.Cluster('eks-cluster',
        role_arn=eks_role.arn,
        vpc_config={
            'subnet_ids': [subnet1.id, subnet2.id],
        },
        tags={'Name': 'eks-cluster'},
    )
    
    # Attach ENI cleanup handler to the EKS cluster
    # This ensures that any ENIs created by the EKS cluster are cleaned up when it's destroyed
    attach_eni_cleanup_handler(eks_cluster, ENICleanupOptions(
        regions=['us-east-1'],
        log_output=True
    ))
    
    return eks_cluster
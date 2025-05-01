import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import { attachENICleanupHandler, ENICleanupComponent } from './index';

// Example 1: Using the ENICleanupComponent for global cleanup
export function globalCleanupExample(): void {
    // Create a global ENI cleanup component that will handle ENIs across multiple regions
    const eniCleanup = new ENICleanupComponent('global', {
        regions: ['us-east-1', 'us-west-2'],
    });
    
    // You can then create AWS resources that might create ENIs
    const vpc = new aws.ec2.Vpc('example-vpc', {
        cidrBlock: '10.0.0.0/16',
        tags: {
            Name: 'example-vpc',
        },
    }, { parent: eniCleanup }); // Parent relationship ensures VPC is cleaned up before ENIs
    
    // Other AWS resources like Lambda, EKS, etc., would be created as children of eniCleanup
}

// Example 2: Using the attachENICleanupHandler function for resource-specific cleanup
export function resourceSpecificCleanupExample(): void {
    // Create a VPC resource
    const vpc = new aws.ec2.Vpc('example-vpc', {
        cidrBlock: '10.0.0.0/16',
        tags: {
            Name: 'example-vpc',
        },
    });
    
    // Attach an ENI cleanup handler to the VPC
    attachENICleanupHandler(vpc, {
        regions: ['us-east-1'],
        logOutput: true,
    });
    
    // When the VPC is destroyed, the handler will clean up any orphaned ENIs
}

// Example 3: Using with EKS Cluster
export function eksClusterCleanupExample(): void {
    // Create a VPC for the EKS cluster
    const vpc = new aws.ec2.Vpc('eks-vpc', {
        cidrBlock: '10.0.0.0/16',
        tags: {
            Name: 'eks-vpc',
        },
    });
    
    // Create subnets
    const subnet1 = new aws.ec2.Subnet('eks-subnet-1', {
        vpcId: vpc.id,
        cidrBlock: '10.0.1.0/24',
        availabilityZone: 'us-east-1a',
        tags: {
            Name: 'eks-subnet-1',
        },
    });
    
    const subnet2 = new aws.ec2.Subnet('eks-subnet-2', {
        vpcId: vpc.id,
        cidrBlock: '10.0.2.0/24',
        availabilityZone: 'us-east-1b',
        tags: {
            Name: 'eks-subnet-2',
        },
    });
    
    // Create an IAM role for the EKS cluster
    const eksRole = new aws.iam.Role('eks-role', {
        assumeRolePolicy: JSON.stringify({
            Version: '2012-10-17',
            Statement: [{
                Action: 'sts:AssumeRole',
                Effect: 'Allow',
                Principal: {
                    Service: 'eks.amazonaws.com',
                },
            }],
        }),
    });
    
    // Attach required policies
    new aws.iam.RolePolicyAttachment('eks-policy-attachment', {
        role: eksRole.name,
        policyArn: 'arn:aws:iam::aws:policy/AmazonEKSClusterPolicy',
    });
    
    // Create EKS cluster
    const eksCluster = new aws.eks.Cluster('eks-cluster', {
        roleArn: eksRole.arn,
        vpcConfig: {
            subnetIds: [subnet1.id, subnet2.id],
        },
        tags: {
            Name: 'eks-cluster',
        },
    });
    
    // Attach ENI cleanup handler to the EKS cluster
    // This ensures that any ENIs created by the EKS cluster are cleaned up when it's destroyed
    attachENICleanupHandler(eksCluster, {
        regions: ['us-east-1'],
        logOutput: true,
    });
}
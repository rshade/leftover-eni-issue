# AWS ENI Cleanup Problem

## Background
When working with AWS resources that create Elastic Network Interfaces (ENIs), such as Lambda functions, ECS tasks, and Load Balancers, these ENIs can sometimes become "orphaned" or left behind after the parent resource is destroyed. This creates several issues:

1. Unnecessary costs for unused resources
2. VPC resource limits may be reached
3. Subnet IP addresses may be exhausted
4. Security risks from abandoned network interfaces
5. It causes the security groups to not be able to be cleaned up via pulumi reference the following links:
    - https://github.com/terraform-aws-modules/terraform-aws-eks/issues/2048
    - https://github.com/aws/amazon-vpc-cni-k8s/issues/1447
    - https://github.com/aws/amazon-vpc-cni-k8s/issues/1223
    - https://github.com/aws/amazon-vpc-cni-k8s/issues/608

## Current Situation
AWS doesn't automatically clean up all orphaned ENIs when parent resources are destroyed. Our infrastructure deployments using Pulumi have accumulated leftover ENIs over time.

## Goal
Create one of each Pulumi program that can in Golang, Python and Typescript:
1. Identify orphaned ENIs in our AWS accounts
2. Safely detach and delete these orphaned ENIs
3. Implement preventative measures for future deployments 
4. Write a log message about orphaned ENI's on destroy

## Technical Requirements
- The solution should work across multiple AWS accounts and regions
- It should identify the original creator/resource of the ENI if possible
- Implementation should include proper error handling and logging
- Performance should be optimized for accounts with many ENIs
- The cleanup should be non-disruptive to existing resources
- Attempt to use a pulumi resource for this like pulumi-command

Please type `continue` to proceed with implementing the solution.
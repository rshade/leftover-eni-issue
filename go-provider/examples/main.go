package main

import (
	// The SDK would be generated from the provider - commented out for now
	// eni "github.com/organization/aws-eni-cleanup-provider/sdk"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create a VPC and other resources that might create ENIs
		vpc, err := ec2.NewVpc(ctx, "example-vpc", &ec2.VpcArgs{
			CidrBlock: pulumi.String("10.0.0.0/16"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("example-vpc"),
			},
		})
		if err != nil {
			return err
		}

		// Create an example subnet
		subnet, err := ec2.NewSubnet(ctx, "example-subnet", &ec2.SubnetArgs{
			VpcId:            vpc.ID(),
			CidrBlock:        pulumi.String("10.0.1.0/24"),
			AvailabilityZone: pulumi.String("us-east-1a"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("example-subnet"),
			},
		})
		if err != nil {
			return err
		}

		// Create a security group
		securityGroup, err := ec2.NewSecurityGroup(ctx, "example-sg", &ec2.SecurityGroupArgs{
			VpcId: vpc.ID(),
			Egress: ec2.SecurityGroupEgressArray{
				&ec2.SecurityGroupEgressArgs{
					Protocol:   pulumi.String("-1"),
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(0),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Ingress: ec2.SecurityGroupIngressArray{
				&ec2.SecurityGroupIngressArgs{
					Protocol:   pulumi.String("tcp"),
					FromPort:   pulumi.Int(22),
					ToPort:     pulumi.Int(22),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Tags: pulumi.StringMap{
				"Name": pulumi.String("example-sg"),
			},
		})
		if err != nil {
			return err
		}

		// Method 1: Create a standalone ENI cleanup component
		// Commented out since SDK is not available during build
		/*
			cleanup, err := eni.NewENICleanup(ctx, "global-eni-cleanup", &eni.ENICleanupArgs{
				Regions: pulumi.StringArray{
					pulumi.String("us-east-1"),
					pulumi.String("us-west-2"),
				},
				LogLevel: pulumi.String("info"),
			})
			if err != nil {
				return err
			}

			// Method 2: Create an ENI cleanup resource as a child of the VPC
			// This will ensure ENIs are cleaned up when the VPC is destroyed
			vpcCleanup, err := eni.NewENICleanup(ctx, "vpc-eni-cleanup", &eni.ENICleanupArgs{
				Regions: pulumi.StringArray{
					pulumi.String("us-east-1"),
				},
				IncludeTagKeys: pulumi.StringArray{
					pulumi.String("vpc-id"),
				},
			}, pulumi.Parent(vpc))
			if err != nil {
				return err
			}
		*/

		// Create placeholders for the outputs we would normally export
		cleanup := struct{ SuccessCount pulumi.IntOutput }{pulumi.Int(0).ToIntOutput()}
		vpcCleanup := struct{ SuccessCount pulumi.IntOutput }{pulumi.Int(0).ToIntOutput()}

		// Export outputs
		ctx.Export("vpcId", vpc.ID())
		ctx.Export("subnetId", subnet.ID())
		ctx.Export("securityGroupId", securityGroup.ID())
		ctx.Export("cleanupSuccessCount", cleanup.SuccessCount)
		ctx.Export("vpcCleanupSuccessCount", vpcCleanup.SuccessCount)

		return nil
	})
}

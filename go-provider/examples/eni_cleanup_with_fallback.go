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
		// Create a provider for the us-east-1 region
		awsProvider, err := aws.NewProvider(ctx, "aws-provider", &aws.ProviderArgs{
			Region: pulumi.String("us-east-1"),
		})
		if err != nil {
			return err
		}

		// Create a VPC
		vpc, err := ec2.NewVpc(ctx, "cleanup-demo-vpc", &ec2.VpcArgs{
			CidrBlock: pulumi.String("10.0.0.0/16"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("cleanup-demo-vpc"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Create subnets in different AZs
		subnet1, err := ec2.NewSubnet(ctx, "cleanup-demo-subnet1", &ec2.SubnetArgs{
			VpcId:            vpc.ID(),
			CidrBlock:        pulumi.String("10.0.1.0/24"),
			AvailabilityZone: pulumi.String("us-east-1a"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("cleanup-demo-subnet1"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		subnet2, err := ec2.NewSubnet(ctx, "cleanup-demo-subnet2", &ec2.SubnetArgs{
			VpcId:            vpc.ID(),
			CidrBlock:        pulumi.String("10.0.2.0/24"),
			AvailabilityZone: pulumi.String("us-east-1b"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("cleanup-demo-subnet2"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Create a security group for testing
		securityGroup, err := ec2.NewSecurityGroup(ctx, "cleanup-demo-sg", &ec2.SecurityGroupArgs{
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
				"Name": pulumi.String("cleanup-demo-sg"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Create default security group for fallback
		defaultSG, err := ec2.NewSecurityGroup(ctx, "cleanup-default-sg", &ec2.SecurityGroupArgs{
			VpcId: vpc.ID(),
			Egress: ec2.SecurityGroupEgressArray{
				&ec2.SecurityGroupEgressArgs{
					Protocol:   pulumi.String("-1"),
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(0),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Tags: pulumi.StringMap{
				"Name": pulumi.String("default-sg"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Create network interfaces manually for demonstration
		eni1, err := ec2.NewNetworkInterface(ctx, "demo-eni-1", &ec2.NetworkInterfaceArgs{
			SubnetId:         subnet1.ID(),
			SecurityGroups:   pulumi.StringArray{securityGroup.ID()},
			SourceDestCheck:  pulumi.Bool(true),
			PrivateIpAddress: pulumi.String("10.0.1.100"),
			Description:      pulumi.String("Demo ENI for cleanup testing"),
			Tags: pulumi.StringMap{
				"Name":        pulumi.String("demo-eni-1"),
				"TestPurpose": pulumi.String("ENI-Cleanup-Demo"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		eni2, err := ec2.NewNetworkInterface(ctx, "demo-eni-2", &ec2.NetworkInterfaceArgs{
			SubnetId:         subnet2.ID(),
			SecurityGroups:   pulumi.StringArray{securityGroup.ID()},
			SourceDestCheck:  pulumi.Bool(true),
			PrivateIpAddress: pulumi.String("10.0.2.100"),
			Description:      pulumi.String("Another demo ENI for cleanup testing"),
			Tags: pulumi.StringMap{
				"Name":        pulumi.String("demo-eni-2"),
				"TestPurpose": pulumi.String("ENI-Cleanup-Demo"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Create the ENI cleanup resource with fallback strategies
		// Commented out since SDK is not available during build
		/*
			cleanup, err := eni.NewENICleanup(ctx, "demo-eni-cleanup", &eni.ENICleanupArgs{
				// Target only the region where we created the demo resources
				Regions: pulumi.StringArray{
					pulumi.String("us-east-1"),
				},
				// Only clean up ENIs with our test tag
				IncludeTagKeys: pulumi.StringArray{
					pulumi.String("TestPurpose"),
				},
				// Target specific security group to disassociate
				SecurityGroupId: securityGroup.ID(),
				// Default security group to use instead
				DefaultSecurityGroupId: defaultSG.ID(),
				// Set to true to only disassociate security groups, not delete
				DisassociateOnly: pulumi.Bool(true),
				// Set to false for actual cleanup
				DryRun: pulumi.Bool(false),
				// Set to debug for more detailed logs
				LogLevel: pulumi.String("debug"),
			})
			if err != nil {
				return err
			}
		*/

		// Create placeholder for the cleanup resource we would normally create
		cleanup := struct {
			SuccessCount pulumi.IntOutput
			FailureCount pulumi.IntOutput
			SkippedCount pulumi.IntOutput
		}{
			pulumi.Int(0).ToIntOutput(),
			pulumi.Int(0).ToIntOutput(),
			pulumi.Int(0).ToIntOutput(),
		}

		// Export outputs
		ctx.Export("vpcId", vpc.ID())
		ctx.Export("subnet1Id", subnet1.ID())
		ctx.Export("subnet2Id", subnet2.ID())
		ctx.Export("securityGroupId", securityGroup.ID())
		ctx.Export("defaultSecurityGroupId", defaultSG.ID())
		ctx.Export("eni1Id", eni1.ID())
		ctx.Export("eni2Id", eni2.ID())

		// Export cleanup results
		ctx.Export("cleanupSuccessCount", cleanup.SuccessCount)
		ctx.Export("cleanupFailureCount", cleanup.FailureCount)
		ctx.Export("cleanupSkippedCount", cleanup.SkippedCount)

		return nil
	})
}

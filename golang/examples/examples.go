package examples

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/organization/eni-cleanup-go/pkg/enicleanup"
)

// ENICleanupOptions contains options for the ENI cleanup handler
type ENICleanupOptions struct {
	Regions        []string
	DisableCleanup bool
	LogOutput      *bool
}

// ENICleanupComponent is a component resource that registers a destroy-time ENI cleanup handler
type ENICleanupComponent struct {
	pulumi.ComponentResource
}

// NewENICleanupComponent creates a new ENI cleanup component
func NewENICleanupComponent(ctx *pulumi.Context, name string, args *ENICleanupOptions, opts ...pulumi.ResourceOption) (*ENICleanupComponent, error) {
	comp := &ENICleanupComponent{}
	err := ctx.RegisterComponentResource("awsutil:cleanup:ENICleanupComponent", name, comp, opts...)
	if err != nil {
		return nil, err
	}

	// Default options
	if args == nil {
		args = &ENICleanupOptions{}
	}

	// Default to us-east-1 if not specified
	if len(args.Regions) == 0 {
		args.Regions = []string{"us-east-1"}
	}

	// Setup log output
	logOutput := true
	if args.LogOutput != nil {
		logOutput = *args.LogOutput
	}

	// Register the cleanup handler
	if !args.DisableCleanup {
		_, err := enicleanup.RegisterENICleanupHandler(ctx, comp, args.Regions, logOutput, false)
		if err != nil {
			return nil, err
		}
	}

	ctx.RegisterResourceOutputs(comp, pulumi.Map{})
	return comp, nil
}

// AttachENICleanupHandler attaches an ENI cleanup handler to a resource
func AttachENICleanupHandler(ctx *pulumi.Context, resource pulumi.Resource, options *ENICleanupOptions) error {
	// Default options
	if options == nil {
		options = &ENICleanupOptions{}
	}

	// Default to us-east-1 if not specified
	if len(options.Regions) == 0 {
		options.Regions = []string{"us-east-1"}
	}

	// Setup log output
	logOutput := true
	if options.LogOutput != nil {
		logOutput = *options.LogOutput
	}

	// Register the cleanup handler
	if !options.DisableCleanup {
		_, err := enicleanup.RegisterENICleanupHandler(ctx, resource, options.Regions, logOutput, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// GlobalCleanupExample demonstrates using the ENICleanupComponent for global cleanup
func GlobalCleanupExample(ctx *pulumi.Context) (*ec2.Vpc, error) {
	// Create a global ENI cleanup component that will handle ENIs across multiple regions
	eniCleanup, err := NewENICleanupComponent(ctx, "global", &ENICleanupOptions{
		Regions: []string{"us-east-1", "us-west-2"},
	})
	if err != nil {
		return nil, err
	}

	// Create a VPC with the cleanup component as parent
	vpc, err := ec2.NewVpc(ctx, "example-vpc", &ec2.VpcArgs{
		CidrBlock: pulumi.String("10.0.0.0/16"),
		Tags: pulumi.StringMap{
			"Name": pulumi.String("example-vpc"),
		},
	}, pulumi.Parent(eniCleanup))
	if err != nil {
		return nil, err
	}

	return vpc, nil
}

// ResourceSpecificCleanupExample demonstrates using the AttachENICleanupHandler function for resource-specific cleanup
func ResourceSpecificCleanupExample(ctx *pulumi.Context) (*ec2.Vpc, error) {
	// Create a VPC resource
	vpc, err := ec2.NewVpc(ctx, "example-vpc", &ec2.VpcArgs{
		CidrBlock: pulumi.String("10.0.0.0/16"),
		Tags: pulumi.StringMap{
			"Name": pulumi.String("example-vpc"),
		},
	})
	if err != nil {
		return nil, err
	}

	// Attach an ENI cleanup handler to the VPC
	err = AttachENICleanupHandler(ctx, vpc, &ENICleanupOptions{
		Regions: []string{"us-east-1"},
	})
	if err != nil {
		return nil, err
	}

	return vpc, nil
}

// EksClusterCleanupExample demonstrates using with EKS Cluster
func EksClusterCleanupExample(ctx *pulumi.Context) (*eks.Cluster, error) {
	// Create a VPC for the EKS cluster
	vpc, err := ec2.NewVpc(ctx, "eks-vpc", &ec2.VpcArgs{
		CidrBlock: pulumi.String("10.0.0.0/16"),
		Tags: pulumi.StringMap{
			"Name": pulumi.String("eks-vpc"),
		},
	})
	if err != nil {
		return nil, err
	}

	// Create subnets
	subnet1, err := ec2.NewSubnet(ctx, "eks-subnet-1", &ec2.SubnetArgs{
		VpcId:            vpc.ID(),
		CidrBlock:        pulumi.String("10.0.1.0/24"),
		AvailabilityZone: pulumi.String("us-east-1a"),
		Tags: pulumi.StringMap{
			"Name": pulumi.String("eks-subnet-1"),
		},
	})
	if err != nil {
		return nil, err
	}

	subnet2, err := ec2.NewSubnet(ctx, "eks-subnet-2", &ec2.SubnetArgs{
		VpcId:            vpc.ID(),
		CidrBlock:        pulumi.String("10.0.2.0/24"),
		AvailabilityZone: pulumi.String("us-east-1b"),
		Tags: pulumi.StringMap{
			"Name": pulumi.String("eks-subnet-2"),
		},
	})
	if err != nil {
		return nil, err
	}

	// Create an IAM role for the EKS cluster
	eksRole, err := iam.NewRole(ctx, "eks-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Action": "sts:AssumeRole",
				"Effect": "Allow",
				"Principal": {
					"Service": "eks.amazonaws.com"
				}
			}]
		}`),
	})
	if err != nil {
		return nil, err
	}

	// Attach required policies
	_, err = iam.NewRolePolicyAttachment(ctx, "eks-policy-attachment", &iam.RolePolicyAttachmentArgs{
		Role:      eksRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"),
	})
	if err != nil {
		return nil, err
	}

	// Create EKS cluster
	eksCluster, err := eks.NewCluster(ctx, "eks-cluster", &eks.ClusterArgs{
		RoleArn: eksRole.Arn,
		VpcConfig: &eks.ClusterVpcConfigArgs{
			SubnetIds: pulumi.StringArray{
				subnet1.ID(),
				subnet2.ID(),
			},
		},
		Tags: pulumi.StringMap{
			"Name": pulumi.String("eks-cluster"),
		},
	})
	if err != nil {
		return nil, err
	}

	// Attach ENI cleanup handler to the EKS cluster
	// This ensures that any ENIs created by the EKS cluster are cleaned up when it's destroyed
	err = AttachENICleanupHandler(ctx, eksCluster, &ENICleanupOptions{
		Regions: []string{"us-east-1"},
	})
	if err != nil {
		return nil, err
	}

	return eksCluster, nil
}
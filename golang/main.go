package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

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

	// Get default regions from config if not provided
	if len(args.Regions) == 0 {
		conf := config.New(ctx, "")
		var regions []string
		if err := conf.TryObject("regions", &regions); err == nil && len(regions) > 0 {
			args.Regions = regions
		} else {
			// Default to us-east-1 if not specified
			args.Regions = []string{"us-east-1"}
		}
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

	// Get default regions from config if not provided
	if len(options.Regions) == 0 {
		conf := config.New(ctx, "")
		var regions []string
		if err := conf.TryObject("regions", &regions); err == nil && len(regions) > 0 {
			options.Regions = regions
		} else {
			// Default to us-east-1 if not specified
			options.Regions = []string{"us-east-1"}
		}
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

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Get configuration
		conf := config.New(ctx, "")
		var regions []string
		conf.RequireObject("regions", &regions)
		
		// Export outputs
		ctx.Export("regions", pulumi.ToStringArray(regions))
		
		// Example usage (commented out)
		/*
		// Global cleanup component
		eniCleanup, err := NewENICleanupComponent(ctx, "global", &ENICleanupOptions{
			Regions: []string{"us-east-1", "us-west-2"},
		})
		if err != nil {
			return err
		}
		
		// Create a VPC with the cleanup component as parent
		vpc, err := aws.NewVpc(ctx, "example-vpc", &aws.VpcArgs{
			CidrBlock: pulumi.String("10.0.0.0/16"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("example-vpc"),
			},
		}, pulumi.Parent(eniCleanup))
		if err != nil {
			return err
		}
		
		// Or attach cleanup handler to specific resources
		eksCluster, err := aws.NewEksCluster(ctx, "eks-cluster", &aws.EksClusterArgs{
			RoleArn: eksRole.Arn,
			VpcConfig: &aws.EksClusterVpcConfigArgs{
				SubnetIds: pulumi.ToStringArray([]string{subnet1.ID(), subnet2.ID()}),
			},
		})
		if err != nil {
			return err
		}
		
		if err := AttachENICleanupHandler(ctx, eksCluster, &ENICleanupOptions{
			Regions: []string{"us-east-1"},
		}); err != nil {
			return err
		}
		*/
		
		return nil
	})
}
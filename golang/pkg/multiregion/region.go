package multiregion

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// RegionConfig represents configuration for multi-region AWS access
type RegionConfig struct {
	Region   string
	Profile  *string
	Provider *aws.Provider
}

// ConfigureRegions creates AWS providers for each specified region
func ConfigureRegions(ctx *pulumi.Context, regions []string, profile *string) (map[string]*RegionConfig, error) {
	providers := make(map[string]*RegionConfig)
	
	for _, region := range regions {
		provider, err := aws.NewProvider(ctx, "aws-"+region, &aws.ProviderArgs{
			Region:  pulumi.String(region),
			Profile: profile,
		})
		if err != nil {
			return nil, err
		}
		
		providers[region] = &RegionConfig{
			Region:   region,
			Profile:  profile,
			Provider: provider,
		}
	}
	
	return providers, nil
}

// GetAllAwsRegions retrieves a list of all available AWS regions
func GetAllAwsRegions(ctx *pulumi.Context, provider *aws.Provider) ([]string, error) {
	// To be implemented
	return []string{}, nil
}
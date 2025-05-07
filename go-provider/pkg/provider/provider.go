package provider

import (
	"github.com/organization/aws-eni-cleanup-provider/pkg/resource/enicleanup"
	"github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// NewProvider creates a new provider instance
func NewProvider() provider.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource[enicleanup.Resource, enicleanup.ResourceArgs, enicleanup.ResourceState](),
		},
	})
}

package main

import (
	"github.com/organization/aws-eni-cleanup-provider/pkg/provider"
	"github.com/organization/aws-eni-cleanup-provider/pkg/schema"
	pulumiProvider "github.com/pulumi/pulumi-go-provider"
)

func main() {
	pulumiProvider.RunProvider(schema.ProviderName, schema.ProviderVersion, provider.NewProvider())
}

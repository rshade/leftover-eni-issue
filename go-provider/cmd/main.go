package main

import (
	"github.com/organization/aws-eni-cleanup-provider/pkg/provider"
	"github.com/organization/aws-eni-cleanup-provider/pkg/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

func main() {
	provider.Main(schema.ProviderName, schema.ProviderVersion, func(host *provider.HostClient) (provider.Provider, error) {
		return provider.New(host)
	})
}

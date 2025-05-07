package schema

// ProviderName is the name of the provider
const ProviderName = "aws-eni-cleanup"

// ProviderVersion is the version of the provider
const ProviderVersion = "0.0.1"

// ProviderMetadata returns the metadata for the provider
func ProviderMetadata() map[string]interface{} {
	return map[string]interface{}{
		"displayName": "AWS ENI Cleanup",
		"description": "A Pulumi provider for cleaning up orphaned ENIs in AWS",
		"keywords": []string{
			"pulumi",
			"aws",
			"eni",
			"cleanup",
			"orphaned",
			"infrastructure",
		},
		"homepage":   "https://github.com/organization/aws-eni-cleanup-provider",
		"repository": "https://github.com/organization/aws-eni-cleanup-provider",
		"publisher":  "Organization",
		"logoUrl":    "",
		"license":    "Apache-2.0",
		"language":   "go",
	}
}

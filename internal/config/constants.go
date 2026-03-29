package config

// Default values and environment variables for the CLI.
const (
	DefaultTableName = "dynamoctl"
	DefaultNamespace = "default"

	DefaultBackupPrefix = "dynamoctl-backups"
)

const (
	EnvTable     = "DYNAMOCTL_TABLE"
	EnvNamespace = "DYNAMOCTL_NAMESPACE"
	EnvKey       = "DYNAMOCTL_KEY"
	EnvAWSRegion = "AWS_REGION"
)

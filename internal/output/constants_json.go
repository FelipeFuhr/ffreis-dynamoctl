package output

const (
	jsonKeyAction    = "action"
	jsonKeyNamespace = "namespace"
	jsonKeyName      = "name"
	jsonKeyVersion   = "version"

	jsonKeyRotated = "rotated"
	jsonKeySkipped = "skipped"
	jsonKeyFailed  = "failed"

	jsonKeyS3URI     = "s3_uri"
	jsonKeyItemCount = "item_count"

	jsonKeyRestored = "restored"
	jsonKeyErrors   = "errors"

	actionSet     = "set"
	actionDeleted = "deleted"
	actionRotate  = "rotate"
	actionBackup  = "backup"
	actionRestore = "restore"
)

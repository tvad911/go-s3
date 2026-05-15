package s3

type ContextKey string

const (
	CtxKeyIsCustomDomain ContextKey = "IsCustomDomain"
	CtxKeyCustomDomain   ContextKey = "CustomDomain"
)

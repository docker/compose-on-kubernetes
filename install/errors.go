package install

const (
	// General image pull error.
	errImagePull = "ErrImagePull"
)

// IsErrImagePull checks if an error is an errImagePull
func IsErrImagePull(reason string) bool {
	return reason == errImagePull
}

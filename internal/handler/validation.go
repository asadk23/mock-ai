package handler

// validationError is a request validation error that includes the offending
// parameter name. It is used across all handler validation functions.
type validationError struct {
	message string
	param   string
}

func (e *validationError) Error() string {
	return e.message
}

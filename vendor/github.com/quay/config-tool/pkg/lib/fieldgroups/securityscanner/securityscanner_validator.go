package securityscanner

import (
	"github.com/quay/config-tool/pkg/lib/shared"
)

// Validate checks the configuration settings for this field group
func (fg *SecurityScannerFieldGroup) Validate(opts shared.Options) []shared.ValidationError {

	// Make empty errors
	errors := []shared.ValidationError{}

	// Make sure feature is enabled
	if !fg.FeatureSecurityScanner {
		return errors
	}

	// Make sure at least one endpoint is present
	if ok, err := shared.ValidateAtLeastOneOfString([]string{fg.SecurityScannerEndpoint, fg.SecurityScannerV4Endpoint}, []string{"SECURITY_SCANNER_ENDPOINT", "SECURITY_SCANNER_V4_ENDPOINT"}, "SecurityScanner"); !ok {
		errors = append(errors, err)
		return errors
	}

	// If v2 endpoint is present
	if len(fg.SecurityScannerEndpoint) > 0 {
		// Check for endpoint
		if ok, err := shared.ValidateRequiredString(fg.SecurityScannerEndpoint, "SECURITY_SCANNER_ENDPOINT", "SecurityScanner"); !ok {
			errors = append(errors, err)
		}

		// Check endpoint is valid url
		if ok, err := shared.ValidateIsURL(fg.SecurityScannerEndpoint, "SECURITY_SCANNER_ENDPOINT", "SecurityScanner"); !ok {
			errors = append(errors, err)
		}
	}

	// If v4 endpoint is present
	if len(fg.SecurityScannerV4Endpoint) > 0 {
		// Check for endpoint
		if ok, err := shared.ValidateRequiredString(fg.SecurityScannerV4Endpoint, "SECURITY_SCANNER_V4_ENDPOINT", "SecurityScanner"); !ok {
			errors = append(errors, err)
		}

		// Check endpoint is valid url
		if ok, err := shared.ValidateIsURL(fg.SecurityScannerV4Endpoint, "SECURITY_SCANNER_V4_ENDPOINT", "SecurityScanner"); !ok {
			errors = append(errors, err)
		}
	}

	// Return errors
	return errors

}

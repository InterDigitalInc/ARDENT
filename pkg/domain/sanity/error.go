package sanity

import (
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

const (
	// First 10 error codes are kept for fixed errors that are directly
	// mapped to HTTP status codes in models.

	SANITY_SERVICE_INVALID_REQ_ERR = 1 // 1 - 50 are fixed for common error codes.

	SANITY_SERVICE_SPECIFIC_ERROR             = models.SANITY_SERVICE_COMMON_ERR_END // Error code base for sanity service specific errors.
	SANITY_SERVICE_TENANT_RC_NOT_UPLOADED_ERR = SANITY_SERVICE_SPECIFIC_ERROR + 1    // Error code for missing Tenant OpenRC.
	SANITY_SERVICE_SANITY_CHECK_ERR           = SANITY_SERVICE_SPECIFIC_ERROR + 2    // Error code for sanity-check error.
	SANITY_SERVICE_SANITY_CHECK_WARN          = SANITY_SERVICE_SPECIFIC_ERROR + 3    // Error code for sanity-check passed with warning(s).
)

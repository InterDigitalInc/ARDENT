package hot

import (
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

const (
	// First 10 error codes are kept for fixed errors that are directly
	// mapped to HTTP status codes in models.

	HOT_SERVICE_INVALID_REQ_ERR = 1 // 1 - 50 are fixed for common error codes.

	HOT_SERVICE_SPECIFIC_ERROR             = models.HOT_SERVICE_COMMON_ERR_END // Error code base for hot service specific errors.
	HOT_SERVICE_TENANT_RC_NOT_UPLOADED_ERR = HOT_SERVICE_SPECIFIC_ERROR + 1    // Error code for missing Tenant OpenRC.
	HOT_SERVICE_TEMPLATE_GEN_ERR           = HOT_SERVICE_SPECIFIC_ERROR + 2    // Error code for HEAT template generation error.
)

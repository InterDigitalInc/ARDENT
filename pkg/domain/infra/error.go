package infra

import (
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

const (
	// First 10 error codes are kept for fixed errors that are directly
	// mapped to HTTP status codes in models.

	INFRA_SERVICE_INVALID_REQ_ERR = 1 // 1 - 50 are for common error codes.

	INFRA_SERVICE_SPECIFIC_ERROR             = models.INFRA_SERVICE_COMMON_ERR_END // Error code base for infra service specific errors.
	INFRA_SERVICE_TENANT_RC_NOT_UPLOADED_ERR = INFRA_SERVICE_SPECIFIC_ERROR + 1    // Error code for missing Tenant OpenRC.
	INFRA_SERVICE_INFRA_DESC_PARSE_ERR       = INFRA_SERVICE_SPECIFIC_ERROR + 2    // Error code for infra descriptor parsing error.
)

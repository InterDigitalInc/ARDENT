package stack

import (
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

const (
	// First 10 error codes are kept for fixed errors that are directly
	// mapped to HTTP status codes in models.

	STACK_SERVICE_INVALID_REQ_ERR = 1 // 1 - 50 are fixed for common error codes.

	STACK_SERVICE_SPECIFIC_ERROR                  = models.STACK_SERVICE_COMMON_ERR_END // Error code base for stack service specific errors.
	STACK_SERVICE_TENANT_RC_NOT_UPLOADED_ERR      = STACK_SERVICE_SPECIFIC_ERROR + 1    // Error code for missing Tenant OpenRC.
	STACK_SERVICE_ADMIN_RC_NOT_UPLOADED_ERR       = STACK_SERVICE_SPECIFIC_ERROR + 2    // Error code for missing Admin OpenRC.
	STACK_SERVICE_HEAT_TEMPLATE_NOT_GENERATED_ERR = STACK_SERVICE_SPECIFIC_ERROR + 3    // Error code for missing HEAT template.
	STACK_SERVICE_INVALID_HEAT_TEMPLATE_ERR       = STACK_SERVICE_SPECIFIC_ERROR + 4    // Error code for invalid HEAT template.
	STACK_SERVICE_STACK_DOES_NOT_EXIST_ERR        = STACK_SERVICE_SPECIFIC_ERROR + 5    // Error code for missing stack.
)

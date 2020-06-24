package models

// A StatusId is defined for service response status code.
type StatusId int

// A StatusMsg is defined for service response status message.
type StatusMsg string

const (
	NO_ERR StatusId = 0 // HTTP Status Code - 200

	COMMON_ERROR_CODE_START = 1  // Lower limit of common error code range.
	COMMON_ERROR_CODE_END   = 50 // Upper limit of common error code range.

	REQ_FORBIDDEN_ERR_START = 1  // Lower limit of invalid request error code range.
	REQ_FORBIDDEN_ERR_END   = 10 // Upper limit of invalid request error code range.

	INT_SERVER_ERR_START = 11 // Lower limit of internal server error code range.
	INT_SERVER_ERR_END   = 20 // Upper limit of internal server error code range.

	EXTERNAL_ORCH_ERR_START = 21 // Lower limit of orchestrator error code range.
	EXTERNAL_ORCH_ERR_END   = 30 // Upper limit of orchestrator error code range.

	// 31 - 50 are reserved.

	REQ_FORBIDDEN StatusId = 1 // HTTP Status Code - 403

	INT_SERVER_ERR    StatusId = 11 // HTTP Status Code - 500
	INT_SERVER_DB_ERR StatusId = 12 // HTTP Status Code - 500

	EXTERNAL_ORCH_ERR StatusId = 21 // HTTP Status Code - 200

	// Service specific base code ranges.
	INFRA_SERVICE_COMMON_ERR_START = 100 // Lower limit of infra service error code range.
	INFRA_SERVICE_COMMON_ERR_END   = 150 // Upper limit of infra service error code range.

	STACK_SERVICE_COMMON_ERR_START = 200 // Lower limit of stack service error code range.
	STACK_SERVICE_COMMON_ERR_END   = 250 // Upper limit of stack service error code range.

	HOT_SERVICE_COMMON_ERR_START = 300 // Lower limit of hot service error code range.
	HOT_SERVICE_COMMON_ERR_END   = 350 // Upper limit of hot service error code range.

	SANITY_SERVICE_COMMON_ERR_START = 400 // Lower limit of sanity service error code range.
	SANITY_SERVICE_COMMON_ERR_END   = 450 // Upper limit of sanity service error code range.

	SUCCESS_MSG StatusMsg = "Request Successful" // Response Status Msg returned by service interface for successful request.
)

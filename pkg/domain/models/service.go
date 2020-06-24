// Package models provides interfaces and structures that are shared and
// used among Ardent services.
package models

const (
	// Value used for computing final status_id returned in response payload for infra service.
	INFRA_SERVICE byte = 1

	// Value used for computing final status_id returned in response payload for stack service.
	STACK_SERVICE byte = 2

	// Value used for computing final status_id returned in response payload for hot service.
	HOT_SERVICE byte = 3

	// Value used for computing final status_id returned in response payload for sanity service.
	SANITY_SERVICE byte = 4
)

// A HotService interface provides function signatures for hot related operations.
type HotService interface {
	DeleteDescriptorIfExists() (StatusId, StatusMsg)
}

// A SanityService interface provides function signatures for sanity related operations.
type SanityService interface {
	CheckSanityCheckResult() (StatusId, StatusMsg)
	DeleteSanityCheckResult() (StatusId, StatusMsg)
}

// A StackService interface provides function signatures for stack related operations.
type StackService interface {
	DeleteStackStatusFile() (StatusId, StatusMsg)
}

// Package hot implements functions to perform HEAT template generation, retrieval
// and deletion operations.
package hot

import (
	"github.com/sirupsen/logrus"
	"os"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/util"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/orchestrator"
)

// A Service provides function signatures for hot related operations.
type Service interface {
	Generate() (models.StatusId, models.StatusMsg)
	GetDescriptor() (models.StatusId, models.StatusMsg, *os.File)
	DeleteDescriptor() (models.StatusId, models.StatusMsg)
	DeleteDescriptorIfExists() (models.StatusId, models.StatusMsg)
}

type service struct {
	repo models.Repository
}

const (
	// HEAT template file name
	heatTemplate = "stack-flame-platform.yaml"

	// tenant openrc file name
	tenantOpenRC = "tenant-openrc"

	// API request states - unlock/lock
	// e.g. createInProgress is set to lock when HEAT generate request
	// starts processing and set to unlock when request returns
	unlock, lock = 0, 1
)

var (
	// to store logger instance
	logger *logrus.Logger

	// to store sanity service handle
	sanitySer models.SanityService

	// to store path to heat directory
	heatDirPath string

	// to store state of HEAT create/delete API request
	createInProgress, deleteInProgress uint32
)

// NewService initializes hot service and returns its handle.
//
// Parameters:
//  glogger: Logger instance.
//  r: Storage service handle.
//  sty: Sanity service handle.
//  dirPath: Path to heat directory.
//
// Returns:
//  Service: Hot service handle.
//  error: Error(if any), otherwise nil.
func NewService(glogger *logrus.Logger, r models.Repository, sty models.SanityService, dirPath string) (Service, error) {
	logger = glogger

	sanitySer = sty
	heatDirPath = dirPath

	return &service{r}, nil
}

// Generate generates HEAT template for uploaded infra descriptor.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// This function generates HEAT template only if Sanity-Check has passed
// successfully and result has no warning. Post successful generation,
// it updates cluster flavors and node password in storage.
func (s *service) Generate() (models.StatusId, models.StatusMsg) {
	logger.Debug("Called Generate() successfully!")

	// Create exclusive temp file, write bytes and move to actual file.
	// This is to reject a request if another request has already created
	// temp file and in progress.
	file, err := os.OpenFile(heatTemplate+"-temp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("Error in opening temp HEAT template file. %v", err)
		errMsg := "Request is already in progress"
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}
	defer func() {
		file.Close()
		if _, err := os.Stat(heatTemplate + "-temp"); err == nil {
			_ = os.Remove(heatTemplate + "-temp")
		}
	}()

	// HOT will be generated only if Sanity-check has passed.
	sanityStId, sanityStMsg := sanitySer.CheckSanityCheckResult()
	if sanityStId != models.NO_ERR {
		logger.Errorf("Sanity-Check result check has returned error")
		return models.REQ_FORBIDDEN, models.StatusMsg(sanityStMsg)
	}

	// Get infra context instance
	infraCtxt := util.InitInfraContext(logger, s.repo)

	// pass infraCtxt to PopulateInfraStore() to get the infra
	statusId, err := util.PopulateInfraStore(infraCtxt)
	if err != nil {
		logger.Errorf("Error in getting Infra Context. %v", err)
		if statusId == util.UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES {
			statusId = models.REQ_FORBIDDEN
		}
		return statusId, models.StatusMsg(err.Error())
	}

	// generate node password
	nodePasswd := generateNodePasswd()

	var template *[]byte = nil

	template, err = generateHeatTemplate(infraCtxt, nodePasswd)
	if err != nil {
		logger.Errorf("Error returned by generateHeatTemplate(). %v", err)
		return HOT_SERVICE_TEMPLATE_GEN_ERR, models.StatusMsg(err.Error())
	}

	_, err = file.Write(*template)
	if err != nil {
		logger.Errorf("Error in writing bytes into HEAT template file: %v", err)
		errMsg := "Failed to write bytes into HEAT template file"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}

	// Move temp file to actual file
	err = os.Rename(heatTemplate+"-temp", heatTemplate)
	if err != nil {
		logger.Errorf("Error in moving temp HEAT template file to actual file. %v", err)
		errMsg := "Failed to rename temp HEAT template file"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}

	// Remove existing cluster flavors from DB
	err = deleteClusterFlavorsFromDB(s.repo)
	if err != nil {
		logger.Errorf("Error in deleting cluster flavors from DB. %v", err)
		errMsg := "Failed to delete cluster flavors from DB"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}

	// Add cluster flavors to DB
	err = addClusterFlavorsToDB(infraCtxt.Store.Computes, s.repo)
	if err != nil {
		logger.Errorf("Error in adding cluster flavors to DB. %v", err)
		errMsg := "Failed to add cluster flavors to DB"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}

	// Add node-passwd to DB
	err = addNodePasswdToDB(nodePasswd, s.repo)
	if err != nil {
		logger.Errorf("Error in adding node password to DB. %v", err)
		errMsg := "Failed to add node password to DB"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}

	return models.NO_ERR, models.SUCCESS_MSG
}

// GetDescriptor returns the generated HEAT template.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//  *os.File: Pointer to generated HEAT template file.
func (s *service) GetDescriptor() (models.StatusId, models.StatusMsg, *os.File) {
	logger.Debug("Called GetDescriptor() successfully!")

	// check if HEAT template exists or not
	if _, err := os.Stat(heatTemplate); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", heatTemplate, err)
		errMsg := "Failed to locate HEAT template"
		return HOT_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg), nil
	}
	logger.Debugf("%s file exist", heatTemplate)

	// open HEAT template file
	fd, err := os.Open(heatTemplate)
	if err != nil {
		logger.Errorf("Error in opening %s file. %v", heatTemplate, err)
		errMsg := "Failed to open HEAT template file"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg), nil
	}

	return models.NO_ERR, models.SUCCESS_MSG, fd
}

// DeleteDescriptor deletes the generated HEAT template.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// DeleteDescriptor calls orchestrator function to get stack list from OpenStack.
// It deletes the HEAT template and removes cluster flavors from storage if stack
// list is empty.
func (s *service) DeleteDescriptor() (models.StatusId, models.StatusMsg) {
	logger.Debug("Called DeleteDescriptor() successfully!")

	// check heatTemplate existence. If doesn't exist, return
	if _, err := os.Stat(heatTemplate); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for HEAT template file. %v", err)
		errMsg := "Failed to locate HEAT template"
		return HOT_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
	}
	logger.Debugf("HEAT template exists")

	// Check if tenant openrc exists
	if _, err := os.Stat(tenantOpenRC); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", tenantOpenRC, err)
		errMsg := "Failed to locate " + tenantOpenRC
		return HOT_SERVICE_TENANT_RC_NOT_UPLOADED_ERR, models.StatusMsg(errMsg)
	}

	// Check if HEAT Stack is launched
	stackList, err := orchestrator.GetStackList(tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in retrieving Stack List from OpenStack. %v", err)
		return models.EXTERNAL_ORCH_ERR, models.StatusMsg(err.Error())
	}
	logger.Debugf("Stack List from OpenStack: %v", stackList)

	if len(stackList) != 0 {
		errMsg := "HEAT Stack has already been launched"
		logger.Errorf(errMsg)
		return HOT_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
	}
	logger.Debugf("HEAT Stack is yet not launched")

	// Delete HEAT Template
	err = os.Remove(heatTemplate)
	if err != nil {
		logger.Errorf("Error in removing HEAT template file. %v", err)
		errMsg := "Failed to remove HEAT template"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}
	logger.Debugf("Successfully removed HEAT template")

	// Remove existing cluster flavors from DB
	err = deleteClusterFlavorsFromDB(s.repo)
	if err != nil {
		logger.Errorf("Error in deleting cluster flavors from DB. %v", err)
		errMsg := "Failed to delete cluster flavors from DB"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}

	return models.NO_ERR, models.SUCCESS_MSG

}

// DeleteDescriptorIfExists deletes the HEAT template if exists.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// DeleteDescriptorIfExists is called by infra service before deleting infra descriptor.
// It deletes the template if exists, else returns without error.
func (s *service) DeleteDescriptorIfExists() (models.StatusId, models.StatusMsg) {
	logger.Debug("Called DeleteDescriptorIfExists() successfully!")

	if _, err := os.Stat(heatTemplate); os.IsNotExist(err) {
		logger.Debugf("HEAT template doesn't exist")
		return models.NO_ERR, models.SUCCESS_MSG
	}
	logger.Debugf("HEAT template file exists")

	// Delete HEAT Template
	err := os.Remove(heatTemplate)
	if err != nil {
		logger.Errorf("Error in removing HEAT template. %v", err)
		errMsg := "Failed to remove HEAT template file"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}
	logger.Debugf("Successfully removed HEAT template file")

	// Remove existing cluster flavors from DB
	err = deleteClusterFlavorsFromDB(s.repo)
	if err != nil {
		logger.Errorf("Error in deleting cluster flavors from DB. %v", err)
		errMsg := "Failed to delete cluster flavors from DB"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}

	return models.NO_ERR, models.SUCCESS_MSG
}

// Package stack orchestrates stack as Openstack tenant. It implements
// functions to perform stack operations, e.g. stack creation,
// deletion and status retrieval.
package stack

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"sync/atomic"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/orchestrator"
)

// A Create type contains the name of stack to be created in Openstack.
type Create struct {
	Name string `json:"name"`
}

// A Delete type contains the name of stack to be deleted from OpenStack.
type Delete struct {
	Name string `json:"name"`
}

// A Service interface provides function signatures for stack related operations.
type Service interface {
	Create(name Create) (models.StatusId, models.StatusMsg)
	Delete(name Delete) (models.StatusId, models.StatusMsg)
	Status(name string) (models.StatusId, models.StatusMsg)
	DeleteStackStatusFile() (models.StatusId, models.StatusMsg)
}

type service struct {
	repo models.Repository
}

const (
	// HEAT template file name
	heatTemplate = "stack-flame-platform.yaml"

	// admin openrc file name
	adminOpenRC = "admin-openrc"

	// tenanat openrc file name
	tenantOpenRC = "tenant-openrc"

	// API request states - unlock/lock
	// e.g. createInProgress is set to lock when stack create request
	// starts and set to unlock when request returns
	unlock, lock = 0, 1
)

var (
	// to store logger instance
	logger *logrus.Logger

	// to manage API request's current status
	createInProgress, deleteInProgress uint32
)

var (
	// orchGetStackList is a method of type 'orchestrator.GetStackList'
	orchGetStackList = orchestrator.GetStackList
)

// NewService initializes stack service & returns its handle.
//
// Parameters:
//  glogger: Logger instance.
//  r: Storage service handle.
//
// Returns:
//  Service: Stack service handle.
//  error: Error(if any), otherwise nil.
func NewService(glogger *logrus.Logger, r models.Repository) (Service, error) {
	logger = glogger

	return &service{r}, nil
}

// Create creates stack in OpenStack with name provided in the API request.
//
// Parameters:
//  name: Stack name to create.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// Before proceeding with stack creation, this function verifies if generated
// HEAT template is valid. If yes, it initiates goroutine for
// stack creation and returns.
func (s *service) Create(name Create) (models.StatusId, models.StatusMsg) {

	logger.Debugf("Called Create() successfully! Stack to create : %s", name.Name)

	// Check if another stack create request is already in progress
	// If yes, reject request and return
	logger.Debugf("Locking createInProgress")
	if atomic.CompareAndSwapUint32(&createInProgress, unlock, lock) == false {
		logger.Errorf("Error in processing stack create request. " +
			"createInProgress is locked.")
		errMsg := "Request is already in progress"
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}

	// Check if stack delete request is in progress
	// If yes, reject stack create request and return
	if deleteInProgress == lock {
		logger.Errorf("Error in processing stack create request. " +
			"deleteInProgress is locked.")
		errMsg := "Stack delete is in progress"
		unlockCreateInProgress()
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}

	if name.Name == "" {
		logger.Debugf("Stack name is an empty string")
		errMsg := "Empty stack name"
		unlockCreateInProgress()
		return STACK_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
	}

	// Check if tenant openrc exists
	if _, err := os.Stat(tenantOpenRC); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", tenantOpenRC, err)
		errMsg := "Failed to locate " + tenantOpenRC
		unlockCreateInProgress()
		return STACK_SERVICE_TENANT_RC_NOT_UPLOADED_ERR, models.StatusMsg(errMsg)
	}

	// Check if HEAT template is generated or not
	if _, err := os.Stat(heatTemplate); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", heatTemplate, err)
		errMsg := "HEAT template is not generated"
		unlockCreateInProgress()
		return STACK_SERVICE_HEAT_TEMPLATE_NOT_GENERATED_ERR, models.StatusMsg(errMsg)
	}

	// Check if generated HEAT template is valid or not
	isValid, err := checkIfValidHeatTemplate(heatTemplate)
	if err != nil {
		logger.Errorf("Error returned by checkIfValidHeatTemplate(). %v", err)
		unlockCreateInProgress()
		return models.INT_SERVER_ERR, models.StatusMsg(err.Error())
	}
	if isValid == false {
		errMsg := "Empty field(s) found in HEAT template"
		logger.Errorf("HEAT template is invalid. %s", errMsg)
		unlockCreateInProgress()
		return STACK_SERVICE_INVALID_HEAT_TEMPLATE_ERR, models.StatusMsg(errMsg)
	}

	// Check with OpenStack if stack to create already exists
	// If yes, reject request and return
	list, err := orchGetStackList(tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in listing stack")
		unlockCreateInProgress()
		return models.EXTERNAL_ORCH_ERR, models.StatusMsg(err.Error())
	}
	logger.Debugf("list of stack returned by GetStackList(): %v", list)

	if len(list) != 0 {
		logger.Debugf("GetStackList() returned non empty stack list. Stack already exists")
		errMsg := "Stack already exists"
		unlockCreateInProgress()
		return STACK_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
	}

	// Initiating stack launch
	go initiateStackCreation(name.Name, s.repo)

	return models.NO_ERR, models.SUCCESS_MSG
}

// Delete deletes stack from OpenStack.
//
// Parameters:
//  name: Stack name to delete.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// Delete initiates goroutine for stack deletion if stack to be deleted
// exists in OpenStack and returns.
func (s *service) Delete(name Delete) (models.StatusId, models.StatusMsg) {

	logger.Debugf("Called Delete() successfully! Stack to delete : %s", name.Name)

	// Check if another stack delete request is already in progress
	// If yes, reject request and return
	logger.Debugf("Locking deleteInProgress")
	if atomic.CompareAndSwapUint32(&deleteInProgress, unlock, lock) == false {
		logger.Errorf("Error in processing stack delete request. " +
			"deleteInProgress is locked.")
		errMsg := "Request is already in progress"
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}

	if name.Name == "" {
		logger.Debugf("Stack name is an empty string")
		errMsg := "Empty stack name"
		unlockDeleteInProgress()
		return STACK_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
	}

	// Check if tenant openrc exists
	if _, err := os.Stat(tenantOpenRC); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", tenantOpenRC, err)
		errMsg := "Failed to locate " + tenantOpenRC
		unlockDeleteInProgress()
		return STACK_SERVICE_TENANT_RC_NOT_UPLOADED_ERR, models.StatusMsg(errMsg)
	}

	// Check with OpenStack if stack to delete exists or not
	// If stack doesn't exist, reject request and return
	list, err := orchGetStackList(tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in listing stack")
		unlockDeleteInProgress()
		return models.EXTERNAL_ORCH_ERR, models.StatusMsg(err.Error())
	}

	if len(list) == 0 {
		logger.Infof("GetStackList() returned empty stack list. Stack does not exist")
		errMsg := "Stack does not exist"
		unlockDeleteInProgress()
		return STACK_SERVICE_STACK_DOES_NOT_EXIST_ERR, models.StatusMsg(errMsg)
	}

	// Iterate list to check whether stack to delete exists or not.
	// If stack is not there in the list, return error
	if list[0]["Stack Name"] != name.Name {
		logger.Infof("List returned by GetStackList() does not contain stack to delete")
		errMsg := "Stack does not exist"
		unlockDeleteInProgress()
		return STACK_SERVICE_STACK_DOES_NOT_EXIST_ERR, models.StatusMsg(errMsg)
	}

	// Initiating stack delete
	go initiateStackDeletion(name.Name, s.repo)

	return models.NO_ERR, models.SUCCESS_MSG
}

// Status gets status for requested stack from OpenStack and returns it.
//
// Parameters:
//  name: Stack name whose status is to be retrieved.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
func (s *service) Status(name string) (models.StatusId, models.StatusMsg) {

	logger.Debugf("Called Status() successfully! Get status for stack : %s", name)

	var stackStatus string

	// Check if tenant openrc exists
	if _, err := os.Stat(tenantOpenRC); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", tenantOpenRC, err)
		errMsg := "Failed to locate " + tenantOpenRC
		return STACK_SERVICE_TENANT_RC_NOT_UPLOADED_ERR, models.StatusMsg(errMsg)
	}

	// Check if stack create/delete status file exists
	// If either if file exists, read status

	if _, err := os.Stat(stackCreateStatusFile); err == nil {
		// Read status and send response
		fileBytes, err := ioutil.ReadFile(stackCreateStatusFile)
		if err != nil {
			errMsg := "Error in reading " + stackCreateStatusFile + " file"
			logger.Errorf("%s. %v", errMsg, err)
			return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
		}

		status := StackStatus{}
		err = json.Unmarshal(fileBytes, &status)
		if err != nil {
			errMsg := "Error in unmarshaling " + stackCreateStatusFile + " file"
			logger.Errorf("%s. %v", errMsg, err)
			return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
		}

		if status.Name != name {
			errMsg := "Stack does not exist"
			logger.Debugf("%s", errMsg)
			return STACK_SERVICE_STACK_DOES_NOT_EXIST_ERR, models.StatusMsg(errMsg)
		}

		if status.Status.Id == 0 && status.Status.Msg == "" {
			logger.Debugf("%s file exists but empty", stackCreateStatusFile)
			stackStatus = "Stack create request is in progress"
			return models.NO_ERR, models.StatusMsg(stackStatus)
		}

		if status.Status.Id != models.NO_ERR {
			return status.Status.Id, status.Status.Msg
		} else {
			// Get stack status from OpenStack
			osStackStatus, err := orchestrator.GetStackStatus(name, tenantOpenRC)
			if err != nil {
				logger.Errorf("Error in getting stack status. %v", err)
				return models.EXTERNAL_ORCH_ERR, models.StatusMsg(err.Error())
			}
			// Append stack status reason also in case of CREATE_FAILED
			if osStackStatus["stack_status"] == "CREATE_FAILED" {
				stackStatus = osStackStatus["stack_status"].(string) + ": " + osStackStatus["stack_status_reason"].(string)
			} else {
				stackStatus = osStackStatus["stack_status"].(string)
			}
			return models.NO_ERR, models.StatusMsg(stackStatus)
		}

	} else if _, err := os.Stat(stackDeleteStatusFile); err == nil {

		// Read status and send response
		fileBytes, err := ioutil.ReadFile(stackDeleteStatusFile)
		if err != nil {
			errMsg := "Error in reading " + stackDeleteStatusFile + " file"
			logger.Errorf("%s. %v", errMsg, err)
			return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
		}

		status := StackStatus{}
		err = json.Unmarshal(fileBytes, &status)
		if err != nil {
			errMsg := "Error in unmarshaling " + stackDeleteStatusFile + " file"
			logger.Errorf("%s. %v", errMsg, err)
			return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
		}

		if status.Name != name {
			errMsg := "Stack does not exist"
			logger.Debugf("%s", errMsg)
			return STACK_SERVICE_STACK_DOES_NOT_EXIST_ERR, models.StatusMsg(errMsg)
		}

		if status.Status.Id == 0 && status.Status.Msg == "" {
			logger.Debugf("%s file exists but empty", stackDeleteStatusFile)
			stackStatus = "Stack delete request is in progress"
			return models.NO_ERR, models.StatusMsg(stackStatus)
		}

		if status.Status.Id != models.NO_ERR {
			return status.Status.Id, status.Status.Msg
		} else {
			// Get stack status from OpenStack
			// Call orchestrator GetStackStatus()
			osStackStatus, err := orchestrator.GetStackStatus(name, tenantOpenRC)
			if err != nil {
				logger.Errorf("Error in getting stack status. %v", err)
				return models.EXTERNAL_ORCH_ERR, models.StatusMsg(err.Error())
			}
			// Append stack status reason also in case of CREATE_FAILED
			if osStackStatus["stack_status"] == "DELETE_FAILED" {
				stackStatus = osStackStatus["stack_status"].(string) + ": " + osStackStatus["stack_status_reason"].(string)
			} else {
				stackStatus = osStackStatus["stack_status"].(string)
			}
			return models.NO_ERR, models.StatusMsg(stackStatus)
		}
	}

	logger.Debugf("Neither %s nor %s file exists", stackCreateStatusFile, stackDeleteStatusFile)
	stackStatus = "Stack does not exist"

	return STACK_SERVICE_STACK_DOES_NOT_EXIST_ERR, models.StatusMsg(stackStatus)
}

// DeleteStackStatusFile deletes stack create/delete status file.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// DeleteStackStatusFile is called by infra service before deleting infra descriptor.
func (s *service) DeleteStackStatusFile() (models.StatusId, models.StatusMsg) {
	logger.Debug("Called DeleteStackStatusFile() successfully")

	// Remove stack status create/delete file if exists
	if _, err := os.Stat(stackDeleteStatusFile); err == nil {
		err = os.Remove(stackDeleteStatusFile)
		if err != nil {
			logger.Errorf("Error in removing %s file. %v", stackDeleteStatusFile, err)
			errMsg := "Failed to remove " + stackDeleteStatusFile + " file"
			return STACK_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
		}
	}
	if _, err := os.Stat(stackCreateStatusFile); err == nil {
		err = os.Remove(stackCreateStatusFile)
		if err != nil {
			logger.Errorf("Error in removing %s file. %v", stackCreateStatusFile, err)
			errMsg := "Failed to remove " + stackCreateStatusFile + " file"
			return STACK_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
		}
	}

	return models.NO_ERR, models.SUCCESS_MSG
}

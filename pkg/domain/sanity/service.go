// Package sanity implements functions for sanity check initiation and retrieval of
// its status and result.
package sanity

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync/atomic"

	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/util"
)

const (
	// API request states - unlock/lock
	// e.g. initiateInProgress is set to lock when Sanity-Check request
	// starts processing and set to unlock when request returns
	unlock, lock = 0, 1
)

// A Service provides function signatures for sanity related operations.
type Service interface {
	Initiate() (models.StatusId, models.StatusMsg)
	Status() (models.StatusId, models.StatusMsg)
	Results() (models.StatusId, models.StatusMsg, SanityCheckResultPayload)
	CheckSanityCheckResult() (models.StatusId, models.StatusMsg)
	DeleteSanityCheckResult() (models.StatusId, models.StatusMsg)
}

type service struct {
	repo models.Repository
}

// A SanityCheckResultPayload type is used to store Sanity-Check result.
type SanityCheckResultPayload struct {
	Result  Res    `json:"Result"`
	Warning []Warn `json:"Warning"`
}

var (
	// to store logger instance
	logger *logrus.Logger

	// to store heat directory path
	heatDirPath string

	// to store Sanity-Check API request state
	initiateInProgress uint32
)

// NewService initializes sanity service and returns its handle.
//
// Parameters:
//  glogger: Logger instance.
//  r: Storage service handle.
//  dirPath: Path to heat directory.
//
// Returns:
//  Service: Sanity service handle.
//  error: Error(if any), otherwise nil.
func NewService(glogger *logrus.Logger, r models.Repository, dirPath string) (Service, error) {
	logger = glogger
	heatDirPath = dirPath

	return &service{r}, nil
}

// Initiate starts Sanity-Check process for the OpenStack setup.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// This function initiates goroutine for Sanity-Check process and returns.
func (s *service) Initiate() (models.StatusId, models.StatusMsg) {
	logger.Debug("Called Initiate() successfully!")

	// Check if another sanity-check request is already in progress
	// If yes, reject request and return
	// initiateInProgress will be unlocked when sanity process will complete
	logger.Debugf("Locking initiateInProgress")
	if atomic.CompareAndSwapUint32(&initiateInProgress, unlock, lock) == false {
		logger.Errorf("Error in processing sanity-check request. " +
			"initiateInProgress is locked.")
		errMsg := "Request is already in progress"
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}

	// Get infra context instance
	infraCtxt := util.InitInfraContext(logger, s.repo)

	// pass infraCtxt to PopulateInfraStore() to get the infra from storage
	statusId, err := util.PopulateInfraStore(infraCtxt)
	if err != nil {
		logger.Errorf("Error in getting Infra Context. %v", err)
		unlockInitiateInProgress()
		return statusId, models.StatusMsg(err.Error())
	}

	// Initiating sanity-check
	go initiateSanityCheck(infraCtxt, s.repo)

	return models.NO_ERR, models.SUCCESS_MSG
}

// Status returns Sanity-Check status.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// This function interprets Sanity-Check status using Sanity-Check result file
// and returns it.
func (s *service) Status() (models.StatusId, models.StatusMsg) {
	logger.Debug("Called Status() successfully!")

	var status string

	// check if sanity-check is in progress
	if initiateInProgress == lock {
		status = "Sanity-Check is in progress"
		logger.Debugf("%s", status)
		return models.NO_ERR, models.StatusMsg(status)
	}

	// check if sanity-check has ever been performed by checking
	// existance of sanity result file
	if _, err := os.Stat(sanityResultFile); os.IsNotExist(err) {
		status = "Sanity-Check not initiated"
		logger.Debugf("%s", status)
		return SANITY_SERVICE_INVALID_REQ_ERR, models.StatusMsg(status)
	}

	// Read sanity result file and send response
	file, err := ioutil.ReadFile(sanityResultFile)
	if err != nil {
		errMsg := "Error in reading sanity result file"
		logger.Errorf("%s. %v", errMsg, err)
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}
	result := SanityResult{}
	err = json.Unmarshal([]byte(file), &result)
	if err != nil {
		errMsg := "Error in unmarshaling sanity result"
		logger.Errorf("%s. %v", errMsg, err)
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}
	if result.Result.Id != models.NO_ERR {
		status = "Sanity-Check failed"
		logger.Debugf("%s", status)
		return SANITY_SERVICE_SANITY_CHECK_ERR, models.StatusMsg(status)
	}

	status = "Sanity-Check completed"
	return models.NO_ERR, models.StatusMsg(status)
}

// Results returns Sanity-Check result.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//  SanityCheckResultPayload: Sanity-Check result containing status and warning(if any).
//
// This function populates SanityCheckResultPayload with sanity-check result and returns it.
func (s *service) Results() (models.StatusId, models.StatusMsg, SanityCheckResultPayload) {
	logger.Debug("Called Results() successfully")

	result := SanityCheckResultPayload{}

	// check if sanity-check is in progress
	if initiateInProgress == lock {
		errMsg := "Sanity-Check is in progress"
		logger.Debugf("%s", errMsg)
		return SANITY_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg), result
	}

	// check if sanity result file exists
	if _, err := os.Stat(sanityResultFile); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for sanity result file. %v", err)
		statusMsg := "Sanity-Check not initiated"
		return SANITY_SERVICE_INVALID_REQ_ERR, models.StatusMsg(statusMsg), result
	}

	file, err := ioutil.ReadFile(sanityResultFile)
	if err != nil {
		errMsg := "Error in reading sanity result file"
		logger.Errorf("%s. %v", errMsg, err)
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg), result
	}
	err = json.Unmarshal(file, &result)
	if err != nil {
		errMsg := "Error in unmarshaling sanity result"
		logger.Errorf("%s. %v", errMsg, err)
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg), result
	}

	return models.NO_ERR, models.StatusMsg(""), result
}

// CheckSanityCheckResult returns Sanity-Check result.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// CheckSanityCheckResult is called by hot service before generating HEAT template.
func (s *service) CheckSanityCheckResult() (models.StatusId, models.StatusMsg) {
	logger.Debug("Called CheckSanityCheckResult() successfully")

	stId, stMsg, result := s.Results()

	if stId != models.NO_ERR {
		logger.Errorf("Error returned by Result()")
		return stId, stMsg
	}

	status := "Sanity-Check successfully completed"

	if result.Result.Id != models.NO_ERR {
		status = "Sanity-Check results has not passed successfully"
		logger.Errorf("%s", status)

		return result.Result.Id, models.StatusMsg(status)
	}
	if result.Warning != nil && len(result.Warning) != 0 {
		status = "Sanity-Check results has warnings"
		logger.Errorf("%s", status)

		return SANITY_SERVICE_SANITY_CHECK_WARN, models.StatusMsg(status)
	}

	return models.NO_ERR, models.StatusMsg(status)
}

// DeleteSanityCheckResult deletes Sanity-Check result file.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// DeleteSanityCheckResult is called by infra service before deleting infra descriptor.
func (s *service) DeleteSanityCheckResult() (models.StatusId, models.StatusMsg) {
	logger.Debug("Called DeleteSanityCheckResult() successfully")

	// Remove sanity result file if exists
	if _, er := os.Stat(sanityResultFile); er == nil {
		err := os.Remove(sanityResultFile)
		if err != nil {
			logger.Errorf("Error in removing sanity result file. %v", err)
			errMsg := "Failed to remove sanity-result file"
			return SANITY_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
		}
	}

	return models.NO_ERR, models.SUCCESS_MSG
}

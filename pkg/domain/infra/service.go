// Package infra implements functions for uploading infra descriptor and openrc
// files required to use the OpenStack CLI.
package infra

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/orchestrator"
)

const (
	// admin openrc file name
	adminOpenRC = "admin-openrc"

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

	// to store state of infra descriptor process API request
	descriptorInProgress uint32

	// to store state of infra descriptor delete API request
	deleteDescriptorInProgress uint32

	// hot service handle
	hotSer models.HotService

	// sanity service handle
	stySer models.SanityService

	// stack service handle
	stSer models.StackService

	// to store infra descriptor definitions file path
	descDefFilePath string
)

// A Service provides function signatures for infra related operations.
type Service interface {
	ProcessDescriptor(fileBytes []byte) (models.StatusId, models.StatusMsg)
	DeleteDescriptor() (models.StatusId, models.StatusMsg)
	ProcessAdminRC(fileBytes []byte) (models.StatusId, models.StatusMsg)
	DeleteAdminRC() (models.StatusId, models.StatusMsg)
	ProcessTenantRC(fileBytes []byte) (models.StatusId, models.StatusMsg)
	DeleteTenantRC() (models.StatusId, models.StatusMsg)
}

type service struct {
	repo models.Repository
}

// NewService initialises infra service and returns its handle.
//
// Parameters:
//  glogger: Logger instance.
//  r: Storage service handle.
//  ht: Hot service handle.
//  sty: Sanity service handle.
//  st: Stack service handle.
//  filePath: Path to infra descriptor definitions file.
//
// Returns:
//  Service: Infra service handle.
//  error: Error(if any), otherwise nil.
func NewService(glogger *logrus.Logger, r models.Repository, ht models.HotService, sty models.SanityService, st models.StackService, filePath string) (Service, error) {

	logger = glogger

	// Setting infra descriptor file path
	descDefFilePath = filePath

	err := parseDefinitions()
	if err != nil {
		logger.Errorf("Error in parsing infra descriptor definitions. %v", err)
		return nil, err
	}

	hotSer = ht
	stySer = sty
	stSer = st

	return &service{r}, nil
}

// ProcessDescriptor processes received infra descriptor and populates storage with it.
//
// Parameters:
//  fileBytes: Infra descriptor as byte array.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// ProcessDescriptor parses and validates the received infra descriptor
// to further populate storage with it.
func (s *service) ProcessDescriptor(fileBytes []byte) (models.StatusId, models.StatusMsg) {

	// Process Infra Descriptor here (save information into DB)
	// Return error if occurs.

	logger.Debug("Called ProcessDescriptor() successfully!")

	// Check if another infra descriptor upload request is already in progress
	// If yes, reject request and return
	logger.Debugf("Locking descriptorInProgress")
	if atomic.CompareAndSwapUint32(&descriptorInProgress, unlock, lock) == false {
		logger.Errorf("Error in processing infra descriptor upload request. " +
			"descriptorInProgress is locked.")
		errMsg := "Request is already in progress"
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}
	defer func() {
		// Unlock descriptorInProgress
		logger.Debugf("Unlocking descriptorInProgress")
		_ = atomic.SwapUint32(&descriptorInProgress, unlock)
	}()

	// Check if descriptor is already processed and DB is populated
	logger.Debug("Check if descriptor is already processed")
	isProcessed, err := isDescriptorAlreadyProcessed(s.repo)
	if err != nil {
		logger.Errorf("Error in checking if descriptor is already processed. %v", err)
		return models.INT_SERVER_DB_ERR, models.StatusMsg(err.Error())
	}

	if isProcessed == true {
		logger.Debugf("Infra Descriptor is already processed")
		logger.Debugf("Deleting existing infra before updating it")
		// removing existing infra descriptor before updating it
		statusCode, statusMsg := s.DeleteDescriptor()
		if statusCode != models.NO_ERR {
			logger.Error("Error in deleting Infra Descriptor")
			return statusCode, statusMsg
		}
	}

	// Parse and validate descriptor
	logger.Debug("Calling parseAndValidate() to parse and validate descriptor")
	descriptor := yml{}
	err = parseAndValidate(fileBytes, &descriptor)
	if err != nil {
		logger.Errorf("Error in parsing infra descriptor. %v", err)
		return INFRA_SERVICE_INFRA_DESC_PARSE_ERR, models.StatusMsg(err.Error())
	}
	logger.Debugf("infra descriptor: %+v", descriptor)

	// Add InfraServices and Metadata by filling in InfraService and Config structures resp.
	fields := reflect.ValueOf(&descriptor.InfraServices).Elem()
	infraServices := make([]models.InfraService, fields.NumField())
	for i := 0; i < fields.NumField(); i++ {
		infraServices[i].ServiceType = fields.Type().Field(i).Tag.Get("serviceType")
		infraServices[i].Value = fields.Field(i).Interface().(string)
	}
	logger.Debugf("infraServices: %+v", infraServices)

	fields = reflect.ValueOf(&descriptor.Metadata).Elem()
	//metadata := make([]models.Config, fields.NumField())
	metadata := make([]models.Config, 0)
	// check if ipv4-rules has come in descriptor. If yes, set isIpv4RulesExist
	// to true
	isIpv4RulesExist := false
	for i := 0; i < fields.NumField(); i++ {
		if fields.Type().Field(i).Tag.Get("confKey") == "mtu" ||
			fields.Type().Field(i).Tag.Get("confKey") == "dhcp_agents" {
			conf := models.Config{}

			conf.ConfKey = fields.Type().Field(i).Tag.Get("confKey")
			logger.Debugf("Conf Key: '%s' found in infra descriptor", conf.ConfKey)
			conf.Value = strconv.Itoa(fields.Field(i).Interface().(int))

			metadata = append(metadata, conf)
		} else if fields.Type().Field(i).Tag.Get("confKey") == "enable-ipv4-rules" {
			conf := models.Config{}

			logger.Debug("Conf Key: 'enable-ipv4-rules' found in infra descriptor")
			logger.Debugf("enable-ipv4_rules: %s", fields.Field(i).Interface().(string))
			if fields.Field(i).Interface().(string) != "" {
				conf.ConfKey = "enable-ipv4-rules"
				conf.Value = fields.Field(i).Interface().(string)

				logger.Debug("Descriptor has ipv4-rules in metedata")
				isIpv4RulesExist = true

				metadata = append(metadata, conf)
			}
		} else {
			conf := models.Config{}

			conf.ConfKey = fields.Type().Field(i).Tag.Get("confKey")
			logger.Debugf("Conf Key: '%s' found in infra descriptor", conf.ConfKey)
			conf.Value = fields.Field(i).Interface().(string)

			metadata = append(metadata, conf)
		}
	}
	logger.Debugf("metadata: %+v", metadata)

	// Add Infra Descriptor to DB.
	desc := make([]interface{}, 0)

	computeSlice := make([]models.Compute, 0)
	for _, v := range descriptor.ComputeNodes {
		computeSlice = append(computeSlice, v)
	}
	desc = append(desc, computeSlice)

	networkSlice := make([]models.Network, 0)
	for _, v := range descriptor.Networks {
		networkSlice = append(networkSlice, v)
	}
	desc = append(desc, networkSlice)

	subnetSlice := make([]models.Subnet, 0)
	for _, v := range descriptor.Subnets {
		subnetSlice = append(subnetSlice, v)
	}
	desc = append(desc, subnetSlice)

	securityGroupSlice := make([]models.SecurityGroup, 0)
	for _, v := range descriptor.SecurityGroups {
		securityGroupSlice = append(securityGroupSlice, v)
	}
	desc = append(desc, securityGroupSlice)

	desc = append(desc, infraServices)

	var ipv4Rules interface{}
	if isIpv4RulesExist == true {
		// before removing, get value from db
		config := models.Config{ConfKey: "enable-ipv4-rules"}
		q := models.Query{Entity: config}
		ipv4Rules, err = s.repo.Get(&q)
		if err != nil {
			logger.Errorf("Error returned by Get() Interface. %v", err)
			errMsg := "Failed to retrieve enable-ipv4-rules from storage"
			return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
		}
		// remove existing config for 'enable-ipv4-rules' from db
		err = s.repo.Remove(models.Config{ConfKey: "enable-ipv4-rules"})
		if err != nil {
			logger.Errorf("Error in deleting 'enable-ipv4-rules' Configuration from Storage: %v", err)
			errMsg := "Failed to delete 'enable-ipv4-rules' Configuration from Storage"
			return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
		}
	}
	desc = append(desc, metadata)

	err = s.repo.Add(desc)
	if err != nil {
		logger.Errorf("Error in adding descriptor to DB. %v", err)
		// if Add() fails, re-insert already existing enable-ipv4-rules to DB
		if ipv4Rules != nil {
			ifc := make([]interface{}, 0)
			ifc = append(ifc, ipv4Rules)
			err = s.repo.Add(ifc)
			if err != nil {
				logger.Errorf("Error returned by Add() Interface. %v", err)
				errMsg := "Failed to add existing 'enable-ipv4-rules' Configuration to Storage"
				return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
			}
		}
		errMsg := "Failed to add descriptor to DB"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}

	return models.NO_ERR, models.SUCCESS_MSG
}

// DeleteDescriptor deletes infra descriptor from storage.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
//
// Before deleting infra descriptor, this function deletes HEAT template,
// sanity-check result file and stack creation/deletion status file if exist.
func (s *service) DeleteDescriptor() (models.StatusId, models.StatusMsg) {
	logger.Debugf("Successfully called DeleteDescriptor()")

	// Check if another Delete Infra Descriptor Request is already in progress
	// If yes, reject request and return
	logger.Debugf("Locking deleteDescriptorInProgress")
	if atomic.CompareAndSwapUint32(&deleteDescriptorInProgress, unlock, lock) == false {
		logger.Errorf("Error in deleting Infra Descriptor: " +
			"deleteDescriptorInProgress is locked.")
		errMsg := "Request is already in progress"
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}
	defer func() {
		// Unlock deleteDescriptorInProgress
		logger.Debugf("Unlocking deleteDescriptorInProgress")
		_ = atomic.SwapUint32(&deleteDescriptorInProgress, unlock)
	}()

	// Check if tenant openrc exists
	if _, err := os.Stat(tenantOpenRC); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", tenantOpenRC, err)
		errMsg := "Failed to locate " + tenantOpenRC
		return INFRA_SERVICE_TENANT_RC_NOT_UPLOADED_ERR, models.StatusMsg(errMsg)
	}

	// Check if HEAT Stack is launched
	stackList, err := orchestrator.GetStackList(tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in retrieving Stack List from OpenStack. %v", err)
		return models.EXTERNAL_ORCH_ERR, models.StatusMsg(err.Error())
	}
	logger.Debugf("Stack List from OpenStack: %v", stackList)

	if len(stackList) != 0 {
		errMsg := "HEAT stack is launched"
		logger.Errorf(errMsg)
		return INFRA_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
	}
	logger.Debugf("HEAT Stack is yet not launched")

	// Delete HEAT Template if already generated
	statusId, statusMsg := hotSer.DeleteDescriptorIfExists()
	if statusId != models.NO_ERR {
		logger.Errorf("Error in removing already generated HEAT template")
		return statusId, statusMsg
	}

	// Delete sanity result if already exists
	statusId, statusMsg = stySer.DeleteSanityCheckResult()
	if statusId != models.NO_ERR {
		logger.Errorf("Error in removing already existing sanity result")
		return statusId, statusMsg
	}

	// Delete stack status file if exists
	statusId, statusMsg = stSer.DeleteStackStatusFile()
	if statusId != models.NO_ERR {
		logger.Errorf("Error in removing already existing stack status file")
		return statusId, statusMsg
	}

	err = s.repo.Remove(models.Compute{Vcpus: -1, RAM: -1, Disk: -1})
	if err != nil {
		logger.Errorf("Error in deleting Compute Nodes from Storage: %v", err)
		errMsg := "Failed to delete Compute Nodes from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	logger.Debug("Successfully deleted all Compute Nodes from Storage")

	err = s.repo.Remove(models.Config{ConfKey: "os-tenant-id"})
	if err != nil {
		logger.Errorf("Error in deleting 'os-tenant-id' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'os-tenant-id' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	err = s.repo.Remove(models.Config{ConfKey: "os-cli-version"})
	if err != nil {
		logger.Errorf("Error in deleting 'os-cli-version' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'os-cli-version' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	err = s.repo.Remove(models.Config{ConfKey: "mtu"})
	if err != nil {
		logger.Errorf("Error in deleting 'mtu' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'mtu' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	err = s.repo.Remove(models.Config{ConfKey: "cidr"})
	if err != nil {
		logger.Errorf("Error in deleting 'cidr' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'cidr' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	err = s.repo.Remove(models.Config{ConfKey: "ardent-version"})
	if err != nil {
		logger.Errorf("Error in deleting 'ardent-version' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'ardent-version' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	err = s.repo.Remove(models.Config{ConfKey: "sia-ip-frontend"})
	if err != nil {
		logger.Errorf("Error in deleting 'sia-ip-frontend' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'sia-ip-frontend' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	err = s.repo.Remove(models.Config{ConfKey: "dhcp_agents"})
	if err != nil {
		logger.Errorf("Error in deleting 'dhcp_agents' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'dhcp_agents' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}

	// remove existing row from config table for confKey 'enable-ipv4-rules'
	// in db and update with default value
	err = s.repo.Remove(models.Config{ConfKey: "enable-ipv4-rules"})
	if err != nil {
		logger.Errorf("Error in deleting 'enable-ipv4-rules' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'enable-ipv4-rules' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	// add the row with default value 0
	ifc := make([]interface{}, 0)
	configSlice := make([]models.Config, 0)
	defaultIpv4Rules := models.Config{ConfKey: "enable-ipv4-rules", Value: "0"}
	configSlice = append(configSlice, defaultIpv4Rules)
	// append configSlice to ifc
	ifc = append(ifc, configSlice)
	err = s.repo.Add(ifc)
	if err != nil {
		logger.Errorf("Error returned by Add() Interface. %v", err)
		errMsg := "Failed to add default 'enable-ipv4-rules' Configuration to Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}

	err = s.repo.Remove(models.Config{ConfKey: "node-passwd"})
	if err != nil {
		logger.Errorf("Error in deleting 'node-passwd' Configuration from Storage: %v", err)
		errMsg := "Failed to delete 'node-passwd' Configuration from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	logger.Debug("Successfully deleted all Configurations from Storage")

	err = s.repo.Remove(models.InfraService{})
	if err != nil {
		logger.Errorf("Error in deleting Infrastructure Services from Storage: %v", err)
		errMsg := "Failed to delete Infrastructure Services from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	logger.Debug("Successfully deleted all Infrastructure Services from Storage")

	err = s.repo.Remove(models.SecurityGroup{})
	if err != nil {
		logger.Errorf("Error in deleting Security Groups from Storage: %v", err)
		errMsg := "Failed to delete Security Groups from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	logger.Debug("Successfully deleted all Security Groups from Storage")

	err = s.repo.Remove(models.Subnet{})
	if err != nil {
		logger.Errorf("Error in deleting Subnets from Storage: %v", err)
		errMsg := "Failed to delete Subnets from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	logger.Debug("Successfully deleted all Subnets from Storage")

	err = s.repo.Remove(models.Network{})
	if err != nil {
		logger.Errorf("Error in deleting Networks from Storage: %v", err)
		errMsg := "Failed to delete Networks from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	logger.Debug("Successfully deleted all Networks from Storage")

	// Retrieving Cluster Flavors from Storage to later remove only them from Storage
	// Static Flavors will remain in Storage
	flavor := models.Flavor{Vcpus: -1, RAM: -1, Disk: -1}
	query := models.Query{Entity: flavor}
	flavors, err := s.repo.Get(&query)
	if err != nil {
		logger.Errorf("Error in retrieving Flavors from Storage: %v", err)
		errMsg := "Failed to retrieve Flavors from Storage"
		return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
	}
	flavorsArr := flavors.([]models.Flavor)
	for i := 0; i < len(flavorsArr); i++ {
		name := flavorsArr[i].Name
		if strings.Contains(name, "flame-cluster") {
			err = s.repo.Remove(models.Flavor{Name: name, Vcpus: -1, RAM: -1, Disk: -1})
			if err != nil {
				logger.Errorf("Error in deleting Flavors from Storage: %v", err)
				errMsg := "Failed to delete Flavors from Storage"
				return models.INT_SERVER_DB_ERR, models.StatusMsg(errMsg)
			}
		}
	}
	logger.Debug("Successfully deleted Cluster Flavors from Storage")

	return models.NO_ERR, models.SUCCESS_MSG
}

// ProcessAdminRC processes received Admin OpenRC and saves it locally in a file.
//
// Parameters:
//  fileBytes: Admin OpenRC as byte array.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
func (s *service) ProcessAdminRC(fileBytes []byte) (models.StatusId, models.StatusMsg) {

	// Process Admin Openrc here (save bytes in a file)
	// Return error if occurs.

	logger.Debug("Called ProcessAdminRC() successfully!")

	// Check if Admin OpenRC has already been processed
	if _, err := os.Stat(adminOpenRC); err == nil {
		logger.Debugf("%s has already been processed", adminOpenRC)
		logger.Debugf("Deleting existing %s before updating it", adminOpenRC)
		// removing existing admin openrc before updating it
		statusCode, statusMsg := s.DeleteAdminRC()
		if statusCode != models.NO_ERR {
			logger.Errorf("Error in deleting %s: %s", adminOpenRC, statusMsg)
			return statusCode, statusMsg
		}
	}

	// Create exclusive temp file, write bytes and move to actual file.
	// This is to reject a request if another request has already created temp file and in progress
	file, err := os.OpenFile(adminOpenRC+"-temp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("Error in opening Temp Admin Openrc file. %v", err)
		errMsg := "Request is already in progress"
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}
	defer func() {
		file.Close()
		if _, err := os.Stat(adminOpenRC + "-temp"); err == nil {
			_ = os.Remove(adminOpenRC + "-temp")
		}
	}()
	_, err = file.Write(fileBytes)
	if err != nil {
		logger.Errorf("Error in writing Temp Admin Openrc file. %v", err)
		errMsg := "Failed to write Temp Admin Openrc file"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}

	// Move temp file to actual file
	err = os.Rename(adminOpenRC+"-temp", adminOpenRC)
	if err != nil {
		logger.Errorf("Error in moving Temp Admin Openrc file to actual file. %v", err)
		errMsg := "Failed to write Admin Openrc file"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}

	return models.NO_ERR, models.SUCCESS_MSG
}

// DeleteAdminRC deletes Admin OpenRC file.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface'.
func (s *service) DeleteAdminRC() (models.StatusId, models.StatusMsg) {
	logger.Debugf("Successfully called DeleteAdminRC()")

	// Check if 'admin-openrc' file exists
	if _, err := os.Stat(adminOpenRC); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", adminOpenRC, err)
		errMsg := "Failed to locate " + adminOpenRC
		return INFRA_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
	} else {
		logger.Debugf("admin-openrc file found")

		// Delete 'admin-openrc' file
		fileDeleteErr := os.Remove(adminOpenRC)
		if fileDeleteErr != nil {
			logger.Errorf("Error in removing 'admin-openrc' file: %v", fileDeleteErr)
			errMsg := "Failed to remove 'admin-openrc' file"
			return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
		}
		logger.Debugf("Successfully removed 'admin-openrc' file")
	}
	return models.NO_ERR, models.SUCCESS_MSG
}

// ProcessTenantRC processes received Tenant OpenRC and saves it locally in a file.
//
// Parameters:
//  fileBytes: Tenant OpenRC as byte array.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface.
func (s *service) ProcessTenantRC(fileBytes []byte) (models.StatusId, models.StatusMsg) {

	// Process Tenant Openrc here (save bytes in a file)
	// Return error if occurs.

	logger.Debug("Called ProcessTenantRC() successfully!")

	// Check if Tenant OpenRC has already been processed
	if _, err := os.Stat(tenantOpenRC); err == nil {
		logger.Debugf("%s has already been processed", tenantOpenRC)
		logger.Debugf("Deleting existing %s before updating it", tenantOpenRC)
		// removing existing tenant openrc before updating it
		statusCode, statusMsg := s.DeleteTenantRC()
		if statusCode != models.NO_ERR {
			logger.Errorf("Error in deleting %s: %s", tenantOpenRC, statusMsg)
			return statusCode, statusMsg
		}
	}

	// Create exclusive temp file, write bytes and move to actual file.
	// This is to reject a request if another request has already created temp file and in progress
	file, err := os.OpenFile(tenantOpenRC+"-temp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("Error in opening Temp Tenant Openrc file. %v", err)
		errMsg := "Request is already in progress"
		return models.REQ_FORBIDDEN, models.StatusMsg(errMsg)
	}
	defer func() {
		file.Close()
		if _, err := os.Stat(tenantOpenRC + "-temp"); err == nil {
			_ = os.Remove(tenantOpenRC + "-temp")
		}
	}()

	_, err = file.Write(fileBytes)
	if err != nil {
		logger.Errorf("Error in writing Temp Tenant Openrc file. %v", err)
		errMsg := "Failed to write Temp Tenant Openrc file"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}

	// Move temp file to actual file
	err = os.Rename(tenantOpenRC+"-temp", tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in moving Temp Tenant Openrc file to actual file. %v", err)
		errMsg := "Failed to write Tenant Openrc file"
		return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
	}

	return models.NO_ERR, models.SUCCESS_MSG
}

// DeleteTenantRC deletes Tenant OpenRC file.
//
// Parameters:
//  Nil.
//
// Returns:
//  models.StatusId: Response Status Id returned by service interface.
//  models.StatusMsg: Response Status Msg returned by service interface'.
func (s *service) DeleteTenantRC() (models.StatusId, models.StatusMsg) {
	logger.Debugf("Successfully called DeleteTenantRC()")

	// Check if 'tenant-openrc' file exists
	if _, err := os.Stat(tenantOpenRC); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", tenantOpenRC, err)
		errMsg := "Failed to locate " + tenantOpenRC
		return INFRA_SERVICE_INVALID_REQ_ERR, models.StatusMsg(errMsg)
	} else {
		logger.Debugf("tenant-openrc file found")

		// Delete 'tenant-openrc' file
		fileDeleteErr := os.Remove(tenantOpenRC)
		if fileDeleteErr != nil {
			logger.Errorf("Error in removing 'tenant-openrc' file: %v", fileDeleteErr)
			errMsg := "Failed to remove 'tenant-openrc' file"
			return models.INT_SERVER_ERR, models.StatusMsg(errMsg)
		}
		logger.Debugf("Successfully removed 'tenant-openrc' file")
	}
	return models.NO_ERR, models.SUCCESS_MSG
}

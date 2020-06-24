package stack

import (
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"sync/atomic"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/orchestrator"
)

// struct for storing stack status
type Res struct {
	Id  models.StatusId  `json:"status_id"`
	Msg models.StatusMsg `json:"status_str"`
}

// StackStatus will contain stack name and it's status
type StackStatus struct {
	Name   string
	Status *Res
}

const (
	// files to store stack creation/deletion status
	stackCreateStatusFile = "stack-create-status"
	stackDeleteStatusFile = "stack-delete-status"
)

func initiateStackCreation(name string, repo models.Repository) {

	logger.Debugf("Inside initiateStackCreation()")

	// deferring unlocking of createInProgress before returning
	defer func() {
		unlockCreateInProgress()
	}()

	var (
		stackStatus       = &StackStatus{}
		isStatusFileExist = false
	)

	// rename stack create status file if already exists
	if _, err := os.Stat(stackCreateStatusFile); err == nil {
		logger.Debugf("%s file already exist", stackCreateStatusFile)
		isStatusFileExist = true
		logger.Debugf("renaming %s file to %s-temp", stackCreateStatusFile, stackCreateStatusFile)
		_ = os.Rename(stackCreateStatusFile, stackCreateStatusFile+"-temp")
	}

	file, err := os.OpenFile(stackCreateStatusFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("Error in opening %s file. %v", stackCreateStatusFile, err)
		// restore old status file if existed
		if isStatusFileExist == true {
			logger.Debugf("restoring already existing %s file", stackCreateStatusFile)
			_ = os.Rename(stackCreateStatusFile+"-temp", stackCreateStatusFile)
		}
		return

	} else {
		stackStatus.Name = name
		stackStatus.Status = &Res{}
		encodeStackStatusAndWriteToFile(stackStatus, file)

		// remove old status file if existed
		if isStatusFileExist == true {
			logger.Debugf("removing %s-temp file", stackCreateStatusFile)
			_ = os.Remove(stackCreateStatusFile + "-temp")
		}
	}
	defer file.Close()

	// remove stack delete status file if exists
	if _, err := os.Stat(stackDeleteStatusFile); err == nil {
		logger.Debugf("%s file already exist", stackDeleteStatusFile)
		logger.Debugf("removing %s file", stackDeleteStatusFile)
		_ = os.Remove(stackDeleteStatusFile)
	}

	// Retrieve Flavors from Storage and create in OpenStack
	flavor := models.Flavor{Vcpus: -1, RAM: -1, Disk: -1}
	query := models.Query{Entity: flavor}
	flavors, err := repo.Get(&query)
	if err != nil {
		logger.Errorf("Error in retrieving Flavors from Storage: %v", err)
		errMsg := "Failed to retrieve Flavors from Storage"
		stackStatus.Status = &Res{
			Id:  models.INT_SERVER_DB_ERR,
			Msg: models.StatusMsg(errMsg),
		}
		encodeStackStatusAndWriteToFile(stackStatus, file)
		return
	}

	// Retrieve Flavors from OpenStack
	flavorsList, err := orchestrator.GetFlavorList(tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in retrieving list of Flavors from OpenStack. %v", err)
		stackStatus.Status = &Res{
			Id:  models.EXTERNAL_ORCH_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeStackStatusAndWriteToFile(stackStatus, file)
		return
	}
	logger.Debugf("List of Flavors from OpenStack before adding from Storage: %v", flavorsList)

	// Openrc which is going to be used for flavor creation
	var openrcToCreateFlavor string

	// Iterate through Flavors
	flavorsArr := flavors.([]models.Flavor)
	for i := 0; i < len(flavorsArr); i++ {
		flavorNameExists := false

		name := flavorsArr[i].Name
		vcpus := flavorsArr[i].Vcpus
		ram := flavorsArr[i].RAM
		disk := flavorsArr[i].Disk

		for j := 0; j < len(flavorsList); j++ {
			if flavorsList[j]["Name"].(string) == name &&
				flavorsList[j]["VCPUs"].(float64) == float64(vcpus) &&
				flavorsList[j]["RAM"].(float64) == float64(ram) &&
				flavorsList[j]["Disk"].(float64) == float64(disk) {
				flavorNameExists = true
				break
			} else {
				continue
			}
		}

		// If flavor does not exist in OpenStack, create it
		if !flavorNameExists {
			logger.Debugf("Creating Flavor: %s in OpenStack", name)

			openrcToCreateFlavor, err = createFlavorInOpenStack(name, vcpus, ram, disk, openrcToCreateFlavor)
			if err != nil {
				logger.Debugf("Error returned by createFlavorInOpenStack(). %v", err)
				if err.Error() == "Failed to locate "+adminOpenRC {
					stackStatus.Status = &Res{
						Id:  STACK_SERVICE_ADMIN_RC_NOT_UPLOADED_ERR,
						Msg: models.StatusMsg(err.Error()),
					}
					encodeStackStatusAndWriteToFile(stackStatus, file)
					return
				} else {
					stackStatus.Status = &Res{
						Id:  models.EXTERNAL_ORCH_ERR,
						Msg: models.StatusMsg(err.Error()),
					}
					encodeStackStatusAndWriteToFile(stackStatus, file)
					return
				}
			}
		}
	}

	// Call orchestrator LaunchHeatStack()
	_, err = orchestrator.LaunchHeatStack(name, heatTemplate, tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in launching HEAT Stack. %v", err)
		stackStatus.Status = &Res{
			Id:  models.EXTERNAL_ORCH_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeStackStatusAndWriteToFile(stackStatus, file)
		return
	}

	statusMsg := "CREATE_IN_PROGRESS"
	logger.Debugf("stack status before returning from initiateStackCreation() : %s", statusMsg)
	stackStatus.Status = &Res{
		Id:  models.NO_ERR,
		Msg: models.StatusMsg(statusMsg),
	}
	encodeStackStatusAndWriteToFile(stackStatus, file)
}

func initiateStackDeletion(name string, repo models.Repository) {

	logger.Debugf("Inside initiateStackDeletion()")

	// deferring unlocking of deleteInProgress before returning
	defer func() {
		unlockDeleteInProgress()
	}()

	var (
		stackStatus       = &StackStatus{}
		isStatusFileExist = false
	)

	// rename stack delete status file if already exists
	if _, err := os.Stat(stackDeleteStatusFile); err == nil {
		logger.Debugf("%s file already exist", stackDeleteStatusFile)
		isStatusFileExist = true
		logger.Debugf("renaming %s file to %s-temp", stackDeleteStatusFile, stackDeleteStatusFile)
		_ = os.Rename(stackDeleteStatusFile, stackDeleteStatusFile+"-temp")
	}

	file, err := os.OpenFile(stackDeleteStatusFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("Error in opening %s file. %v", stackDeleteStatusFile, err)
		// restore old status file if existed
		if isStatusFileExist == true {
			logger.Debugf("restoring already existing %s file", stackDeleteStatusFile)
			_ = os.Rename(stackDeleteStatusFile+"-temp", stackDeleteStatusFile)
		}
		return

	} else {
		stackStatus.Name = name
		stackStatus.Status = &Res{}
		encodeStackStatusAndWriteToFile(stackStatus, file)

		// remove old status file if existed
		if isStatusFileExist == true {
			logger.Debugf("removing %s-temp file", stackDeleteStatusFile)
			_ = os.Remove(stackDeleteStatusFile + "-temp")
		}
	}
	defer file.Close()

	// remove stack create status file if exists
	if _, err := os.Stat(stackCreateStatusFile); err == nil {
		logger.Debugf("%s file already exist", stackCreateStatusFile)
		logger.Debugf("removing %s file", stackCreateStatusFile)
		_ = os.Remove(stackCreateStatusFile)
	}

	// Call orchestrator DeleteHeatStack()
	err = orchestrator.DeleteHeatStack(name, tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in deleting HEAT stack. %v", err)
		stackStatus.Status = &Res{
			Id:  models.EXTERNAL_ORCH_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeStackStatusAndWriteToFile(stackStatus, file)
		return
	}
	logger.Debugf("HEAT stack deleted successfully. Now delete flavors from OpenStack.")

	// Retrieve Flavors from Storage and delete from OpenStack
	flavor := models.Flavor{Vcpus: -1, RAM: -1, Disk: -1}
	query := models.Query{Entity: flavor}
	flavors, err := repo.Get(&query)
	if err != nil {
		logger.Errorf("Error in retrieving Flavors from Storage: %v", err)
		errMsg := "Failed to retrieve Flavors from Storage"
		stackStatus.Status = &Res{
			Id:  models.INT_SERVER_DB_ERR,
			Msg: models.StatusMsg(errMsg),
		}
		encodeStackStatusAndWriteToFile(stackStatus, file)
		return
	}

	// Retrieve Flavors exist in OpenStack
	flavorsList, err := orchestrator.GetFlavorList(tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in retrieving list of Flavors from OpenStack. %v", err)
		stackStatus.Status = &Res{
			Id:  models.EXTERNAL_ORCH_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeStackStatusAndWriteToFile(stackStatus, file)
		return
	}
	logger.Debugf("List of Flavors from OpenStack before deletion: %v", flavorsList)

	// Openrc which is going to be used for flavor deletion
	var openrcToDeleteFlavor string

	// Iterate through Flavors
	flavorsArr := flavors.([]models.Flavor)
	for i := 0; i < len(flavorsArr); i++ {
		flavorNameExists := false

		for j := 0; j < len(flavorsList); j++ {
			if flavorsList[j]["Name"].(string) == flavorsArr[i].Name {
				flavorNameExists = true
				break
			} else {
				continue
			}
		}

		// If flavor exists in OpenStack, delete it
		if flavorNameExists {
			logger.Debugf("Deleting Flavor: %s from OpenStack", flavorsArr[i].Name)

			openrcToDeleteFlavor, err = deleteFlavorFromOpenStack(flavorsArr[i].Name, openrcToDeleteFlavor)
			if err != nil {
				logger.Debugf("Error returned by deleteFlavorFromOpenStack(). %v", err)
				if err.Error() == "Failed to locate "+adminOpenRC {
					stackStatus.Status = &Res{
						Id:  STACK_SERVICE_ADMIN_RC_NOT_UPLOADED_ERR,
						Msg: models.StatusMsg(err.Error()),
					}
					encodeStackStatusAndWriteToFile(stackStatus, file)
					return
				} else {
					stackStatus.Status = &Res{
						Id:  models.EXTERNAL_ORCH_ERR,
						Msg: models.StatusMsg(err.Error()),
					}
					encodeStackStatusAndWriteToFile(stackStatus, file)
					return
				}
			}
		}
	}

	statusMsg := "DELETE_IN_PROGRESS"
	logger.Debugf("stack status before returning from initiateStackDeletion() : %s", statusMsg)
	stackStatus.Status = &Res{
		Id:  models.NO_ERR,
		Msg: models.StatusMsg(statusMsg),
	}
	encodeStackStatusAndWriteToFile(stackStatus, file)
}

func checkIfValidHeatTemplate(templatePath string) (bool, error) {

	logger.Debugf("Inside checkIfValidHeatTemplate()")

	var isValid bool

	// Open and read heat template file
	file, err := os.Open(templatePath)
	if err != nil {
		logger.Errorf("Error in opening HEAT template file. %v", err)
		return isValid, err
	}
	defer file.Close()

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		logger.Errorf("Error in reading HEAT template file. %v", err)
		return isValid, err
	}

	templateMap := make(map[string]interface{})
	yaml.Unmarshal(fileBytes, &templateMap)

	resourcesMap := templateMap["resources"].(map[interface{}]interface{})
	for key, val := range resourcesMap {
		resourceName := key.(string)
		resourceMap := val.(map[interface{}]interface{})
		for key1, val1 := range resourceMap {
			if key1.(string) == "properties" {
				propertiesMap := val1.(map[interface{}]interface{})
				for key2, val2 := range propertiesMap {
					propertyName := key2.(string)
					if val2 == nil {
						logger.Debugf("For resource %s, property %s is empty. HEAT template is invalid.", resourceName, propertyName)
						return isValid, nil
					}
				}
			}
		}
	}
	logger.Debugf("HEAT template is valid")
	isValid = true

	return isValid, nil
}

func createFlavorInOpenStack(name string, vcpus int, ram int, disk int, openrc string) (string, error) {

	logger.Debugf("Inside createFlavorInOpenStack()")

	if openrc != "" {
		err := orchestrator.CreateFlavor(name, vcpus, ram, disk, openrc)
		if err != nil {
			logger.Errorf("Error in creating flavor using %s. %v", openrc, err)
			return openrc, err
		}

	} else {
		// try creating flavor with tenant openrc
		err := orchestrator.CreateFlavor(name, vcpus, ram, disk, tenantOpenRC)
		if err != nil {
			logger.Errorf("Error in creating flavor using tenant openrc. %v", err)
			logger.Debugf("Check if admin openrc is present")
			if _, err := os.Stat(adminOpenRC); os.IsNotExist(err) {
				logger.Errorf("Error in getting stat for %s file. %v", adminOpenRC, err)
				errMsg := "Failed to locate " + adminOpenRC
				return openrc, errors.New(errMsg)
			} else {
				// try creating flavor with admin openrc
				err := orchestrator.CreateFlavor(name, vcpus, ram, disk, adminOpenRC)
				if err != nil {
					logger.Errorf("Error in creating flavor using admin openrc. %v", err)
					return openrc, err
				} else {
					logger.Debugf("Successfully created flavor %s in OpenStack", name)
					return adminOpenRC, nil
				}
			}
		} else {
			logger.Debugf("Successfully created flavor %s in OpenStack", name)
			return tenantOpenRC, nil
		}
	}

	return openrc, nil
}

func deleteFlavorFromOpenStack(name string, openrc string) (string, error) {

	logger.Debugf("Inside deleteFlavorFromOpenStack()")

	if openrc != "" {
		err := orchestrator.DeleteFlavor(name, openrc)
		if err != nil {
			logger.Errorf("Error in deleting flavor using %s. %v", name, err)
			return openrc, err
		}

	} else {
		// try deleting flavor with tenant openrc
		err := orchestrator.DeleteFlavor(name, tenantOpenRC)
		if err != nil {
			logger.Errorf("Error in deleting flavor using tenant openrc. %v", err)
			logger.Debugf("Check if admin openrc is present")
			if _, err := os.Stat(adminOpenRC); os.IsNotExist(err) {
				logger.Errorf("Error in getting stat for %s file. %v", adminOpenRC, err)
				errMsg := "Failed to locate " + adminOpenRC
				return openrc, errors.New(errMsg)
			} else {
				// try deleting flavor with admin openrc
				err := orchestrator.DeleteFlavor(name, adminOpenRC)
				if err != nil {
					logger.Errorf("Error in deleting flavor using admin openrc. %v", err)
					return openrc, err
				} else {
					logger.Debugf("Successfully deleted flavor %s from OpenStack", name)
					return adminOpenRC, nil
				}
			}
		} else {
			logger.Debugf("Successfully deleted flavor %s from OpenStack", name)
			return tenantOpenRC, nil
		}
	}

	return openrc, nil
}

// encode stack creation/deletion status and write into file
func encodeStackStatusAndWriteToFile(ss *StackStatus, file *os.File) {
	logger.Debug("Inside encodeStackStatusAndWriteToFile()")

	// encoding stack status to json
	resJson, err := json.MarshalIndent(ss, "", " ")
	if err != nil {
		logger.Errorf("Error in marshalling stack status to json. %v", err)
		return
	}

	// truncating file and writing encoded result in it
	file.Truncate(0)
	file.Seek(0, 0)
	_, err = file.WriteString(string(resJson))
	if err != nil {
		logger.Errorf("Error in writing stack status in %s file. %v", file, err)
		return
	}
}

// Unlock createInProgress variable
func unlockCreateInProgress() {
	logger.Debugf("Unlocking createInProgress")
	_ = atomic.SwapUint32(&createInProgress, unlock)
}

// Unlock deleteInProgress variable
func unlockDeleteInProgress() {
	logger.Debugf("Unlocking deleteInProgress")
	_ = atomic.SwapUint32(&deleteInProgress, unlock)
}

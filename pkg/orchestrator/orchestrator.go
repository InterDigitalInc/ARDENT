// Package orchestrator implements functions for performing various
// OpenStack operations, e.g. reading, setting, configuring and deploying
// desired infra resources on OpenStack.
package orchestrator

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"

	"github.com/sirupsen/logrus"
)

// to store logger instance
var logger *logrus.Logger

// Initialize initializes logger for orchestrator.
//
// Parameters:
//  glogger: Logger instance.
//
// Returns:
//  error: Error(if any), otherwise nil. otherwise nil.
func Intialize(glogger *logrus.Logger) error {
	logger = glogger
	return nil
}

// GetStackList gets list of stacks from OpenStack and returns it.
//
// Parameters:
//  openrc: OpenRC file to source.
//
// Returns:
//  []map[string]string: List of stacks.
//  error: Error(if any), otherwise nil. otherwise nil.
//
// GetStackList sources the provided OpenRC file, runs OpenStack command
// to list stack and returns the command output.
func GetStackList(openrc string) ([]map[string]string, error) {

	logger.Debug("Inside GetStackList()")

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to list stack
	srcOpenrcCmd := "source " + openrc
	stackListCmd := "openstack stack list -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+stackListCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack stack list command. %v. %s", err, string(out))
		errMsg := "Failed to list stacks. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty []map[string]string to unmarshal
	// output returned by OpenStack command
	stackList := []map[string]string{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &stackList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return stackList, nil
}

// LaunchHeatStack launches HEAT stack in OpenStack with provided name.
//
// Parameters:
//  name: Stack name to create.
//  templatePath: Path to HEAT template.
//  openrc: OpenRC file to source.
//
// Returns:
//  map[string]string: Stack launch status.
//  error: Error(if any), otherwise nil. otherwise nil.
//
// LaunchHeatStack sources the provided OpenRC file, runs OpenStack command
// to launch stack and returns the command output.
func LaunchHeatStack(name string, templatePath string, openrc string) (map[string]string, error) {

	logger.Debugf("Inside LaunchHeatStack(). Stack to launch : %s", name)

	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		errMsg := "HEAT template does not exist"
		logger.Errorf("%s. %v", errMsg, err)
		return nil, errors.New(errMsg)
	}
	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to launch stack
	srcOpenrcCmd := "source " + openrc
	stackLaunchCmd := "openstack stack create -t " + templatePath + " " + name + " -f json"
	logger.Debugf("stack launch cmd : %s", stackLaunchCmd)
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+stackLaunchCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack stack create command. %v. %s", err, string(out))
		errMsg := "Failed to create stack. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty map[string]string to unmarshal
	// output returned by OpenStack command
	status := map[string]string{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &status)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return status, nil
}

// DeleteHeatStack deletes requested HEAT stack from OpenStack.
//
// Parameters:
//  name: Stack name to delete.
//  openrc: OpenRC file to source.
//
// Returns:
//  error: Error(if any), otherwise nil.
//
// DeleteHeatStack sources the provided OpenRC file, runs OpenStack command
// to delete stack and returns the command output.
func DeleteHeatStack(name string, openrc string) error {

	logger.Debugf("Inside DeleteHeatStack(). Stack to delete : %s", name)

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return err
	}

	// Source openrc and then run OpenStack command to delete stack
	// OpenStack stack delete command does not output anything
	srcOpenrcCmd := "source " + openrc
	//stackDeleteCmd := "openstack stack delete -y --wait " + name
	stackDeleteCmd := "openstack stack delete " + name
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+stackDeleteCmd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		logger.Errorf("Error in running OpenStack stack delete command. %v", err)
		errMsg := "Failed to delete Stack. " + stderr.String()
		return errors.New(errMsg)
	}

	return nil
}

// GetStackStatus gets stack status from OpenStack and returns it.
//
// Parameters:
//  name: Stack name whose status is to be returned.
//  openrc: OpenRC file to source.
//
// Returns:
//  map[string]interface{}: Stack status.
//  error: Error(if any), otherwise nil.
//
// GetStackStatus sources the provided OpenRC file, runs OpenStack command
// to get stack status and returns the command output.
func GetStackStatus(name string, openrc string) (map[string]interface{}, error) {

	logger.Debugf("Inside GetStackStatus(). Stack to check status : %s", name)

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to get stack status
	srcOpenrcCmd := "source " + openrc
	stackStatusCmd := "openstack stack show " + name + " -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+stackStatusCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack stack show command. %v. %s", err, string(out))
		errMsg := "Failed to get stack status. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty map[string]interface{} to unmarshal
	// output returned by OpenStack command
	status := map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &status)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return status, nil
}

// GetFlavorList gets list of flavors from OpenStack and returns it.
//
// Parameters:
//  openrc: OpenRC file to source.
//
// Returns:
//  []map[string]interface{}: List of flavors.
//  error: Error(if any), otherwise nil.
//
// GetFlavorList sources the provided OpenRC file, runs OpenStack command
// to get flavor list and returns the command output.
func GetFlavorList(openrc string) ([]map[string]interface{}, error) {

	logger.Debug("Inside GetFlavorList()")

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to list flavors
	srcOpenrcCmd := "source " + openrc
	flavorListCmd := "openstack flavor list -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+flavorListCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack flavor list command. %v. %s", err, string(out))
		errMsg := "Failed to list flavors. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty []map[string]interface{} to unmarshal
	// output returned by OpenStack command
	flavorList := []map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &flavorList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return flavorList, nil
}

// CreateFlavor creates flavor in OpenStack.
//
// Parameters:
//  flavorName: Flavor name to create.
//  vcpus: Vcpus to configure.
//  ram: RAM to configure.
//  disk: Disk to configure.
//  openrc: OpenRC file to source.
//
// Returns:
//  error: Error(if any), otherwise nil.
//
// CreateFlavor sources the provided OpenRC file, runs OpenStack command
// to create flavor and returns the command output.
func CreateFlavor(flavorName string, vcpus int, ram int, disk int, openrc string) error {

	logger.Debugf("Inside CreateFlavor(). Flavor to create : "+
		"vcpus - %d, ram - %dMB, disk - %dGB", vcpus, ram, disk)

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return err
	}

	// Source openrc and then run OpenStack command to list and create flavor
	srcOpenrcCmd := "source " + openrc
	flavorListCmd := "openstack flavor list -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+flavorListCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack flavor list command. %v. %s", err, string(out))
		errMsg := "Failed to list flavors. " + string(out)
		return errors.New(errMsg)
	}

	// Create an empty []map[string]interface{} to unmarshal
	// output returned by OpenStack command
	flavorList := []map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &flavorList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return errors.New(errMsg)
	}
	logger.Debugf("flavorList : %v", flavorList)

	// Check if flavorName already exist in flavorList
	var isFlavorExist bool = false
	for i := 0; i < len(flavorList); i++ {
		if flavorList[i]["Name"] == flavorName {
			logger.Debugf("Flavor %s already exists", flavorName)
			isFlavorExist = true
			break
		}
	}

	if isFlavorExist == true {
		logger.Debugf("%s flavor already exist", flavorName)
		return nil
	}

	createFlavorCmd := "openstack flavor create " +
		" --vcpus " + strconv.Itoa(vcpus) +
		" --ram " + strconv.Itoa(ram) +
		" --disk " + strconv.Itoa(disk) +
		" " + flavorName

	logger.Debugf("create flavor cmd : %s", createFlavorCmd)
	cmd = exec.Command("bash", "-c", srcOpenrcCmd+"&&"+createFlavorCmd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logger.Errorf("Error in running OpenStack create flavor command. %v", err)
		errMsg := "Failed to create flavor " + flavorName + ". " + stderr.String()
		return errors.New(errMsg)
	}

	logger.Debugf("Successfully created flavor : %s", flavorName)

	return nil
}

// DeleteFlavor deletes flavor from OpenStack.
//
// Parameters:
//  flavorName: Flavor name to delete.
//  openrc: OpenRC file to source.
//
// Returns:
//  error: Error(if any), otherwise nil.
//
// DeleteFlavor sources the provided OpenRC file, runs OpenStack command
// to delete flavor and returns the command output.
func DeleteFlavor(flavorName string, openrc string) error {

	logger.Debugf("Inside DeleteFlavor(). Flavor to delete : %s", flavorName)

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return err
	}

	// Source openrc and then run OpenStack command to delete flavor
	srcOpenrcCmd := "source " + openrc
	deleteFlavorCmd := "openstack flavor delete " + flavorName
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+deleteFlavorCmd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		logger.Errorf("Error in running OpenStack delete flavor command. %v", err)
		errMsg := "Failed to delete flavor " + flavorName + ". " + stderr.String()
		return errors.New(errMsg)
	}

	logger.Debugf("Successfully deleted flavor : %s", flavorName)

	return nil
}

// GetSecurityGroupList gets list of security groups from OpenStack and returns it.
//
// Parameters:
//  openrc: OpenRC file to source.
//
// Returns:
//  []map[string]interface{}: List of security groups.
//  error: Error(if any), otherwise nil.
//
// GetSecurityGroupList sources the provided OpenRC file, runs OpenStack command
// to get list of security groups and returns the command output.
func GetSecurityGroupList(openrc string) ([]map[string]interface{}, error) {

	logger.Debug("Inside GetSecurityGroupList()")

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to list security groups
	srcOpenrcCmd := "source " + openrc
	securityGroupListCmd := "openstack security group list -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+securityGroupListCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack security group list command. %v. %s", err, string(out))
		errMsg := "Failed to list security groups. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty []map[string]interface{} to unmarshal
	// output returned by OpenStack command
	securityGroupList := []map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &securityGroupList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return securityGroupList, nil
}

// ShowSecurityGroup gets the requested security group's configuration from
// OpenStack and returns it.
//
// Parameters:
//  securityGroupName: Security group name to show.
//  openrc: OpenRC file to source.
//
// Returns:
//  map[string]interface{}: Security group's configuration.
//  error: Error(if any), otherwise nil.
//
// ShowSecurityGroup sources the provided OpenRC file, runs OpenStack command
// to show requested security group and returns the command output.
func ShowSecurityGroup(securityGroupName string, openrc string) (map[string]interface{}, error) {
	logger.Debugf("Inside ShowSecurityGroup(). Security Group to show : %s", securityGroupName)

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to show security group
	srcOpenrcCmd := "source " + openrc
	showSecurityGroupCmd := "openstack security group show " + securityGroupName + " -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+showSecurityGroupCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack security group show command. %v. %s", err, string(out))
		errMsg := "Failed to show security group " + securityGroupName + ". " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty map[string]interface{} to unmarshal
	// output returned by OpenStack command
	securityGroupShow := map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &securityGroupShow)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return securityGroupShow, nil
}

// GetTenantQuotas returns OpenStack quota set for tenant.
//
// Parameters:
//  openrc: OpenRC file to source.
//
// Returns:
//  map[string]interface{}: OpenStack quota for tenant.
//  error: Error(if any), otherwise nil.
//
// GetTenantQuotas sources the provided OpenRC file, runs OpenStack command
// to show quota and returns the command output.
func GetTenantQuotas(openrc string) (map[string]interface{}, error) {
	logger.Debug("Inside GetTenantQuotas()")

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to list quotas
	srcOpenrcCmd := "source " + openrc
	quotaCmd := "openstack quota show -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+quotaCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack quota show command. %v. %s", err, string(out))
		errMsg := "Failed to show quota. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create a map[string]interface{} to unmarshal
	// output returned by OpenStack command
	quotaList := map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &quotaList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return quotaList, nil
}

// GetNetworkList gets network list from OpenStack and returns it.
//
// Parameters:
//  openrc: OpenRC file to source.
//
// Returns:
//  []map[string]interface{}: List of networks.
//  error: Error(if any), otherwise nil.
//
// GetNetworkList sources the provided OpenRC file, runs OpenStack command
// to get list of networks and returns the command output.
func GetNetworkList(openrc string) ([]map[string]interface{}, error) {
	logger.Debug("Inside GetNetworkList()")

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to list networks
	srcOpenrcCmd := "source " + openrc
	networkListCmd := "openstack network list -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+networkListCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack network list command. %v. %s", err, string(out))
		errMsg := "Failed to list networks. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty []map[string]interface{} to unmarshal
	// output returned by OpenStack command
	networkList := []map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &networkList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return networkList, nil
}

// ShowNetwork gets the requested network's configuration from
// OpenStack and returns it.
//
// Parameters:
//  networkName: Network name to show.
//  openrc: OpenRC file to source.
//
// Returns:
//  map[string]interface{}: Network's configuration.
//  error: Error(if any), otherwise nil.
//
// ShowNetwork sources the provided OpenRC file, runs OpenStack command
// to show requested network and returns the command output.
func ShowNetwork(networkName string, openrc string) (map[string]interface{}, error) {
	logger.Debugf("Inside ShowNetwork(). Network to show : %s", networkName)

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to show network
	srcOpenrcCmd := "source " + openrc
	showNetworkCmd := "openstack network show " + networkName + " -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+showNetworkCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack network show command. %v. %s", err, string(out))
		errMsg := "Failed to show network " + networkName + ". " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty map[string]interface{} to unmarshal
	// output returned by OpenStack command
	networkShow := map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &networkShow)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return networkShow, nil
}

// GetSubnetList gets list of subnets from OpenStack and returns it.
//
// Parameters:
//  openrc: OpenRC file to source.
//
// Returns:
//  []map[string]interface{}: List of subnets.
//  error: Error(if any), otherwise nil.
//
// GetSubnetList sources the provided OpenRC file, runs OpenStack command
// to get list of subnets and returns the command output.
func GetSubnetList(openrc string) ([]map[string]interface{}, error) {
	logger.Debug("Inside GetSubnetList()")

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to list subnets
	srcOpenrcCmd := "source " + openrc
	subnetListCmd := "openstack subnet list -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+subnetListCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack subnet list command. %v. %s", err, string(out))
		errMsg := "Failed to list subnets. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty []map[string]interface{} to unmarshal
	// output returned by OpenStack command
	subnetList := []map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &subnetList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return subnetList, nil
}

// GetImageList gets list of uploaded images from OpenStack and returns it.
//
// Parameters:
//  openrc: OpenRC file to source.
//
// Returns:
//  []map[string]interface{}: List of uploaded images.
//  error: Error(if any), otherwise nil.
//
// GetImageList sources the provided OpenRC file, runs OpenStack command
// to get list of uploaded images and returns the command output.
func GetImageList(openrc string) ([]map[string]interface{}, error) {
	logger.Debug("Inside GetImageList()")

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to list images
	srcOpenrcCmd := "source " + openrc
	imageListCmd := "openstack image list -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+imageListCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack image list command. %v. %s", err, string(out))
		errMsg := "Failed to list images. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty []map[string]interface{} to unmarshal
	// output returned by OpenStack command
	imageList := []map[string]interface{}{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the interface
	err = json.Unmarshal(out, &imageList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return imageList, nil
}

// GetHotVersionList gets list of HOT versions available in OpenStack and returns it.
//
// Parameters:
//  openrc: OpenRC file to source.
//
// Returns:
//  []map[string]string: List of available HOT versions.
//  error: Error(if any), otherwise nil.
//
// GetHotVersionList sources the provided OpenRC file, runs OpenStack command
// to get list of available HOT versions and returns the command output.
func GetHotVersionList(openrc string) ([]map[string]string, error) {
	logger.Debug("Inside GetHotVersionList()")

	// Check if openrc file is present. If not present, return error
	if err := checkIfOpenrcPresent(openrc); err != nil {
		return nil, err
	}

	// Source openrc and then run OpenStack command to list
	// heat template versions
	srcOpenrcCmd := "source " + openrc
	versionListCmd := "openstack orchestration template version list -f json"
	cmd := exec.Command("bash", "-c", srcOpenrcCmd+"&&"+versionListCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error in running OpenStack orchestration template version list command. %v. %s", err, string(out))
		errMsg := "Failed to list orchestration template versions. " + string(out)
		return nil, errors.New(errMsg)
	}

	// Create an empty []map[string]string to unmarshal
	// output returned by OpenStack command
	versionList := []map[string]string{}

	logger.Debug("Going to unmarshal OpenStack command output")

	// Unmarshal or Decode the JSON to the map
	err = json.Unmarshal(out, &versionList)
	if err != nil {
		logger.Errorf("Error in unmarshalling OpenStack command output. %v", err)
		errMsg := "Failed to unmarshal OpenStack command output"
		return nil, errors.New(errMsg)
	}

	return versionList, nil
}

func checkIfOpenrcPresent(openrc string) error {
	if _, err := os.Stat(openrc); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", openrc, err)
		errMsg := openrc + " file is not uploaded"
		return errors.New(errMsg)
	}
	return nil
}

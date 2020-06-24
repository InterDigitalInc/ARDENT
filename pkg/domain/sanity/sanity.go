package sanity

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/util"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/orchestrator"
)

type context struct {
	isOnlySingleDCAvail bool
}

type quota struct {
	cores     float64
	ram       float64
	instances float64
	ports     float64
	subnets   float64
}

// Struct for security group rule containing port and
// protocol
type rule struct {
	port     int
	protocol string
}

// A Warn is a struct for Sanity-Check warning to contain
// its category and description.
type Warn struct {
	Category    string `json:"category"`
	Description string `json:"description"`
}

// A Res is a struct for Sanity-Check status to contain Sanity-Check
// result's status Id and Msg.
type Res struct {
	Id  models.StatusId  `json:"status_id"`
	Msg models.StatusMsg `json:"status_str"`
}

// A SanityResult is struct for Sanity-Check result to contain
// status and warning(s) generated while checking sanity
// of the platform.
type SanityResult struct {
	Result  *Res
	Warning []*Warn
}

const (
	sanityResultFile    = "sanity-result"
	heatTemplateVersion = "2017-02-24"
	tenantOpenRC        = "tenant-openrc"
	adminOpenRC         = "admin-openrc"
)

// Map of resources required by each node depending on what it is
// going to host
var typeBasedResReqMap = map[string]*quota{
	"ctrl_func": &quota{
		cores:     0,  // req cores will be fetched from db and updated in map
		ram:       0,  // req ram will be fetched from db and updated in map
		instances: 9,  // 4(PCE_NM_SR_PS)+3(SR_CLMC_SFEMC)+1(FE)+1(MOOSE)
		ports:     33, // 16(PCE_NM_SR_PS)+11(SR_CLMC_SFEMC)+2(FE)+4(MOOSE)
		subnets:   3,  // 2(PCE_NM_SR_PS)+1(SR_CLMC_SFEMC)+0(FE)+0(MOOSE)
	},
	"sr_cluster": &quota{
		cores:     0, // req cores will be fetched from db and updated in map
		ram:       0, // req ram will be fetched from db and updated in map
		instances: 2, // 2(SR_CLUSTER)
		ports:     6, // 6(SR_CLUSTER)
		subnets:   1, // 1(SR_CLUSTER)
	},
	"sr_poa": &quota{
		cores:     0, // req cores will be fetched from db and updated in map
		ram:       0, // req ram will be fetched from db and updated in map
		instances: 1, // 1(SR_POA)
		ports:     4, // 4(SR_POA)
		subnets:   1, // 1(SR_POA)
	},
}

var platformFixedResReqMap = map[string]float64{
	"subnets":          4,  //SIA, WAN, MGMT and SDNCTRL
	"securityGrps":     5,  //MGMT, MSP, SDNCTRL, SIA, WAN - get the count from DB
	"securityGrpRules": 17, //1(CLMC)+4(MGMT)+3(MSP)+1(PS)+5(SDNCTRL)+1(SIA)+2(WAN) - get the count from DB
}

// Map of networks required by each node depending on it's tier type
var tierBasedNetworksMap = map[string][]string{
	"data_centre": []string{
		"data",
		"wan",
		"sdnctrl",
		"mgmt",
		"msp",
		"ps",
		"clmc-sfemc",
		"sia",
		"cluster",
	},
	"data_centre_ctrl_func": []string{
		"data",
		"wan",
		"sdnctrl",
		"mgmt",
		"msp",
		"ps",
		"clmc-sfemc",
		"sia",
	},
	"data_centre_sr_clust": []string{
		"data",
		"sdnctrl",
		"mgmt",
		"cluster",
	},
	"edge": []string{
		"data",
		"sdnctrl",
		"mgmt",
		"cluster",
	},
	"far_edge": []string{
		"data",
		"sdnctrl",
		"mgmt",
		"cluster",
		"access",
	},
	"mist": []string{
		"data",
		"mgmt",
		"sdnctrl",
		"access",
	},
}

func initiateSanityCheck(infraCtxt *util.InfraContext, repo models.Repository) {
	logger.Debug("Inside initiateSanityCheck()")

	// deferring unlocking of initiateInProgress before returning
	defer func() {
		unlockInitiateInProgress()
	}()

	// init sanityResult
	r := initSanityResult()
	r.Warning = []*Warn{}

	// remove existing sanity result file
	// Check if sanity result file already exists
	if _, err := os.Stat(sanityResultFile); err == nil {
		_ = os.Remove(sanityResultFile)
	}

	file, err := os.OpenFile(sanityResultFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("Error in opening %s file. %v", sanityResultFile, err)
		return
	}
	defer file.Close()

	// Check if tenant openrc exists
	if _, err := os.Stat(tenantOpenRC); os.IsNotExist(err) {
		logger.Errorf("Error in getting stat for %s file. %v", tenantOpenRC, err)
		errMsg := "Failed to locate " + tenantOpenRC
		r.Result = &Res{
			Id:  SANITY_SERVICE_TENANT_RC_NOT_UPLOADED_ERR,
			Msg: models.StatusMsg(errMsg),
		}
		encodeSanityResAndWriteToFile(r, file)
		return
	}

	// init context
	localCtxt := initContext()

	err = prepareForSanityCheck(infraCtxt, localCtxt)
	if err != nil {
		logger.Errorf("Error returned by prepareForHeatTempGeneration(). %v", err)
		r.Result = &Res{
			Id:  SANITY_SERVICE_SANITY_CHECK_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeSanityResAndWriteToFile(r, file)
		return
	}

	// To check if sufficient resources are available on each compute node depending
	// on it's tier type
	res, err := checkSanityForCompNodesResReq(infraCtxt)
	if err != nil {
		logger.Errorf("Error returned by checkSanityForCompNodesResReq(). %v", err)
		r.Result = &Res{
			Id:  SANITY_SERVICE_SANITY_CHECK_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeSanityResAndWriteToFile(r, file)
		return
	}
	r.Warning = append(r.Warning, res...)

	// To check if OpenStack has sufficient resources to deploy the platform
	res, err = checkSanityForQuotas(infraCtxt, localCtxt)
	if err != nil {
		logger.Errorf("Error returned by checkSanityForQuotas(). %v", err)
		r.Result = &Res{
			Id:  SANITY_SERVICE_SANITY_CHECK_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeSanityResAndWriteToFile(r, file)
		return
	}
	r.Warning = append(r.Warning, res...)

	// To check if OpenStack has required networks
	// And each compute node is attached to the mandatory networks depending
	// on it's type
	res, err = checkSanityForNetworks(infraCtxt, localCtxt)
	if err != nil {
		logger.Errorf("Error returned by checkSanityForNetworks(). %v", err)
		r.Result = &Res{
			Id:  SANITY_SERVICE_SANITY_CHECK_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeSanityResAndWriteToFile(r, file)
		return
	}
	r.Warning = append(r.Warning, res...)

	// To check if OpenStack has required security groups
	// And required security group rules are applied on each security group
	res, err = checkSanityForSecurityGroupsAndRules(infraCtxt, repo)
	if err != nil {
		logger.Errorf("Error returned by checkSanityForSecurityGroupsAndRules(). %v", err)
		r.Result = &Res{
			Id:  SANITY_SERVICE_SANITY_CHECK_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeSanityResAndWriteToFile(r, file)
		return
	}
	r.Warning = append(r.Warning, res...)

	// To check if OpenStack has required images
	res, err = checkSanityForImages()
	if err != nil {
		logger.Errorf("Error returned by checkSanityForImages(). %v", err)
		r.Result = &Res{
			Id:  SANITY_SERVICE_SANITY_CHECK_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeSanityResAndWriteToFile(r, file)
		return
	}
	r.Warning = append(r.Warning, res...)

	// To confirm that OpenStack flavor creation/deletion is
	// happening without any error
	res1 := checkSanityForFlavorCreationDeletion()
	if res1 != nil {
		r.Warning = append(r.Warning, res1)
	}

	// To check whether used heat template version is available or not
	res2, err := checkAvailabilityOfHotVersionUsed()
	if err != nil {
		logger.Errorf("Error returned by checkAvailabilityOfHotVersionUsed(). %v", err)
		r.Result = &Res{
			Id:  SANITY_SERVICE_SANITY_CHECK_ERR,
			Msg: models.StatusMsg(err.Error()),
		}
		encodeSanityResAndWriteToFile(r, file)
		return
	}
	if res2 != nil {
		r.Warning = append(r.Warning, res2)
	}

	r.Result = &Res{
		Id:  models.NO_ERR,
		Msg: models.StatusMsg("successful"),
	}
	encodeSanityResAndWriteToFile(r, file)
}

func prepareForSanityCheck(infraCtxt *util.InfraContext, localCtxt *context) error {

	// check if there are multiple DC nodes available
	isMultipleDCAvail, err := util.CheckIfMultipleDCsAvail(infraCtxt)
	if err != nil {
		logger.Errorf("Error returned by CheckIfMultipleDCsAvail(). %v", err)
		return err
	}
	localCtxt.isOnlySingleDCAvail = !(isMultipleDCAvail)

	return nil
}

func checkSanityForCompNodesResReq(infraCtxt *util.InfraContext) ([]*Warn, error) {
	logger.Debug("Inside checkSanityForCompNodesResReq()")

	var result = []*Warn{}

	// fetch index of compute node hosting control functions
	idx, err := util.FetchCompNodeForCtrlFunctions(infraCtxt)
	if err != nil {
		if idx == -1 {
			logger.Errorf("Error returned by fetchCompNodeForCtrlFunctions(). %v", err)
			return nil, err
		}
		if idx == 0 {
			res := &Warn{
				Category:    "compute-node",
				Description: err.Error(),
			}
			result = append(result, res)
		}
	}

	err = util.CheckResAvailabilityOnAllCompNodes(infraCtxt)
	if err != nil {
		logger.Errorf("Error returned by checkResAvailabilityOnAllCompNodes(). %v", err)
		res := &Warn{
			Category:    "compute-node",
			Description: err.Error(),
		}
		result = append(result, res)
	}

	return result, nil
}

func checkSanityForQuotas(infraCtxt *util.InfraContext, localCtxt *context) ([]*Warn, error) {
	logger.Debug("Inside checkSanityForQuotas()")

	// update typeBasedResReqMap for cores and ram
	updateTypeBasedResReqMap(infraCtxt)

	// get quotas from OpenStack
	quotas, err := orchestrator.GetTenantQuotas(tenantOpenRC)
	if err != nil {
		logger.Debugf("Error in showing OpenStack quotas. %v", err)
		return nil, err
	}
	logger.Debugf("OpenStack quotas : %v", quotas)

	// get platform resource requirements
	platformResReq := getPlatformResReq(infraCtxt.Store, localCtxt)
	logger.Debugf("Platform resource requirements : %v", *platformResReq)

	// compare platform requirements with OpenStack quota
	res := comparePlatformReqWithOSQuotas(quotas, platformResReq)

	return res, nil
}

func checkSanityForNetworks(infraCtxt *util.InfraContext, localCtxt *context) ([]*Warn, error) {
	logger.Debug("Inside checkSanityForNetworks()")

	var res = []*Warn{}

	// check if required networks are attached to each compute node depending on it's type
	res1 := checkIfCompReqNwsAttached(infraCtxt.Store.Computes, localCtxt)
	res = append(res, res1...)

	// get networks from OpenStack
	networkList, err := orchestrator.GetNetworkList(tenantOpenRC)
	if err != nil {
		logger.Debugf("Error in getting OpenStack network list. %v", err)
		return nil, err
	}
	logger.Debugf("OpenStack network list : %v", networkList)

	// check if required networks are created in OpenStack
	res2, existingNetworks, networkCat, networkSubnet := checkIfReqNwsExistInOS(infraCtxt.Store.Networks, networkList)
	res = append(res, res2...)

	// check if subnets created while infrastructure-setup (wan, sdnctrl, sia and mgmt)
	// exist in OpenStack
	res3, err := checkSubnetsForExistingNws(existingNetworks, networkCat, networkSubnet)
	if err != nil {
		logger.Debugf("Error in checking subnet for networks in OpenStack. %v", err)
		return nil, err
	}
	res = append(res, res3...)

	// check port_security_enabled status for existing networks depending on it's category
	res4, err := checkPortSecurityForExistingNws(existingNetworks, networkCat)
	if err != nil {
		logger.Debugf("Error in checking port security for networks in OpenStack. %v", err)
		return nil, err
	}
	res = append(res, res4...)

	return res, nil
}

func checkSanityForSecurityGroupsAndRules(infraCtxt *util.InfraContext, repo models.Repository) ([]*Warn, error) {
	logger.Debug("Inside checkSanityForSecurityGroupsAndRules()")

	// get security groups from OpenStack
	securityGroupList, err := orchestrator.GetSecurityGroupList(tenantOpenRC)
	if err != nil {
		logger.Debugf("Error in getting OpenStack security group list. %v", err)
		return nil, err
	}
	logger.Debugf("OpenStack security group List : %v", securityGroupList)

	var res = []*Warn{}
	// check if required security groups exist in OpenStack
	res1, existingSecurityGrps, securityGrpsCat := checkIfReqSecurityGrpsExistInOS(infraCtxt.Store.SecurityGroups, securityGroupList)
	res = append(res, res1...)

	// check if all the required rules are applied to existing security groups
	res2, err := checkRulesOnExistingSecurityGrps(existingSecurityGrps, securityGrpsCat, repo)
	if err != nil {
		logger.Debugf("Error in checking rules applied on security groups in OpenStack. %v", err)
		return nil, err
	}
	res = append(res, res2...)

	return res, nil
}

func checkSanityForImages() ([]*Warn, error) {
	logger.Debug("Inside checkSanityForImages()")

	// get OpenStack images
	osImageList, err := orchestrator.GetImageList(tenantOpenRC)
	if err != nil {
		logger.Debugf("Error in getting OpenStack image list. %v", err)
		return nil, err
	}
	logger.Debugf("OpenStack image List : %v", osImageList)

	heatImageList, err := getReqHeatImageList()
	if err != nil {
		logger.Debugf("Error in getting required image list for HEAT. %v", err)
		return nil, err
	}
	logger.Debugf("HEAT image List : %v", heatImageList)

	// check if images required for HEAT exist in OpenStack
	res, err := checkIfReqHeatImagesExistInOS(osImageList, heatImageList)
	if err != nil {
		logger.Debugf("Error in checking required images existance in OpenStack. %v", err)
		return nil, err
	}

	return res, nil
}

func checkSanityForFlavorCreationDeletion() *Warn {
	logger.Debug("Inside checkSanityForFlavorCreationDeletion()")

	// test flavor configuration
	var (
		testFlavor = "flame-test-flavor"
		vcpus      = 1
		ram        = 512
		disk       = 2
	)

	// creating test flavor
	// try creating flavor with tenant openrc
	err := orchestrator.CreateFlavor(testFlavor, vcpus, ram, disk, tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in creating dummy flavor '%s' in OpenStack using %s", testFlavor, tenantOpenRC)
		logger.Debugf("Check if admin openrc is present")
		if _, err1 := os.Stat(adminOpenRC); os.IsNotExist(err1) {
			logger.Errorf("Error in getting stat for %s file. %v", adminOpenRC, err1)
			res := &Warn{
				Category: "flavors",
				Description: "flavor creation failed using " + tenantOpenRC + ". " + err.Error() +
					". Upload " + adminOpenRC + " and try again.",
			}
			return res

		} else {
			// try creating flavor with admin openrc
			err := orchestrator.CreateFlavor(testFlavor, vcpus, ram, disk, adminOpenRC)
			if err != nil {
				logger.Errorf("Error in creating dummy flavor '%s' in OpenStack using %s", testFlavor, adminOpenRC)
				res := &Warn{
					Category:    "flavors",
					Description: "flavor creation failed using " + adminOpenRC + ". " + err.Error(),
				}
				return res
			}
		}
	}

	// deleting test flavor
	// try deleting flavor with tenant openrc
	err = orchestrator.DeleteFlavor(testFlavor, tenantOpenRC)
	if err != nil {
		logger.Errorf("Error in deleting dummy flavor '%s' from OpenStack using %s", testFlavor, tenantOpenRC)
		logger.Debugf("Check if admin openrc is present")
		if _, err1 := os.Stat(adminOpenRC); os.IsNotExist(err1) {
			logger.Errorf("Error in getting stat for %s file. %v", adminOpenRC, err1)
			res := &Warn{
				Category: "flavors",
				Description: "flavor deletion failed using " + tenantOpenRC + ". " + err.Error() +
					". Upload " + adminOpenRC + " and try again.",
			}
			return res

		} else {
			// try deleting flavor with admin openrc
			err := orchestrator.DeleteFlavor(testFlavor, adminOpenRC)
			if err != nil {
				logger.Errorf("Error in deleting dummy flavor '%s' from OpenStack using %s", testFlavor, adminOpenRC)
				res := &Warn{
					Category:    "flavors",
					Description: "flavor deletion failed using" + adminOpenRC + ". " + err.Error(),
				}
				return res
			}
		}
	}

	return nil
}

func checkAvailabilityOfHotVersionUsed() (*Warn, error) {
	logger.Debug("Inside checkAvailabilityOfHotVersionUsed()")

	var (
		result  *Warn
		isAvail = false
	)

	// get heat template version list from OpenStack
	versionList, err := orchestrator.GetHotVersionList(tenantOpenRC)
	if err != nil {
		logger.Debugf("Error in getting OpenStack template version list. %v", err)
		return nil, err
	}
	logger.Debugf("OpenStack template version list : %v", versionList)

	version := "heat_template_version." + heatTemplateVersion
	for i := 0; i < len(versionList); i++ {
		if versionList[i]["Version"] == version {
			logger.Debugf("%s is available", version)
			isAvail = true
			break
		}
	}
	if isAvail == false {
		logger.Debugf("%s is not available", version)
		result = &Warn{
			Category:    "heat-template",
			Description: version + " is not available",
		}
	}

	return result, nil
}

func comparePlatformReqWithOSQuotas(quotas map[string]interface{}, platformResReq *quota) []*Warn {
	logger.Debug("Inside comparePlatformReqWithOSQuotas()")

	var result = []*Warn{}

	// compare platform requirements with OpenStack quotas

	// compare cores, ram, instances and ports with OpenStack quota
	logger.Debugf("OpenStack cores : %.0f", quotas["cores"].(float64))
	logger.Debugf("platform cores : %.0f", platformResReq.cores)
	if platformResReq.cores > quotas["cores"].(float64) {
		reqCores := fmt.Sprintf("%0.f", platformResReq.cores)
		logger.Debug("cores are insufficient")
		res := &Warn{
			Category: "quotas",
			Description: "cores are insufficient, required number of cores are: " +
				reqCores,
		}
		result = append(result, res)
	}
	logger.Debugf("OpenStack ram : %.0f", quotas["ram"].(float64))
	logger.Debugf("platform ram : %.0f", platformResReq.ram)
	if platformResReq.ram > quotas["ram"].(float64) {
		reqRam := fmt.Sprintf("%0.f", platformResReq.ram)
		logger.Debug("ram is insufficient")
		res := &Warn{
			Category:    "quotas",
			Description: "ram is insufficient, required ram is: " + reqRam,
		}
		result = append(result, res)
	}
	logger.Debugf("OpenStack instances : %.0f", quotas["instances"].(float64))
	logger.Debugf("platform instances : %.0f", platformResReq.instances)
	if platformResReq.instances > quotas["instances"].(float64) {
		reqInstances := fmt.Sprintf("%0.f", platformResReq.instances)
		logger.Debug("instances are insufficient")
		res := &Warn{
			Category: "quotas",
			Description: "instances are insufficient, required number of instances are: " +
				reqInstances,
		}
		result = append(result, res)
	}
	logger.Debugf("OpenStack ports : %.0f", quotas["ports"].(float64))
	logger.Debugf("platform ports : %.0f", platformResReq.ports)
	if platformResReq.ports > quotas["ports"].(float64) {
		reqPorts := fmt.Sprintf("%0.f", platformResReq.ports)
		logger.Debug("ports are insufficient")
		res := &Warn{
			Category: "quotas",
			Description: "ports are insufficient, required number of ports are: " +
				reqPorts,
		}
		result = append(result, res)
	}

	reqSubnets := platformResReq.subnets + platformFixedResReqMap["subnets"]
	logger.Debugf("OpenStack subnets : %.0f", quotas["subnets"].(float64))
	logger.Debugf("platform subnets : %.0f", reqSubnets)
	if reqSubnets > quotas["subnets"].(float64) {
		reqSubs := fmt.Sprintf("%0.f", reqSubnets)
		logger.Debug("subnets are insufficient")
		res := &Warn{
			Category: "quotas",
			Description: "subnets are insufficient, required number of subnets are: " +
				reqSubs,
		}
		result = append(result, res)
	}

	logger.Debugf("OpenStack security groups : %.0f", quotas["secgroups"].(float64))
	logger.Debugf("platform security groups : %.0f", platformFixedResReqMap["securityGrps"])
	if platformFixedResReqMap["securityGrps"] > quotas["secgroups"].(float64) {
		reqSecGrps := fmt.Sprintf("%0.f", platformFixedResReqMap["securityGrps"])
		logger.Debug("security groups are insufficient")
		res := &Warn{
			Category: "quotas",
			Description: "security groups are insufficient, required number of security groups are: " +
				reqSecGrps,
		}
		result = append(result, res)
	}
	logger.Debugf("OpenStack security group rules : %.0f", quotas["secgroup-rules"])
	logger.Debugf("platform security group rules : %.0f", platformFixedResReqMap["securityGrpRules"])
	if platformFixedResReqMap["securityGrpRules"] > quotas["secgroup-rules"].(float64) {
		reqSecGrpRules := fmt.Sprintf("%0.f", platformFixedResReqMap["securityGrpRules"])
		logger.Debug("security group rules are insufficient")
		res := &Warn{
			Category: "quotas",
			Description: "security group rules are insufficient, required number of security group rules are: " +
				reqSecGrpRules,
		}
		result = append(result, res)
	}

	return result
}

func checkIfReqNwsExistInOS(infraNws []models.Network, osNws []map[string]interface{}) ([]*Warn, []string, []string, [][]string) {
	logger.Debug("Inside checkIfReqNwsExistInOS()")

	var (
		result = []*Warn{}

		// existingNetworks and existingNetworksCat will contain Identifier and Category
		// respectively of required networks existing in OpenStack
		existingNetworks    = []string{}
		existingNetworksCat = []string{}
		existingNetworksSub = [][]string{}
	)

	// Iterate networks required by infra and check if they exist in OpenStack
	// and validate identifier for each network
	for i := 0; i < len(infraNws); i++ {
		identifier := infraNws[i].Identifier
		category := infraNws[i].Category
		isExist := false
		// check if this network exist in OpenStack
		for j := 0; j < len(osNws); j++ {
			if osNws[j]["ID"] == identifier {
				logger.Debugf("Network %s exist in OpenStack", identifier)
				existingNetworks = append(existingNetworks, identifier)
				existingNetworksCat = append(existingNetworksCat, category)

				// arr will contain subnets for this network
				arr := make([]string, 0)

				subnets := osNws[j]["Subnets"]
				switch subnets.(type) {
				case string:
					// subnets are coming as comma separated strings,
					// split and add each subnet in arr and append arr
					// to existingNetworksSub
					logger.Debug("Subnets are coming as string")
					logger.Debugf("Subnets : %v", subnets)
					parts := strings.Split(subnets.(string), ", ")
					// iterate parts
					for n := 0; n < len(parts); n++ {
						arr = append(arr, parts[n])
					}
				case []interface{}:
					// subnets are coming as array of interface so iterate interface array,
					// type assert each value to string and append to arr
					// finally append arr to existingNetworksSub
					logger.Debug("Subnets are coming as interface array")
					logger.Debugf("Subnets : %v", subnets)
					ifcArr := subnets.([]interface{})
					// iterate interface array ifcArr
					for n := 0; n < len(ifcArr); n++ {
						arr = append(arr, ifcArr[n].(string))
					}
				}
				existingNetworksSub = append(existingNetworksSub, arr)

				isExist = true
				break
			}
		}
		if isExist == false {
			logger.Debugf("Network %s does not exist in OpenStack", identifier)
			res := &Warn{
				Category: "networks",
				Description: "network " + identifier + " of category " + category +
					" does not exist in OpenStack",
			}
			result = append(result, res)
		}
	}

	return result, existingNetworks, existingNetworksCat, existingNetworksSub
}

func checkSubnetsForExistingNws(existingNws []string, networkCat []string, networkSub [][]string) ([]*Warn, error) {
	logger.Debug("Inside checkSubnetsForExistingNws()")

	var result = []*Warn{}

	for i := 0; i < len(existingNws); i++ {
		network := existingNws[i]
		category := networkCat[i]
		if category == "wan" || category == "sdnctrl" ||
			category == "sia" || category == "mgmt" {

			logger.Debugf("Subnets for network %s of category %s : %v", network, category, networkSub)
			if len(networkSub[i]) == 0 {
				res := &Warn{
					Category:    "subnets",
					Description: "subnet for network " + network + " of category " + category + " is not configured",
				}
				result = append(result, res)
			}
		}
	}

	return result, nil
}

func checkPortSecurityForExistingNws(existingNws []string, networkCat []string) ([]*Warn, error) {
	logger.Debug("Inside checkPortSecurityForExistingNws()")

	var result = []*Warn{}

	for i := 0; i < len(existingNws); i++ {
		network := existingNws[i]
		category := networkCat[i]
		out, err := orchestrator.ShowNetwork(network, tenantOpenRC)
		if err != nil {
			logger.Debugf("Error in showing %s network in OpenStack. %v", network, err)
			return nil, err
		}
		portSecurityEnabled := out["port_security_enabled"].(bool)
		logger.Debugf("Network: %s (category: %s) has port security: %v", network,
			category, portSecurityEnabled)

		res := checkPortSecurityForNwCat(network, category, portSecurityEnabled)
		if res != nil {
			result = append(result, res)
		}
	}

	return result, nil
}

func checkPortSecurityForNwCat(network string, category string, portSecurity bool) *Warn {
	logger.Debug("Inside checkPortSecurityForNw")

	var result *Warn

	if category == "access" && portSecurity == true {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security enabled",
		}
	} else if category == "data" && portSecurity == true {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security enabled",
		}
	} else if category == "clmc-sfemc" && portSecurity == true {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security enabled",
		}
	} else if category == "cluster" && portSecurity == true {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security enabled",
		}
	} else if category == "mgmt" && portSecurity == false {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security disabled",
		}
	} else if category == "msp" && portSecurity == false {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security disabled",
		}
	} else if category == "ps" && portSecurity == true {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security enabled",
		}
	} else if category == "sdnctrl" && portSecurity == false {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security disabled",
		}
	} else if category == "sia" && portSecurity == false {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security disabled",
		}
	} else if category == "wan" && portSecurity == false {
		result = &Warn{
			Category:    "port-security",
			Description: "network " + network + " of category " + category + " has port security disabled",
		}
	}

	return result
}

func checkIfCompReqNwsAttached(computes []util.ComputeInfo, localCtxt *context) []*Warn {
	logger.Debug("Inside checkIfCompReqNwsAttached()")

	var result = []*Warn{}

	// check if all the networks required by each compute node
	// depending on its tier type are attached to the node or not
	for i := 0; i < len(computes); i++ {
		// networks attached to the node
		networksAttached := computes[i].NetworkCatNetworksMap
		logger.Debugf("Attached networks to compute node %s is : %v",
			computes[i].Compute.Name, networksAttached)

		// required networks depending on the tier type
		tierType := computes[i].Compute.Tier
		reqNetworks := []string{}
		if tierType == "data_centre" {
			if localCtxt.isOnlySingleDCAvail == false {
				if computes[i].IsCtrlHost == true {
					reqNetworks = tierBasedNetworksMap[tierType+"_ctrl_func"]
				} else {
					reqNetworks = tierBasedNetworksMap[tierType+"_sr_clust"]
				}
			} else {
				reqNetworks = tierBasedNetworksMap[tierType]
			}
		} else {
			reqNetworks = tierBasedNetworksMap[tierType]
		}

		logger.Debugf("Required networks for compute node of type  %s is : %v",
			tierType, reqNetworks)

		// Iterate reqNetworks to check if all the required networks are
		// attached to compute node
		for j := 0; j < len(reqNetworks); j++ {
			reqNw := reqNetworks[j]
			if _, ok := networksAttached[reqNw]; ok && len(networksAttached[reqNw]) != 0 {
				logger.Debugf("network of category %s is attached", reqNw)
			} else {
				logger.Debugf("network of category %s is missing", reqNw)
				res := &Warn{
					Category: "networks",
					Description: "network of category " + reqNw + " is not attached to " +
						computes[i].Compute.Tier + " type node " + computes[i].Compute.Name,
				}
				result = append(result, res)
			}
		}
	}

	return result
}

func checkIfReqSecurityGrpsExistInOS(infraSgs []models.SecurityGroup, osSgs []map[string]interface{}) ([]*Warn, []string, []string) {
	logger.Debug("Inside checkIfReqSecurityGrpsExistInOS()")

	var (
		result = []*Warn{}

		// existingSecurityGrps and securityGrpsCat will contain Identifier and Category
		// respectively of required security-groups existing in OpenStack
		existingSecurityGrps = []string{}
		securityGrpsCat      = []string{}
	)

	// Iterate security groups required by infra and check for each
	// security group's existance in OpenStack
	for i := 0; i < len(infraSgs); i++ {
		identifier := infraSgs[i].Identifier
		category := infraSgs[i].Category
		isExist := false
		// check if this security group exist in OpenStack
		for j := 0; j < len(osSgs); j++ {
			if osSgs[j]["ID"] == identifier {
				logger.Debugf("Security group %s exist in OpenStack", identifier)
				existingSecurityGrps = append(existingSecurityGrps, identifier)
				securityGrpsCat = append(securityGrpsCat, category)
				isExist = true
				break
			}
		}
		if isExist == false {
			logger.Debugf("Security group %s does not exist in OpenStack", identifier)
			res := &Warn{
				Category: "security-groups",
				Description: "security group " + identifier + " of category " + category +
					" does not exist in OpenStack",
			}
			result = append(result, res)
		}
	}

	return result, existingSecurityGrps, securityGrpsCat
}

func checkRulesOnExistingSecurityGrps(existingSgs []string, securityGrpsCat []string, repo models.Repository) ([]*Warn, error) {
	logger.Debug("Inside checkRulesOnExistingSecurityGrps()")

	var (
		result = []*Warn{}

		rulePort     int
		ruleProtocol string
	)

	for i := 0; i < len(existingSgs); i++ {
		securityGroup := existingSgs[i]
		category := securityGrpsCat[i]

		// get all the rules from DB for this security group
		// and check if all the desired rules are applied
		securityGrpRule := models.SecurityGrpRule{Name: category, Port: -1}
		q := models.Query{Entity: securityGrpRule}
		securityGrpRules, err := repo.Get(&q)
		if err != nil {
			logger.Errorf("Error returned by Get() Interface. %v", err)
			errMsg := "Failed to retrieve security group rules from storage"
			return nil, errors.New(errMsg)
		}
		reqRules := securityGrpRules.([]models.SecurityGrpRule)
		logger.Debugf("Required Rules : %v", reqRules)

		// get applied rules on this security group
		out, err := orchestrator.ShowSecurityGroup(securityGroup, tenantOpenRC)
		if err != nil {
			logger.Debugf("Error in showing %s security group in OpenStack. %v", securityGroup, err)
			return nil, err
		}

		// array of applied rules on this security group
		appliedRules := make([]rule, 0)

		rules := out["rules"]
		switch rules.(type) {
		case string:
			// rules are coming as comma separated strings,
			logger.Debug("Rules are coming as string")
			logger.Debugf("Rules : %v", rules)
			parts := strings.Split(rules.(string), "\n")
			for i := 0; i < len(parts); i++ {
				rulePort, ruleProtocol = getPortAndProtocolFromRuleStr(parts[i])
				r := rule{
					port:     rulePort,
					protocol: ruleProtocol,
				}
				appliedRules = append(appliedRules, r)
			}
		case []interface{}:
			logger.Debug("Rules are coming as interface array")
			logger.Debugf("Rules : %v", rules)
			ifcArr := rules.([]interface{})
			// iterate interface array ifcArr
			for n := 0; n < len(ifcArr); n++ {
				ruleMap := ifcArr[n].(map[string]interface{})
				rulePort, ruleProtocol = getPortAndProtocolFromRuleMap(ruleMap)
				r := rule{
					port:     rulePort,
					protocol: ruleProtocol,
				}
				appliedRules = append(appliedRules, r)
			}
		}
		logger.Debugf("Applied Rules : %v", appliedRules)

		// check if desired rules for security group which are fetched from DB
		// are applied to the security group existing in OpenStack
		res := checkIfReqSecurityGrpRulesApplied(securityGroup, reqRules, appliedRules)
		result = append(result, res...)
	}

	return result, nil
}

func checkIfReqSecurityGrpRulesApplied(securityGrp string, reqRules []models.SecurityGrpRule, appliedRules []rule) []*Warn {
	logger.Debug("Inside checkIfReqSecurityGrpRulesApplied()")

	var result = []*Warn{}

	for j := 0; j < len(reqRules); j++ {
		isExist := false
		for k := 0; k < len(appliedRules); k++ {
			if appliedRules[k].port == reqRules[j].Port &&
				appliedRules[k].protocol == reqRules[j].Protocol {
				logger.Debugf("%s rule (port : %d protocol : %s) is applied", reqRules[j].Name, reqRules[j].Port, reqRules[j].Protocol)
				isExist = true
				break
			}
		}
		if isExist == false {
			logger.Debugf("Security group rule %s is not applied on security group %s",
				reqRules[j].Name, securityGrp)
			var res *Warn
			if reqRules[j].Port == 0 {
				res = &Warn{
					Category: "security-group-rules",
					Description: "security group rule " + reqRules[j].Name + " (port: NULL, protocol: " +
						reqRules[j].Protocol + ") is not applied to security group " + securityGrp,
				}
			} else {
				res = &Warn{
					Category: "security-group-rules",
					Description: "security group rule " + reqRules[j].Name + " (port: " + strconv.Itoa(reqRules[j].Port) + ", protocol: " +
						reqRules[j].Protocol + ") is not applied to security group " + securityGrp,
				}
			}
			result = append(result, res)
		}
	}

	return result
}

func getReqHeatImageList() ([]string, error) {
	logger.Debug("Inside getReqHeatImageList()")

	var list []string

	// list yaml files inside heat directory
	files, err := ioutil.ReadDir(heatDirPath)
	if err != nil {
		logger.Errorf("Error in listing files in heat directory. %v", err)
		return list, err
	}

	for i := 0; i < len(files); i++ {
		fileName := files[i].Name()
		if strings.Contains(fileName, "yaml") {
			filePath := heatDirPath + "/" + fileName
			substr := "image: "
			lines, err := grepSubstrInFile(filePath, substr)
			if err != nil {
				logger.Errorf("Error in getting lines containing substring '%s' in file '%s'. %v", substr, fileName, err)
				return list, err
			}
			logger.Debugf("lines containing substring '%s' in file '%s' : %v", substr, fileName, lines)
			for j := 0; j < len(lines); j++ {
				imageName := strings.Split(lines[j], ":")[1]
				trimmedImageName := strings.TrimSpace(imageName)
				list = append(list, trimmedImageName)
			}
		}
	}

	imageList := removeDuplicatesFromStrSlice(list)

	return imageList, nil
}

func checkIfReqHeatImagesExistInOS(osImages []map[string]interface{}, heatImgs []string) ([]*Warn, error) {
	logger.Debug("Inside checkIfReqHeatImagesExistInOS()")

	var result = []*Warn{}

	// Iterate images required by HEAT and check if
	// that image exist in OpenStack
	for i := 0; i < len(heatImgs); i++ {
		isExist := false
		for j := 0; j < len(osImages); j++ {
			if osImages[j]["Name"] == heatImgs[i] {
				logger.Debugf("Image %s exist in OpenStack", heatImgs[i])
				isExist = true
				break
			}
		}
		if isExist == false {
			logger.Debugf("Image %s does not exist in OpenStack", heatImgs[i])
			res := &Warn{
				Category:    "images",
				Description: "image " + heatImgs[i] + " does not exist in OpenStack",
			}
			result = append(result, res)
		}
	}
	return result, nil
}

func getPlatformResReq(store *util.InfraStore, localCtxt *context) *quota {
	logger.Debug("Inside getPlatformResReq()")

	q := &quota{}

	// iterate computes and calculate platform requirements
	for i := 0; i < len(store.Computes); i++ {
		tierType := store.Computes[i].Compute.Tier
		nodeName := store.Computes[i].Compute.Name
		switch tierType {
		case "data_centre":
			logger.Debugf("node type is data centre")
			if localCtxt.isOnlySingleDCAvail == false {
				if store.Computes[i].IsCtrlHost == true {
					logger.Debugf("node type is data centre and this is control fn host")
					addCtrlFuncReqRes(q)
					logger.Debugf("quota now : %v", *q)
				} else {
					logger.Debugf("node type is data centre and this is sr cluster host")
					clusterRes := store.Computes[i].ClusterRes
					logger.Debugf("For node %s, cluster res are : %v", nodeName, clusterRes)
					addSrClusterReqRes(q, clusterRes)
					logger.Debugf("quota now : %v", *q)
				}
			} else {
				logger.Debugf("node type is data centre and this node is control fn and sr cluster host")
				addCtrlFuncReqRes(q)
				clusterRes := store.Computes[i].ClusterRes
				logger.Debugf("For node %s, cluster res are : %v", nodeName, clusterRes)
				addSrClusterReqRes(q, clusterRes)
				logger.Debugf("quota now : %v", *q)
			}
		case "edge":
			logger.Debugf("node type is edge and this is sr cluster host")
			clusterRes := store.Computes[i].ClusterRes
			logger.Debugf("For node %s, cluster res are : %v", nodeName, clusterRes)
			addSrClusterReqRes(q, clusterRes)
			logger.Debugf("quota now : %v", *q)
		case "far_edge":
			logger.Debugf("node type is far edge and this is sr poa and sr cluster host")
			noOfAccessNw := len(store.Computes[i].NetworkCatNetworksMap["access"])
			logger.Debugf("attached access networks : %d", noOfAccessNw)
			for j := 0; j < noOfAccessNw; j++ {
				addSrPoaReqRes(q)
				clusterRes := store.Computes[i].ClusterRes
				logger.Debugf("For node %s, cluster res are : %v", nodeName, clusterRes)
				addSrClusterReqRes(q, clusterRes)
				logger.Debugf("quota now : %v", *q)
			}
		case "mist":
			logger.Debugf("node type is mist and this is sr poa host")
			noOfAccessNw := len(store.Computes[i].NetworkCatNetworksMap["access"])
			logger.Debugf("attached access networks : %d", noOfAccessNw)
			for j := 0; j < noOfAccessNw; j++ {
				addSrPoaReqRes(q)
				logger.Debugf("quota now : %v", *q)
			}
		}
	}

	// add port for gateway and dhcp_agents to platform port requirements
	// add 1 for gateway
	q.ports = q.ports + 1
	// add number of dhcp_agents
	for n := 0; n < len(store.Metadata); n++ {
		if store.Metadata[n].ConfKey == "dhcp_agents" {
			dhcpAgents, _ := strconv.ParseFloat(store.Metadata[n].Value, 64)
			q.ports = q.ports + dhcpAgents
		}
	}

	return q
}

func addCtrlFuncReqRes(q *quota) {
	q.cores = q.cores + typeBasedResReqMap["ctrl_func"].cores
	q.ram = q.ram + typeBasedResReqMap["ctrl_func"].ram
	q.instances = q.instances + typeBasedResReqMap["ctrl_func"].instances
	q.ports = q.ports + typeBasedResReqMap["ctrl_func"].ports
	q.subnets = q.subnets + typeBasedResReqMap["ctrl_func"].subnets
}

func addSrClusterReqRes(q *quota, clusterRes util.Resources) {
	// typeBasedResReqMap["sr_cluster"] contains required cores and ram for SR only
	q.cores = q.cores + typeBasedResReqMap["sr_cluster"].cores
	q.ram = q.ram + typeBasedResReqMap["sr_cluster"].ram

	// adding provided cluster's required resources to quota
	q.cores = q.cores + float64(clusterRes.Vcpus)
	q.ram = q.ram + float64(clusterRes.RAM)

	q.instances = q.instances + typeBasedResReqMap["sr_cluster"].instances
	q.ports = q.ports + typeBasedResReqMap["sr_cluster"].ports
	q.subnets = q.subnets + typeBasedResReqMap["sr_cluster"].subnets
}

func addSrPoaReqRes(q *quota) {
	q.cores = q.cores + typeBasedResReqMap["sr_poa"].cores
	q.ram = q.ram + typeBasedResReqMap["sr_poa"].ram
	q.instances = q.instances + typeBasedResReqMap["sr_poa"].instances
	q.ports = q.ports + typeBasedResReqMap["sr_poa"].ports
	q.subnets = q.subnets + typeBasedResReqMap["sr_poa"].subnets
}

func getPortAndProtocolFromRuleStr(ruleStr string) (int, string) {
	logger.Debug("Inside getPortAndProtocolFromRuleStr()")

	var (
		port     int
		protocol string
	)

	fields := strings.Split(ruleStr, ",")
	for i := 0; i < len(fields); i++ {
		if strings.Contains(fields[i], "port_range_min") {
			logger.Debugf("string containing port : %s", fields[i])
			p := strings.Split(fields[i], "=")[1]
			pTrimmed := p[1 : len(p)-1]
			port, _ = strconv.Atoi(pTrimmed)
		}
		if strings.Contains(fields[i], "protocol") {
			logger.Debugf("string containing protocol : %s", fields[i])
			p := strings.Split(fields[i], "=")[1]
			protocol = p[1 : len(p)-1]
		}
	}
	logger.Debugf("port : %d protocol : %s", port, protocol)

	return port, protocol
}

func getPortAndProtocolFromRuleMap(ruleMap map[string]interface{}) (int, string) {
	logger.Debug("Inside getPortAndProtocolFromRuleMap()")

	var (
		port     int
		protocol string
	)

	portIfc := ruleMap["port_range_min"]
	switch portIfc.(type) {
	case float64:
		port = int(portIfc.(float64))
	case nil:
		port = 0
	}

	protocolIfc := ruleMap["protocol"]
	switch protocolIfc.(type) {
	case string:
		protocol = protocolIfc.(string)
	case nil:
		protocol = ""
	}

	logger.Debugf("port : %d protocol : %s", port, protocol)

	return port, protocol
}

func encodeSanityResAndWriteToFile(r *SanityResult, file *os.File) {
	logger.Debug("Inside encodeSanityResAndWriteToFile()")

	// encoding sanity result to json
	resJson, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		logger.Errorf("Error in marshalling sanity result to json. %v", err)
		return
	}

	// writting encoded result in file
	_, err = file.WriteString(string(resJson))
	if err != nil {
		logger.Errorf("Error in writing sanity result in file. %v", err)
		return
	}
}

func updateTypeBasedResReqMap(infraCtxt *util.InfraContext) {
	logger.Debug("Inside updateTypeBasedResReqMap()")

	for fn := range typeBasedResReqMap {
		res := util.GetReqResToHostFn(infraCtxt, fn)
		logger.Debugf("For fn %s, req res are : %v", fn, res)
		typeBasedResReqMap[fn].cores = float64(res.Vcpus)
		typeBasedResReqMap[fn].ram = float64(res.RAM)
	}
}

func grepSubstrInFile(filePath string, substr string) ([]string, error) {
	logger.Debug("Inside grepSubstrInFile()")

	var lines []string

	file, err := os.Open(filePath)
	if err != nil {
		logger.Errorf("Error in opening %s file. %v", filePath, err)
		return lines, err
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, substr) {
			trimmedLine := strings.TrimSpace(line)
			lines = append(lines, trimmedLine)
		}
	}
	file.Close()

	return lines, nil
}

func removeDuplicatesFromStrSlice(strSlice []string) []string {
	logger.Debugf("Inside removeDuplicatesFromStrSlice()")

	var (
		// map to record duplicates
		dupMap = map[string]bool{}
		result = []string{}
	)

	for i := 0; i < len(strSlice); i++ {
		if dupMap[strSlice[i]] == false {
			dupMap[strSlice[i]] = true
			result = append(result, strSlice[i])
		}
	}

	return result
}

func initContext() *context {
	return &context{}
}

func initSanityResult() *SanityResult {
	return &SanityResult{}
}

// Unlock initiateInProgress variable
func unlockInitiateInProgress() {
	logger.Debugf("Unlocking initiateInProgress")
	_ = atomic.SwapUint32(&initiateInProgress, unlock)
}

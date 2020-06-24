// Package util provides utility functions that are used by
// Ardent services.
package util

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

// A Resources is a type for stroing Compute node resources.
type Resources struct {
	Vcpus int
	RAM   int // Units: MB
	Disk  int // Units: GB
}

// A ComputeInfo is a type for storing Compute node specific properties.
type ComputeInfo struct {
	Compute *models.Compute

	// TODO: Map contains list of networks per category of network.
	NetworkCatNetworksMap map[string][]string

	// true if this is a DC node choosen to hold Ctrl functions.
	IsCtrlHost bool

	// cluster resources that will be used to create cluster flavor for this
	// compute.
	ClusterRes Resources
}

// An InfraStore defines structures for infra resources.
type InfraStore struct {
	Computes       []ComputeInfo
	Networks       []models.Network
	Subnets        []models.Subnet
	InfraServices  []models.InfraService
	SecurityGroups []models.SecurityGroup
	Metadata       []models.Config
	FlavorsMap     map[string]*models.Flavor
}

// An InfraContext provides logger, storage instance and infra store
// instance.
type InfraContext struct {
	logger *logrus.Logger
	repo   models.Repository

	Store *InfraStore
}

const (
	FlavorClmc     = "clmc"     // flavor name for 'clmc' type
	FlavorFrontend = "frontend" // flavor name for 'frontend' type
	FlavorMoose    = "moose"    // flavor name for 'moose' type
	FlavorPce      = "pce"      // flavor name for 'pce' type
	FlavorPs       = "ps"       // flavor name for 'ps' type
	FlavorSfemc    = "sfemc"    // flavor name for 'sfemc' type
	FlavorSr       = "sr"       // flavor name for 'sr' type
	FlavorNm       = "nm"       // flavor name for 'nm' type
)

// InitInfraContext initialises infra context.
//
// Parameters:
//  logger: Logger instance.
//  repo: Storage service handle.
//
// Returns:
//  *InfraContext: Infra context instance.
func InitInfraContext(logger *logrus.Logger, repo models.Repository) *InfraContext {
	ic := &InfraContext{}
	ic.logger = logger
	ic.repo = repo
	ic.Store = &InfraStore{}
	return ic
}

func isInfraCtxtInitialised(ic *InfraContext) bool {
	isInitialised := false
	if ic.logger != nil && ic.repo != nil && ic.Store != nil {
		isInitialised = true
	}
	return isInitialised
}

// PopulateInfraStore makes a call to 'util.getInfraFromStore' to verify
// initialisation of infra store.
//
// Parameters:
//  ic: Infra context instance.
// Returns:
//  models.StatusId: API response status code of type 'models.StatusId'.
//  error: Error(if any).
// PopulateInfraStore populates infra store with infra context if it's
// not initialised.
func PopulateInfraStore(ic *InfraContext) (models.StatusId, error) {
	ic.logger.Debugf("Inside PopulateInfraStore()")

	if isInfraCtxtInitialised(ic) == false {
		errMsg := "Infra Context not initialised properly"
		return models.INT_SERVER_ERR, errors.New(errMsg)
	}

	errCode, err := getInfraFromStore(ic)
	if err != nil {
		ic.logger.Errorf("Error in getting Infra from Store. %v", err)
		return errCode, err
	}

	return models.NO_ERR, nil
}

func getInfraFromStore(ic *InfraContext) (models.StatusId, error) {
	ic.logger.Debugf("Inside getInfraFromStore()")

	// retrieve computes from storage and fill in ComputeInfo{}
	compute := models.Compute{Vcpus: -1, RAM: -1, Disk: -1}
	q := models.Query{Entity: compute}
	computes, err := ic.repo.Get(&q)
	if err != nil {
		ic.logger.Errorf("Error returned by Get() Interface. %v", err)
		errMsg := "Failed to retrieve computes from storage"
		return models.INT_SERVER_DB_ERR, errors.New(errMsg)
	}
	if computes == nil {
		ic.logger.Debugf("Empty computes retrieved from storage")
		errMsg := "Incomplete infra descriptor found: compute-nodes are not present"
		return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, errors.New(errMsg)
	}
	comps := computes.([]models.Compute)
	for i := 0; i < len(comps); i++ {
		compInfo := ComputeInfo{}
		compInfo.Compute = &comps[i]
		ic.Store.Computes = append(ic.Store.Computes, compInfo)
	}

	// Compute []Networks is empty for all the computes
	// Get networks connected with each compute
	for i := 0; i < len(ic.Store.Computes); i++ {

		network := models.Network{}
		qCompNw := models.Query{Entity: network}
		compute := models.Compute{Name: ic.Store.Computes[i].Compute.Name,
			Vcpus: -1, RAM: -1, Disk: -1}
		qCompNw.And(compute)

		computeNetworks, err := ic.repo.Get(&qCompNw)
		if err != nil {
			ic.logger.Errorf("Error returned by Get() Interface. %v", err)
			errMsg := "Incomplete infra descriptor found: networks is empty"
			return models.INT_SERVER_DB_ERR, errors.New(errMsg)
		}
		if computeNetworks == nil {
			ic.logger.Debugf("Empty networks retrieved from storage for compute: '%s'",
				ic.Store.Computes[i].Compute.Name)
			errMsg := "Incomplete infra descriptor found: compute " + ic.Store.Computes[i].Compute.Name +
				" has no network connected to it"
			return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, errors.New(errMsg)
		}

		compNetworks := computeNetworks.([]models.Network)
		compNwIdMap := make(map[string][]string)
		for i := 0; i < len(compNetworks); i++ {
			compNwIdMap[compNetworks[i].Category] = append(compNwIdMap[compNetworks[i].Category], compNetworks[i].Identifier)
		}
		ic.Store.Computes[i].NetworkCatNetworksMap = compNwIdMap
		ic.logger.Debugf("Comp Name : '%s' networks: %v net id category %v", ic.Store.Computes[i].Compute.Name,
			ic.Store.Computes[i].Compute.Networks, ic.Store.Computes[i].NetworkCatNetworksMap)
	}
	ic.logger.Debugf("store.Computes: %v", ic.Store.Computes)

	// retrieve networks from storage
	network := models.Network{}
	q = models.Query{Entity: network}
	networks, err := ic.repo.Get(&q)
	if err != nil {
		ic.logger.Errorf("Error returned by Get() Interface. %v", err)
		errMsg := "Failed to retrieve networks from storage"
		return models.INT_SERVER_DB_ERR, errors.New(errMsg)
	}
	if networks == nil {
		ic.logger.Debugf("Empty networks retrieved from storage")
		errMsg := "Incomplete infra descriptor found: networks is empty"
		return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, errors.New(errMsg)
	}
	ic.Store.Networks = networks.([]models.Network)

	// retrieve subnets from storage
	subnet := models.Subnet{}
	q = models.Query{Entity: subnet}
	subnets, err := ic.repo.Get(&q)
	if err != nil {
		ic.logger.Errorf("Error returned by Get() Interface. %v", err)
		errMsg := "Failed to retrieve subnets from storage"
		return models.INT_SERVER_DB_ERR, errors.New(errMsg)
	}
	if subnets == nil {
		ic.logger.Debugf("Empty subnets retrieved from storage")
		errMsg := "Incomplete infra descriptor found: subnets is empty"
		return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, errors.New(errMsg)
	}
	ic.Store.Subnets = subnets.([]models.Subnet)

	// retrieve infra services from storage
	infraService := models.InfraService{}
	q = models.Query{Entity: infraService}
	infraServices, err := ic.repo.Get(&q)
	if err != nil {
		ic.logger.Errorf("Error returned by Get() Interface. %v", err)
		errMsg := "Failed to retrieve infra services from storage"
		return models.INT_SERVER_DB_ERR, errors.New(errMsg)
	}
	if infraServices == nil {
		ic.logger.Debugf("Empty infraServices retrieved from storage")
		errMsg := "Incomplete infra descriptor found: infrastructure-services is empty"
		return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, errors.New(errMsg)
	}
	ic.Store.InfraServices = infraServices.([]models.InfraService)

	// retrieve security groups from storage
	securityGroup := models.SecurityGroup{}
	q = models.Query{Entity: securityGroup}
	securityGroups, err := ic.repo.Get(&q)
	if err != nil {
		ic.logger.Errorf("Error returned by Get() Interface. %v", err)
		errMsg := "Failed to retrieve security groups from storage"
		return models.INT_SERVER_DB_ERR, errors.New(errMsg)
	}
	if securityGroups == nil {
		ic.logger.Debugf("Empty securityGroups retrieved from storage")
		errMsg := "Incomplete infra descriptor found: security-groups is empty"
		return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, errors.New(errMsg)
	}
	ic.Store.SecurityGroups = securityGroups.([]models.SecurityGroup)

	// retrieve metadata from storage
	config := models.Config{}
	q = models.Query{Entity: config}
	metadata, err := ic.repo.Get(&q)
	if err != nil {
		ic.logger.Errorf("Error returned by Get() Interface. %v", err)
		errMsg := "Failed to retrieve metadata from storage"
		return models.INT_SERVER_DB_ERR, errors.New(errMsg)
	}
	if metadata == nil {
		ic.logger.Debugf("Empty metadata retrieved from storage")
		errMsg := "Incomplete infra descriptor found: config is empty"
		return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, errors.New(errMsg)
	}
	ic.Store.Metadata = metadata.([]models.Config)

	// retrieve flavors from storage
	flavor := models.Flavor{Vcpus: -1, RAM: -1, Disk: -1}
	q = models.Query{Entity: flavor}
	flavors, err := ic.repo.Get(&q)
	if err != nil {
		ic.logger.Errorf("Error returned by Get() Interface. %v", err)
		errMsg := "Failed to retrieve flavors from storage"
		return models.INT_SERVER_DB_ERR, errors.New(errMsg)
	}
	if flavors == nil {
		ic.logger.Debugf("Empty flavors retrieved from storage")
		errMsg := "Incomplete infra descriptor found: flavors is empty"
		return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, errors.New(errMsg)
	}
	flavorsArr := flavors.([]models.Flavor)
	ic.Store.FlavorsMap = make(map[string]*models.Flavor)
	for i := 0; i < len(flavorsArr); i++ {
		ic.Store.FlavorsMap[flavorsArr[i].Name] = &flavorsArr[i]
	}
	err = isAllReqFlavorsExist(ic.Store.FlavorsMap)
	if err != nil {
		ic.logger.Debugf("isAllReqFlavorsExist() returned error. %v", err)
		return UTIL_DB_ENTITY_CONTAINS_NO_ENTRIES, err
	}

	return models.NO_ERR, nil
}

func CheckIfMultipleDCsAvail(ic *InfraContext) (bool, error) {
	ic.logger.Debug("Inside CheckIfMultipleDCsAvail()")

	compCount := 0
	isMultipleDCsAvailable := false

	if isInfraCtxtInitialised(ic) == false {
		errMsg := "Infra Context not initialised properly"
		return isMultipleDCsAvailable, errors.New(errMsg)
	}

	computes := ic.Store.Computes
	for i := 0; i < len(computes); i++ {
		if computes[i].Compute.Tier == "data_centre" {
			compCount = compCount + 1
			if compCount > 1 {
				ic.logger.Debugf("More than one DCs available")
				isMultipleDCsAvailable = true
				break
			}
		}
	}

	return isMultipleDCsAvailable, nil
}

func getCtrlFuncResReq(ic *InfraContext) (*Resources, error) {
	ic.logger.Debug("Inside getCtrlFuncResReq()")

	if isInfraCtxtInitialised(ic) == false {
		errMsg := "Infra Context not initialised properly"
		return nil, errors.New(errMsg)
	}

	reqRes := Resources{}

	flavorsMap := ic.Store.FlavorsMap

	psResReq, _ := flavorsMap[FlavorPs]
	addResources(&reqRes, getResourceFromFlavor(psResReq))

	pceResReq, _ := flavorsMap[FlavorPce]
	addResources(&reqRes, getResourceFromFlavor(pceResReq))

	clmcResReq, _ := flavorsMap[FlavorClmc]
	addResources(&reqRes, getResourceFromFlavor(clmcResReq))

	sfemcResReq, _ := flavorsMap[FlavorSfemc]
	addResources(&reqRes, getResourceFromFlavor(sfemcResReq))

	frontendResReq, _ := flavorsMap[FlavorFrontend]
	addResources(&reqRes, getResourceFromFlavor(frontendResReq))

	mooseResReq, _ := flavorsMap[FlavorMoose]
	addResources(&reqRes, getResourceFromFlavor(mooseResReq))

	nmResReq, _ := flavorsMap[FlavorNm]
	addResources(&reqRes, getResourceFromFlavor(nmResReq))

	srFlavor, _ := flavorsMap[FlavorSr]
	// Total SR resource requirements = 2 * (Res Reqd for Single SR)
	srResReq := getScaledFlavorResources(srFlavor, 2)
	addResources(&reqRes, srResReq)

	return &reqRes, nil
}

func FetchCompNodeForCtrlFunctions(ic *InfraContext) (int, error) {
	ic.logger.Debug("Inside FetchCompNodeForCtrlFunctions()")

	var compIdx int = -1

	if isInfraCtxtInitialised(ic) == false {
		errMsg := "Infra Context not initialised properly"
		return compIdx, errors.New(errMsg)
	}

	var cnq models.Query
	cnq.Entity = models.Compute{Vcpus: -1, RAM: -1, Disk: -1, Tier: "data_centre"}
	net := models.Network{Category: "wan"}
	cnq.And(net)

	computesWithWan, err := ic.repo.Get(&cnq)
	if err != nil {
		er := "Error in getting Computes with network 'wan' from Storage"
		ic.logger.Errorf("%s", er)

		err := errors.New(er)
		return compIdx, err
	}
	if computesWithWan == nil {
		er := "No Compute Node found associated with Network type 'wan'"
		ic.logger.Debugf("%s", er)

		// compIdx is set to 0 to consider this as a warning while performing sanity check
		compIdx = 0
		err := errors.New(er)
		return compIdx, err
	}
	ic.logger.Debugf("Compute Nodes connected with network type: 'wan' are: %v", computesWithWan)

	compIdx = chooseDCCompSatisfyingResReq(computesWithWan.([]models.Compute), ic)
	if compIdx == -1 {
		er := "No Compute Node found satisfying resource requirements for Ctrl functions"
		ic.logger.Errorf("%s", er)

		// compIdx is set to 0 to consider this as a warning while performing sanity check
		compIdx = 0
		err := errors.New(er)
		return compIdx, err
	}

	// Set it to be used as node for hosting Ctrl functions.
	ic.Store.Computes[compIdx].IsCtrlHost = true

	return compIdx, nil
}

func chooseDCCompSatisfyingResReq(computes []models.Compute, ic *InfraContext) int {
	tempIdx := -1

	resReq := Resources{}
	srResReq := Resources{}
	minClusterResReq := Resources{}

	ctrlFuncRes, _ := getCtrlFuncResReq(ic)
	addResources(&resReq, *ctrlFuncRes)
	ic.logger.Debugf("Res Req to host Ctrl functions: %v", resReq)

	isMultipleDCAvail, _ := CheckIfMultipleDCsAvail(ic)
	if isMultipleDCAvail == false {
		ic.logger.Debugf("There is only one DC available, so it will also host SR + Cluster")

		srFlavor, _ := ic.Store.FlavorsMap[FlavorSr]

		srResReq = getResourceFromFlavor(srFlavor)
		ic.logger.Debugf("SR resource requirement: %v", srResReq)

		// Add SR resource requirement in consumed resources.
		addResources(&resReq, srResReq)

		// Add Min. cluster resource requirement.
		minClusterResReq = getMinClusterResReq()
		ic.logger.Debugf("Min. cluster resource requirement: %v", minClusterResReq)

		addResources(&resReq, minClusterResReq)
	}

	ic.logger.Debugf("Try to find Compute Node to host Ctrl functions without reducing Clmc requirements")
	ic.logger.Debugf("Initial Resource Requirements: %v", resReq)
	tempIdx = findCompNodeSatisfyingResReq(computes, ic, &resReq)
	if tempIdx != -1 {
		return tempIdx
	}

	ic.logger.Debugf("Try to find Compute Node to host Ctrl functions after reducing Clmc requirements")
	clmcReq, _ := ic.Store.FlavorsMap[FlavorClmc]
	resReq.RAM = resReq.RAM - clmcReq.RAM/2
	ic.logger.Debugf("Reduced Resource Requirements: %v", resReq)
	tempIdx = findCompNodeSatisfyingResReq(computes, ic, &resReq)
	if tempIdx != -1 {
		return tempIdx
	}

	return -1
}

func findCompNodeSatisfyingResReq(computes []models.Compute, ic *InfraContext, resReq *Resources) int {
	tempIdx := -1
	consumedRes := Resources{}

	for i, _ := range computes {
		ic.logger.Debugf("Evaluating Compute Node: '%s' for resources", computes[i].Name)
		if areReqdResourcesAvailable(&computes[i], &consumedRes, resReq) {
			ic.logger.Debugf("Compute Node: '%s' have sufficient resources to host reqd functions",
				computes[i].Name)

			tempIdx = i
			break
		}
	}
	if tempIdx == -1 {
		ic.logger.Debugf("Reqd resources not available at any DC Compute Node")
		return tempIdx
	}
	// Find appropriate index in ic.Store
	for i, comp := range ic.Store.Computes {
		if comp.Compute.Name == computes[tempIdx].Name &&
			comp.Compute.AvailZone == computes[tempIdx].AvailZone {
			tempIdx = i
			break
		}
	}
	isMultipleDCAvail, _ := CheckIfMultipleDCsAvail(ic)
	if isMultipleDCAvail == false {
		// Compute cluster resources, if Single DC node is available.
		minClusterResReq := getMinClusterResReq()
		subtractResources(resReq, minClusterResReq)

		clusterRes := getAvailComputeResources(ic.Store.Computes[tempIdx].Compute, resReq)
		ic.Store.Computes[tempIdx].ClusterRes = *clusterRes

		ic.logger.Debugf("Cluster Resources available at DC that will host Ctrl + SR + Cluster are: %v",
			ic.Store.Computes[tempIdx].ClusterRes)
	}

	return tempIdx
}

func CheckResAvailabilityOnAllCompNodes(ic *InfraContext) error {
	ic.logger.Debug("Inside CheckResAvailabilityOnAllCompNodes()")

	if isInfraCtxtInitialised(ic) == false {
		errMsg := "Infra Context not initialised properly"
		return errors.New(errMsg)
	}

	computes := ic.Store.Computes

	for i, _ := range computes {
		compute := computes[i].Compute

		if computes[i].IsCtrlHost == true {
			ic.logger.Debugf("Skipping Compute Node: '%s' for res check as it is already choosen as Ctrl Host",
				compute.Name)
			continue
		}
		ic.logger.Debugf("Evaluating Compute Node: '%s' in Tier: '%s' for resources",
			compute.Name, compute.Tier)
		switch compute.Tier {
		case "data_centre":
			err := computeResReqForDataCentreAndEdge(&computes[i], ic)
			if err != nil {
				ic.logger.Errorf("Error returned by computeResReqForDataCentreAndEdge()")
				return err
			}
		case "edge":
			err := computeResReqForDataCentreAndEdge(&computes[i], ic)
			if err != nil {
				ic.logger.Errorf("Error returned by computeResReqForDataCentreAndEdge()")
				return err
			}
		case "far_edge":
			err := computeResReqForFarEdge(&computes[i], ic)
			if err != nil {
				ic.logger.Errorf("Error returned by computeResReqForFarEdge()")
				return err
			}
		case "mist":
			err := computeResReqForMist(&computes[i], ic)
			if err != nil {
				ic.logger.Errorf("Error returned by computeResReqForMist()")
				return err
			}
		}
	}
	return nil
}

func computeResReqForDataCentreAndEdge(cInfo *ComputeInfo, ic *InfraContext) error {

	flavorsMap := ic.Store.FlavorsMap

	consumedRes := Resources{}
	resReq := Resources{}

	// For other DC nodes, add SR + Cluster
	ic.logger.Debugf("Checking availability of required resources on %s node: '%s'",
		cInfo.Compute.Tier, cInfo.Compute.Name)

	srFlavor, _ := flavorsMap[FlavorSr]

	srResReq := getResourceFromFlavor(srFlavor)
	ic.logger.Debugf("SR resource requirement: %v", srResReq)

	// Add SR resource requirement in consumed resources.
	addResources(&resReq, srResReq)

	// Add Min. cluster resource requirement.
	// Min. resources for cluster are 2 VCPUs, 2048 MB RAM, 2 GB Disk.
	minClusterResReq := getMinClusterResReq()
	ic.logger.Debugf("Min. cluster resource requirement: %v", minClusterResReq)

	addResources(&resReq, minClusterResReq)

	ic.logger.Debugf("Min. resources required: %v", resReq)

	// if min resources required for (SRpoa and SR + Cluster) nodes are not available, return error.
	if false == areReqdResourcesAvailable(cInfo.Compute, &consumedRes, &resReq) {
		er := fmt.Sprintf("Min. resources required are not available on %s Compute Node: '%s'",
			cInfo.Compute.Tier, cInfo.Compute.Name)
		ic.logger.Errorf("%s", er)

		return errors.New(er)
	}

	// Calculate remaining resources on Compute Node after fulfilling SR + SRpoa requirements.
	availRes := getAvailComputeResources(cInfo.Compute, &srResReq)
	ic.logger.Debugf("Avail res on Compute Node after fulfilling SR res req: %v",
		*availRes)

	// Assign remaining resources to cluster i.e. update cluster resource in
	// computeInfo so that cluster flavor can be created for this Compute Node later.
	cInfo.ClusterRes = *availRes

	ic.logger.Debugf("Cluster Flavor to be created for Compute Node: '%s' is : %v",
		cInfo.Compute.Name, cInfo.ClusterRes)

	return nil
}

func computeResReqForFarEdge(cInfo *ComputeInfo, ic *InfraContext) error {

	flavorsMap := ic.Store.FlavorsMap

	consumedRes := Resources{}
	resReq := Resources{}

	ic.logger.Debugf("Checking availability of required resources on Far-Edge node: '%s'",
		cInfo.Compute.Name)
	// Find number of access networks attached to the Compute Node.
	accessNWs, found := cInfo.NetworkCatNetworksMap["access"]
	if !found {
		er := fmt.Sprintf("No Access N/W attached to Far-Edge Compute Node: '%s'",
			cInfo.Compute.Name)
		ic.logger.Errorf("%s", er)

		return errors.New(er)
	}
	numAccessNWs := len(accessNWs)
	ic.logger.Debugf("%d Access N/Ws attached to Compute Node", numAccessNWs)

	srFlavor, _ := flavorsMap[FlavorSr]

	// Total SR resource requirements =
	// NUM_ACCESS_NW * (Res Reqd for Single SR + Res Reqd for single SR-PoA)
	srResReq := getScaledFlavorResources(srFlavor, (2 * numAccessNWs))
	ic.logger.Debugf("SR resource requirement: %v", srResReq)

	// Add SR resource requirement in consumed resources.
	addResources(&resReq, srResReq)

	// Add Min. cluster resource requirement.
	// Min. resources for cluster are 2 VCPUs, 2048 MB RAM, 2 GB Disk.
	// Number of cluster resources = NUM_ACCESS_NW
	minClusterResReq := getMinClusterResReq()
	getScaledResources(&minClusterResReq, numAccessNWs)
	ic.logger.Debugf("Min. cluster resource requirement: %v", minClusterResReq)

	addResources(&resReq, minClusterResReq)

	ic.logger.Debugf("Min. resources required: %v", resReq)

	// if min resources required for (SRpoa and SR + Cluster) nodes are not available, return error.
	if false == areReqdResourcesAvailable(cInfo.Compute, &consumedRes, &resReq) {
		er := fmt.Sprintf("Min. resources required are not available on %s Compute Node: '%s'",
			cInfo.Compute.Tier, cInfo.Compute.Name)
		ic.logger.Errorf("%s", er)

		return errors.New(er)
	}

	// Calculate remaining resources on Compute Node after fulfilling SR + SRpoa requirements.
	availRes := getAvailComputeResources(cInfo.Compute, &srResReq)
	ic.logger.Debugf("Avail res on Compute Node after fulfilling SR + SRpoa res req: %v",
		*availRes)

	// Assign remaining resources to clusters, where each cluster has
	// (Avail Resources/NUM_ACCESS_NW) resources.
	// Update cluster resource in computeInfo so that cluster flavor can be
	// created for this Compute Node later.
	cInfo.ClusterRes = *availRes
	getScaledDownResources(&cInfo.ClusterRes, numAccessNWs)

	ic.logger.Debugf("Cluster Flavor to be created for Compute Node: '%s' is : %v",
		cInfo.Compute.Name, cInfo.ClusterRes)

	return nil
}

func computeResReqForMist(cInfo *ComputeInfo, ic *InfraContext) error {

	flavorsMap := ic.Store.FlavorsMap

	consumedRes := Resources{}
	resReq := Resources{}

	ic.logger.Debugf("Checking availability of required resources on Mist node: '%s'",
		cInfo.Compute.Name)
	// Find number of access networks attached to the Compute Node.
	accessNWs, found := cInfo.NetworkCatNetworksMap["access"]
	if !found {
		er := fmt.Sprintf("No Access N/W attached to Mist Compute Node: '%s'",
			cInfo.Compute.Name)
		ic.logger.Errorf("%s", er)

		return errors.New(er)
	}
	numAccessNWs := len(accessNWs)
	ic.logger.Debugf("%d Access N/Ws attached to Compute Node", numAccessNWs)

	srFlavor, _ := flavorsMap[FlavorSr]

	// Total SR resource requirements =
	// NUM_ACCESS_NW * Res Reqd for single SR-PoA
	srResReq := getScaledFlavorResources(srFlavor, numAccessNWs)
	ic.logger.Debugf("SR resource requirement: %v", srResReq)

	// Add SR resource requirement in consumed resources.
	addResources(&resReq, srResReq)

	ic.logger.Debugf("Min. resources required: %v", resReq)

	// if min resources required for SRpoa nodes are not available, return error.
	if false == areReqdResourcesAvailable(cInfo.Compute, &consumedRes, &resReq) {
		er := fmt.Sprintf("Min. resources required are not available on %s Compute Node: '%s'",
			cInfo.Compute.Tier, cInfo.Compute.Name)
		ic.logger.Errorf("%s", er)

		return errors.New(er)
	}

	return nil
}

func areReqdResourcesAvailable(compute *models.Compute, consumedRes *Resources, reqdRes *Resources) bool {
	availRes := getAvailComputeResources(compute, consumedRes)
	if availRes.Vcpus < reqdRes.Vcpus || availRes.RAM < reqdRes.RAM || availRes.Disk < reqdRes.Disk {
		return false
	}

	return true
}

func addResources(out *Resources, resVal Resources) {
	(*out).Vcpus = (*out).Vcpus + resVal.Vcpus
	(*out).RAM = (*out).RAM + resVal.RAM
	(*out).Disk = (*out).Disk + resVal.Disk
}

func getScaledFlavorResources(flavor *models.Flavor, scaleFactor int) Resources {
	resource := Resources{}

	resource.Vcpus = flavor.Vcpus * scaleFactor
	resource.RAM = flavor.RAM * scaleFactor
	resource.Disk = flavor.Disk * scaleFactor

	return resource
}

func getResourceFromFlavor(flavor *models.Flavor) Resources {
	return getScaledFlavorResources(flavor, 1)
}

func getMinClusterResReq() Resources {
	return Resources{Vcpus: 2, RAM: 2048, Disk: 2}
}

func subtractResources(out *Resources, resVal Resources) {
	(*out).Vcpus = (*out).Vcpus - resVal.Vcpus
	(*out).RAM = (*out).RAM - resVal.RAM
	(*out).Disk = (*out).Disk - resVal.Disk
}

func getAvailComputeResources(compute *models.Compute, consumedRes *Resources) *Resources {
	var availRes Resources

	availRes.Vcpus = compute.Vcpus - consumedRes.Vcpus
	availRes.RAM = compute.RAM - consumedRes.RAM
	availRes.Disk = compute.Disk - consumedRes.Disk

	return &availRes
}

func getScaledResources(res *Resources, scaleFactor int) {
	(*res).Vcpus = (*res).Vcpus * scaleFactor
	(*res).RAM = (*res).RAM * scaleFactor
	(*res).Disk = (*res).Disk * scaleFactor
}

func getScaledDownResources(res *Resources, scaleDownFactor int) {
	(*res).Vcpus = (*res).Vcpus / scaleDownFactor
	(*res).RAM = (*res).RAM / scaleDownFactor
	(*res).Disk = (*res).Disk / scaleDownFactor
}

func GetReqResToHostFn(ic *InfraContext, fn string) *Resources {
	reqRes := &Resources{}
	flavorsMap := ic.Store.FlavorsMap

	switch fn {
	case "ctrl_func":
		reqRes, _ = getCtrlFuncResReq(ic)
	case "sr_cluster":
		// Only adding SR resource requirements. Cluster resource requirements
		// will be different for each node hosting SR_CLUSTER fn. So, those will
		// be added later while calculating platform resource requirements.
		srFlavor, _ := flavorsMap[FlavorSr]
		srResReq := getResourceFromFlavor(srFlavor)
		addResources(reqRes, srResReq)
	case "sr_poa":
		srFlavor, _ := flavorsMap[FlavorSr]
		srResReq := getResourceFromFlavor(srFlavor)
		addResources(reqRes, srResReq)
	}

	return reqRes
}

func isAllReqFlavorsExist(flavorMap map[string]*models.Flavor) error {
	if _, exist := flavorMap[FlavorClmc]; !exist {
		errMsg := "Flavor " + FlavorClmc + " is not present in DB"
		return errors.New(errMsg)
	}
	if _, exist := flavorMap[FlavorFrontend]; !exist {
		errMsg := "Flavor " + FlavorFrontend + " is not present in DB"
		return errors.New(errMsg)
	}
	if _, exist := flavorMap[FlavorMoose]; !exist {
		errMsg := "Flavor " + FlavorMoose + " is not present in DB"
		return errors.New(errMsg)
	}
	if _, exist := flavorMap[FlavorPce]; !exist {
		errMsg := "Flavor " + FlavorPce + " is not present in DB"
		return errors.New(errMsg)
	}
	if _, exist := flavorMap[FlavorPs]; !exist {
		errMsg := "Flavor " + FlavorPs + " is not present in DB"
		return errors.New(errMsg)
	}
	if _, exist := flavorMap[FlavorSfemc]; !exist {
		errMsg := "Flavor " + FlavorSfemc + " is not present in DB"
		return errors.New(errMsg)
	}
	if _, exist := flavorMap[FlavorSr]; !exist {
		errMsg := "Flavor " + FlavorSr + " is not present in DB"
		return errors.New(errMsg)
	}
	if _, exist := flavorMap[FlavorNm]; !exist {
		errMsg := "Flavor " + FlavorNm + " is not present in DB"
		return errors.New(errMsg)
	}
	return nil
}

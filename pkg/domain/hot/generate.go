package hot

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/util"
)

type context struct {
	subnetSia             string
	infraServiceDns       string
	infraServiceSdnCtrl   string
	securityGroupMgmt     string
	securityGroupMsp      string
	securityGroupSdnctrl  string
	securityGroupSia      string
	securityGroupWan      string
	configMtu             string
	configCidr            string
	configTenantId        string
	configOsCliVersion    string
	configArdentVersion   string
	configEnableIpv4Rules string
	configSiaIpFrontend   string
	configParentDomain    string
	configDhcpAgents      string

	srCount             int
	lanPrefix           string
	nodePasswd          string
	prevResource        string
	ctrlFuncHostIdx     int
	isOnlySingleDCAvail bool
	lanSrIpOskMax       string
	mspIpMin            string
	mspIpSfemc          string
	mspIpClmc           string
	mspIpMoose          string
	mspIpFrontend       string
	mspIpNm             string
}

const (
	keypair   = "flame"
	subnetMsp = "flame-msp"

	passwdLen = 12
	charset   = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	heatTemplateVersion = "2017-02-24"
)

func generateHeatTemplate(infraCtxt *util.InfraContext, nodePasswd string) (*[]byte, error) {

	localCtxt := initContext()
	localCtxt.nodePasswd = nodePasswd

	err := prepareForHeatTempGeneration(infraCtxt, localCtxt)
	if err != nil {
		logger.Errorf("Error returned by prepareForHeatTempGeneration(). %v", err)
		return nil, err
	}

	err = util.CheckResAvailabilityOnAllCompNodes(infraCtxt)
	if err != nil {
		logger.Errorf("Error returned by checkResAvailabilityOnAllCompNodes(). %v", err)
		return nil, err
	}

	// append bytes to template and write into HEAT template file
	var template []byte

	template = append(template, "heat_template_version: "+heatTemplateVersion+"\n\n"...)
	template = append(template, "description: FLAME platform with clusters "+
		"and platform services\n\n"...)
	template = append(template, "resources:\n"...)

	// Iterate computes
	computes := infraCtxt.Store.Computes
	// counter for data-centre(dc), edge(e), far-edge(fe) and mist(m) nodes
	var dc, e, fe, m int
	for i := 0; i < len(computes); i++ {
		logger.Debugf("compute_node tier : %s", computes[i].Compute.Tier)
		switch computes[i].Compute.Tier {
		case "data_centre":
			dc = dc + 1
			var srClusterCount int = 1
			if localCtxt.ctrlFuncHostIdx == i {
				appendControlFunctions(&template, "dc", dc, localCtxt, &computes[i])
				if localCtxt.isOnlySingleDCAvail == true {
					appendSrAndCluster(&template, "dc", dc, srClusterCount, localCtxt,
						&computes[i])
				}
			} else {
				appendSrAndCluster(&template, "dc", dc, srClusterCount, localCtxt,
					&computes[i])
			}
		case "edge":
			e = e + 1
			var srClusterCount int = 1
			appendSrAndCluster(&template, "e", e, srClusterCount, localCtxt,
				&computes[i])
		case "far_edge":
			fe = fe + 1
			// get access networks attached to the compute node
			accessNwsList := computes[i].NetworkCatNetworksMap["access"]

			var srPoaCount, srClusterCount int

			// Iterate accessNwsList to create SRpoa
			for j := 0; j < len(accessNwsList); j++ {
				srPoaCount = srPoaCount + 1
				appendSrPoa(&template, "fe", fe, srPoaCount, localCtxt,
					&computes[i])
			}
			for j := 0; j < len(accessNwsList); j++ {
				srClusterCount = srClusterCount + 1
				appendSrAndCluster(&template, "fe", fe, srClusterCount, localCtxt,
					&computes[i])
			}
		case "mist":
			m = m + 1

			// get access networks attached to the compute node
			accessNwsList := computes[i].NetworkCatNetworksMap["access"]

			var srPoaCount int

			// Iterate accessNwsList to create SRpoa
			for j := 0; j < len(accessNwsList); j++ {
				srPoaCount = srPoaCount + 1
				appendSrPoa(&template, "mist", m, srPoaCount, localCtxt,
					&computes[i])
			}
		}
	}

	return &template, nil
}

func initContext() *context {
	return &context{}
}

func prepareForHeatTempGeneration(infraCtxt *util.InfraContext, localCtxt *context) error {
	// fill derived Infra Values in localCtxt
	getLocalCtxtValsFromInfraCtxtStore(infraCtxt, localCtxt)

	// check if there are multiple DC nodes available
	isMultipleDCAvail, err := util.CheckIfMultipleDCsAvail(infraCtxt)
	if err != nil {
		logger.Errorf("Error returned by CheckIfMultipleDCsAvail(). %v", err)
		return err
	}
	localCtxt.isOnlySingleDCAvail = !(isMultipleDCAvail)

	// fetch index of compute node hosting control functions
	idx, err := util.FetchCompNodeForCtrlFunctions(infraCtxt)
	if err != nil {
		logger.Errorf("Error returned by fetchCompNodeForCtrlFunctions(). %v", err)
		return err
	}
	localCtxt.ctrlFuncHostIdx = idx

	return nil
}

func generateNodePasswd() string {
	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	psswd := make([]byte, passwdLen)
	for i := 0; i < len(psswd); i++ {
		psswd[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(psswd)
}

func addClusterFlavorsToDB(computes []util.ComputeInfo, repo models.Repository) error {

	logger.Debug("inside addClusterFlavorsToDB()")

	// Add cluster flavors to DB
	ifc := make([]interface{}, 0)
	flavorSlice := make([]models.Flavor, 0)

	for i := 0; i < len(computes); i++ {
		clusterRes := computes[i].ClusterRes
		if clusterRes.Vcpus != 0 && clusterRes.RAM != 0 && clusterRes.Disk != 0 {
			logger.Debugf("Cluster Resources : %v", clusterRes)
			flavor := models.Flavor{
				Name:  getClusterFlavorName(&computes[i]),
				Vcpus: clusterRes.Vcpus,
				RAM:   clusterRes.RAM,
				Disk:  clusterRes.Disk,
			}
			flavorSlice = append(flavorSlice, flavor)
		}
	}

	// append flavorSlice to ifc
	ifc = append(ifc, flavorSlice)

	logger.Debugf("ifc : %v", ifc)

	err := repo.Add(ifc)
	if err != nil {
		logger.Errorf("Error returned by Add() Interface. %v", err)
		return err
	}

	return nil
}

func deleteClusterFlavorsFromDB(repo models.Repository) error {

	logger.Debug("inside deleteClusterFlavorsFromDB()")

	// Delete all the existing cluster flavors in DB
	flavor := models.Flavor{Vcpus: -1, RAM: -1, Disk: -1}
	q := models.Query{Entity: flavor}
	flavors, err := repo.Get(&q)
	if err != nil {
		logger.Errorf("Error returned by Get() Interface. %v", err)
		errMsg := "Failed to retrieve flavors from storage"
		return errors.New(errMsg)
	}
	flavorsArr := flavors.([]models.Flavor)
	for i := 0; i < len(flavorsArr); i++ {
		flavorName := flavorsArr[i].Name
		if strings.Contains(flavorName, "flame-cluster") {
			// Removing cluster flavor from storage
			err = repo.Remove(models.Flavor{Name: flavorName, Vcpus: -1,
				RAM: -1, Disk: -1})
			if err != nil {
				errMsg := "Error in deleting cluster flavor " + flavorName + " from Storage"
				logger.Errorf("%s. %v", errMsg, err)
				return errors.New(errMsg)
			}
		}
	}

	return nil
}

func addNodePasswdToDB(passwd string, repo models.Repository) error {

	logger.Debug("inside addNodePasswdToDB()")

	// Deleting existing node-passwd in DB
	err := repo.Remove(models.Config{ConfKey: "node-passwd"})
	if err != nil {
		errMsg := "Error in deleting 'node-passwd' from Storage"
		logger.Errorf("%s. %v", errMsg, err)
		return errors.New(errMsg)
	}

	// Add node password to DB
	ifc := make([]interface{}, 0)
	configSlice := make([]models.Config, 0)

	config := models.Config{
		ConfKey: "node-passwd",
		Value:   passwd,
	}
	configSlice = append(configSlice, config)

	// append configSlice to ifc
	ifc = append(ifc, configSlice)

	logger.Debugf("ifc : %v", ifc)

	err = repo.Add(ifc)
	if err != nil {
		logger.Errorf("Error returned by Add() Interface. %v", err)
		return err
	}

	return nil
}

func getLocalCtxtValsFromInfraCtxtStore(infraCtxt *util.InfraContext, localCtxt *context) {

	for i := 0; i < len(infraCtxt.Store.Subnets); i++ {
		if infraCtxt.Store.Subnets[i].Category == "sia" {
			localCtxt.subnetSia = infraCtxt.Store.Subnets[i].Identifier
		}
	}

	for i := 0; i < len(infraCtxt.Store.InfraServices); i++ {
		if infraCtxt.Store.InfraServices[i].ServiceType == "dns" {
			localCtxt.infraServiceDns = infraCtxt.Store.InfraServices[i].Value
		} else if infraCtxt.Store.InfraServices[i].ServiceType == "sdn_controller" {
			localCtxt.infraServiceSdnCtrl = infraCtxt.Store.InfraServices[i].Value
		}
	}

	for i := 0; i < len(infraCtxt.Store.SecurityGroups); i++ {
		if infraCtxt.Store.SecurityGroups[i].Category == "mgmt" {
			localCtxt.securityGroupMgmt = infraCtxt.Store.SecurityGroups[i].Identifier
		} else if infraCtxt.Store.SecurityGroups[i].Category == "msp" {
			localCtxt.securityGroupMsp = infraCtxt.Store.SecurityGroups[i].Identifier
		} else if infraCtxt.Store.SecurityGroups[i].Category == "sdnctrl" {
			localCtxt.securityGroupSdnctrl = infraCtxt.Store.SecurityGroups[i].Identifier
		} else if infraCtxt.Store.SecurityGroups[i].Category == "sia" {
			localCtxt.securityGroupSia = infraCtxt.Store.SecurityGroups[i].Identifier
		} else if infraCtxt.Store.SecurityGroups[i].Category == "wan" {
			localCtxt.securityGroupWan = infraCtxt.Store.SecurityGroups[i].Identifier
		}
	}
	for i := 0; i < len(infraCtxt.Store.Metadata); i++ {
		if infraCtxt.Store.Metadata[i].ConfKey == "mtu" {
			localCtxt.configMtu = infraCtxt.Store.Metadata[i].Value
		} else if infraCtxt.Store.Metadata[i].ConfKey == "cidr" {
			localCtxt.configCidr = infraCtxt.Store.Metadata[i].Value
			cidrParts := strings.Split(infraCtxt.Store.Metadata[i].Value, ".")
			localCtxt.lanPrefix = cidrParts[0] + "." + cidrParts[1] + "."
		} else if infraCtxt.Store.Metadata[i].ConfKey == "os-tenant-id" {
			localCtxt.configTenantId = infraCtxt.Store.Metadata[i].Value
		} else if infraCtxt.Store.Metadata[i].ConfKey == "os-cli-version" {
			localCtxt.configOsCliVersion = infraCtxt.Store.Metadata[i].Value
		} else if infraCtxt.Store.Metadata[i].ConfKey == "ardent-version" {
			localCtxt.configArdentVersion = infraCtxt.Store.Metadata[i].Value
		} else if infraCtxt.Store.Metadata[i].ConfKey == "enable-ipv4-rules" {
			localCtxt.configEnableIpv4Rules = infraCtxt.Store.Metadata[i].Value
		} else if infraCtxt.Store.Metadata[i].ConfKey == "sia-ip-frontend" {
			localCtxt.configSiaIpFrontend = infraCtxt.Store.Metadata[i].Value
		} else if infraCtxt.Store.Metadata[i].ConfKey == "parent-domain" {
			localCtxt.configParentDomain = infraCtxt.Store.Metadata[i].Value
		} else if infraCtxt.Store.Metadata[i].ConfKey == "dhcp_agents" {
			localCtxt.configDhcpAgents = infraCtxt.Store.Metadata[i].Value
			dhcpAgentsVal, _ := strconv.Atoi(infraCtxt.Store.Metadata[i].Value)
			oskRangeMaxVal := 2 + dhcpAgentsVal - 1 + 10
			localCtxt.lanSrIpOskMax = strconv.Itoa(oskRangeMaxVal)
			mspIpMinVal := 2 + dhcpAgentsVal
			localCtxt.mspIpMin = strconv.Itoa(mspIpMinVal)
			localCtxt.mspIpSfemc = strconv.Itoa(mspIpMinVal + dhcpAgentsVal)
			localCtxt.mspIpClmc = strconv.Itoa(mspIpMinVal + dhcpAgentsVal + 1)
			localCtxt.mspIpMoose = strconv.Itoa(mspIpMinVal + dhcpAgentsVal + 2)
			localCtxt.mspIpFrontend = strconv.Itoa(mspIpMinVal + dhcpAgentsVal + 3)
			localCtxt.mspIpNm = strconv.Itoa(mspIpMinVal + dhcpAgentsVal + 4)
		}
	}
}

func getClusterFlavorName(compInfo *util.ComputeInfo) string {
	var flavorName string
	flavorName = "flame-cluster-" +
		compInfo.Compute.Name + "-" +
		compInfo.Compute.AvailZone
	return flavorName
}

func appendControlFunctions(template *[]byte, prefix string, nodeCnt int,
	localCtxt *context, compInfo *util.ComputeInfo) {

	id := compInfo.Compute.Name + "-" + compInfo.Compute.AvailZone + "-pce1-nm1-sr1-ps1"
	availZone := compInfo.Compute.AvailZone + ":" + compInfo.Compute.Name

	*template = append(*template, ("\n  " + id + ":\n")...)
	*template = append(*template, "    type: "+heatDirPath+"/stack-pce-nm-sr-ps.yaml\n"...)
	*template = append(*template, "    properties:\n"...)
	*template = append(*template, "      security-group-mgmt: "+localCtxt.securityGroupMgmt+"\n"...)
	*template = append(*template, "      security-group-sdnctrl: "+localCtxt.securityGroupSdnctrl+"\n"...)
	*template = append(*template, "      security-group-msp: "+localCtxt.securityGroupMsp+"\n"...)
	*template = append(*template, "      security-group-wan: "+localCtxt.securityGroupWan+"\n"...)
	*template = append(*template, "      tmpl-name: "+id+"\n"...)
	*template = append(*template, "      zone: "+availZone+"\n"...)
	*template = append(*template, "      pce-flavor: "+util.FlavorPce+"\n"...)
	*template = append(*template, "      nm-flavor: "+util.FlavorNm+"\n"...)
	*template = append(*template, "      sr-flavor: "+util.FlavorSr+"\n"...)
	*template = append(*template, "      ps-flavor: "+util.FlavorPs+"\n"...)
	*template = append(*template, "      base-key: "+keypair+"\n"...)
	*template = append(*template, "      node-passwd: "+localCtxt.nodePasswd+"\n"...)
	*template = append(*template, "      enable-ipv4-rules: "+localCtxt.configEnableIpv4Rules+"\n"...)
	*template = append(*template, "      network-data: "+compInfo.NetworkCatNetworksMap["data"][0]+"\n"...)
	*template = append(*template, "      network-wan: "+compInfo.NetworkCatNetworksMap["wan"][0]+"\n"...)
	*template = append(*template, "      network-sdnctrl: "+compInfo.NetworkCatNetworksMap["sdnctrl"][0]+"\n"...)
	*template = append(*template, "      network-mgmt: "+compInfo.NetworkCatNetworksMap["mgmt"][0]+"\n"...)
	*template = append(*template, "      network-msp: "+compInfo.NetworkCatNetworksMap["msp"][0]+"\n"...)
	*template = append(*template, "      network-lan: "+compInfo.NetworkCatNetworksMap["ps"][0]+"\n"...)
	*template = append(*template, "      subnet-msp: "+subnetMsp+"\n"...)
	*template = append(*template, "      mtu: "+localCtxt.configMtu+"\n"...)
	*template = append(*template, "      lan-cidr: "+localCtxt.configCidr+"\n"...)
	*template = append(*template, "      lan-prefix: "+localCtxt.lanPrefix+"\n"...)
	*template = append(*template, "      lan-dns-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      lan-gw-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      infra-sdn-controller-ip: "+localCtxt.infraServiceSdnCtrl+"\n"...)
	*template = append(*template, "      infra-dns-ip: "+localCtxt.infraServiceDns+"\n"...)
	*template = append(*template, "      lan-dhcp-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      lan-mask: 255.255.0.0\n"...)
	*template = append(*template, "      lan-sr-ip-prefix: "+localCtxt.lanPrefix+"1."+"\n"...)
	*template = append(*template, "      lan-sr-ip-base: "+localCtxt.lanPrefix+"1.0"+"\n"...)
	*template = append(*template, "      lan-sr-ip-mask: 255.255.255.0\n"...)
	*template = append(*template, "      lan-sr-ip-osk-min: "+localCtxt.lanPrefix+"1.2"+"\n"...)
	*template = append(*template, "      lan-sr-ip-osk-max: "+localCtxt.lanPrefix+"1."+localCtxt.lanSrIpOskMax+"\n"...)
	*template = append(*template, "      msp-ip-cidr: "+localCtxt.lanPrefix+"255.0/24"+"\n"...)
	*template = append(*template, "      msp-ip-min: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpMin+"\n"...)
	*template = append(*template, "      msp-ip-max: "+localCtxt.lanPrefix+"255.99"+"\n"...)
	*template = append(*template, "      msp-ip-nm: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpNm+"\n"...)
	*template = append(*template, "      sfid-parent-domain: "+localCtxt.configParentDomain+"\n"...)
	if localCtxt.prevResource != "" {
		*template = append(*template, "    depends_on: "+localCtxt.prevResource+"\n"...)
	}
	localCtxt.prevResource = id

	id = compInfo.Compute.Name + "-" + compInfo.Compute.AvailZone + "-sr2-clmc1-sfemc1"
	*template = append(*template, ("\n  " + id + ":\n")...)
	*template = append(*template, "    type: "+heatDirPath+"/stack-sr-clmc-sfemc.yaml\n"...)
	*template = append(*template, "    properties:\n"...)
	*template = append(*template, "      security-group-mgmt: "+localCtxt.securityGroupMgmt+"\n"...)
	*template = append(*template, "      security-group-sdnctrl: "+localCtxt.securityGroupSdnctrl+"\n"...)
	*template = append(*template, "      security-group-msp: "+localCtxt.securityGroupMsp+"\n"...)
	*template = append(*template, "      tmpl-name: "+id+"\n"...)
	*template = append(*template, "      zone: "+availZone+"\n"...)
	*template = append(*template, "      sr-flavor: "+util.FlavorSr+"\n"...)
	*template = append(*template, "      clmc-flavor: "+util.FlavorClmc+"\n"...)
	*template = append(*template, "      sfemc-flavor: "+util.FlavorSfemc+"\n"...)
	*template = append(*template, "      base-key: "+keypair+"\n"...)
	*template = append(*template, "      node-passwd: "+localCtxt.nodePasswd+"\n"...)
	*template = append(*template, "      network-data: "+compInfo.NetworkCatNetworksMap["data"][0]+"\n"...)
	*template = append(*template, "      network-sdnctrl: "+compInfo.NetworkCatNetworksMap["sdnctrl"][0]+"\n"...)
	*template = append(*template, "      network-mgmt: "+compInfo.NetworkCatNetworksMap["mgmt"][0]+"\n"...)
	*template = append(*template, "      network-msp: "+compInfo.NetworkCatNetworksMap["msp"][0]+"\n"...)
	*template = append(*template, "      network-lan: "+compInfo.NetworkCatNetworksMap["clmc-sfemc"][0]+"\n"...)
	*template = append(*template, "      mtu: "+localCtxt.configMtu+"\n"...)
	*template = append(*template, "      lan-cidr: "+localCtxt.configCidr+"\n"...)
	*template = append(*template, "      lan-dns-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      lan-gw-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      infra-sdn-controller-ip: "+localCtxt.infraServiceSdnCtrl+"\n"...)
	*template = append(*template, "      enable-ipv4-rules: "+localCtxt.configEnableIpv4Rules+"\n"...)
	*template = append(*template, "      lan-dhcp-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      lan-sr-ip-prefix: "+localCtxt.lanPrefix+"2."+"\n"...)
	*template = append(*template, "      lan-sr-ip-base: "+localCtxt.lanPrefix+"2.0"+"\n"...)
	*template = append(*template, "      lan-sr-ip-mask: 255.255.255.0\n"...)
	*template = append(*template, "      lan-sr-ip-osk-min: "+localCtxt.lanPrefix+"2.2"+"\n"...)
	*template = append(*template, "      lan-sr-ip-osk-max: "+localCtxt.lanPrefix+"2."+localCtxt.lanSrIpOskMax+"\n"...)
	*template = append(*template, "      msp-ip-sfemc: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpSfemc+"\n"...)
	*template = append(*template, "      msp-ip-clmc: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpClmc+"\n"...)
	*template = append(*template, "      sfid-parent-domain: "+localCtxt.configParentDomain+"\n"...)
	if localCtxt.prevResource != "" {
		*template = append(*template, "    depends_on: "+localCtxt.prevResource+"\n"...)
	}
	localCtxt.prevResource = id

	id = compInfo.Compute.Name + "-" + compInfo.Compute.AvailZone + "-frontend1"
	*template = append(*template, ("\n  " + id + ":\n")...)
	*template = append(*template, "    type: "+heatDirPath+"/stack-frontend.yaml\n"...)
	*template = append(*template, "    properties:\n"...)
	*template = append(*template, "      security-group-sia: "+localCtxt.securityGroupSia+"\n"...)
	*template = append(*template, "      security-group-msp: "+localCtxt.securityGroupMsp+"\n"...)
	*template = append(*template, "      name: "+id+"\n"...)
	*template = append(*template, "      zone: "+availZone+"\n"...)
	*template = append(*template, "      flavor: "+util.FlavorFrontend+"\n"...)
	*template = append(*template, "      base-key: "+keypair+"\n"...)
	*template = append(*template, "      node-passwd: "+localCtxt.nodePasswd+"\n"...)
	*template = append(*template, "      network-sia: "+compInfo.NetworkCatNetworksMap["sia"][0]+"\n"...)
	*template = append(*template, "      network-msp: "+compInfo.NetworkCatNetworksMap["msp"][0]+"\n"...)
	*template = append(*template, "      subnet-sia: "+localCtxt.subnetSia+"\n"...)
	*template = append(*template, "      subnet-msp: "+subnetMsp+"\n"...)
	*template = append(*template, "      sia-ip-frontend: "+localCtxt.configSiaIpFrontend+"\n"...)
	*template = append(*template, "      msp-ip-sfemc: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpSfemc+"\n"...)
	*template = append(*template, "      msp-ip-clmc: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpClmc+"\n"...)
	*template = append(*template, "      msp-ip-moose: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpMoose+"\n"...)
	*template = append(*template, "      msp-ip-frontend: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpFrontend+"\n"...)
	*template = append(*template, "      msp-ip-nm: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpNm+"\n"...)
	if localCtxt.prevResource != "" {
		*template = append(*template, "    depends_on: "+localCtxt.prevResource+"\n"...)
	}
	localCtxt.prevResource = id

	id = compInfo.Compute.Name + "-" + compInfo.Compute.AvailZone + "-moose1"
	*template = append(*template, ("\n  " + id + ":\n")...)
	*template = append(*template, "    type: "+heatDirPath+"/stack-moose.yaml\n"...)
	*template = append(*template, "    properties:\n"...)
	*template = append(*template, "      security-group-mgmt: "+localCtxt.securityGroupMgmt+"\n"...)
	*template = append(*template, "      security-group-sdnctrl: "+localCtxt.securityGroupSdnctrl+"\n"...)
	*template = append(*template, "      security-group-msp: "+localCtxt.securityGroupMsp+"\n"...)
	*template = append(*template, "      name: "+id+"\n"...)
	*template = append(*template, "      zone: "+availZone+"\n"...)
	*template = append(*template, "      flavor: "+util.FlavorMoose+"\n"...)
	*template = append(*template, "      base-key: "+keypair+"\n"...)
	*template = append(*template, "      node-passwd: "+localCtxt.nodePasswd+"\n"...)
	*template = append(*template, "      infra-sdn-controller-ip: "+localCtxt.infraServiceSdnCtrl+"\n"...)
	*template = append(*template, "      enable-ipv4-rules: "+localCtxt.configEnableIpv4Rules+"\n"...)
	*template = append(*template, "      network-data: "+compInfo.NetworkCatNetworksMap["data"][0]+"\n"...)
	*template = append(*template, "      network-sdnctrl: "+compInfo.NetworkCatNetworksMap["sdnctrl"][0]+"\n"...)
	*template = append(*template, "      network-mgmt: "+compInfo.NetworkCatNetworksMap["mgmt"][0]+"\n"...)
	*template = append(*template, "      network-msp: "+compInfo.NetworkCatNetworksMap["msp"][0]+"\n"...)
	*template = append(*template, "      msp-ip-moose: "+localCtxt.lanPrefix+"255."+localCtxt.mspIpMoose+"\n"...)
	*template = append(*template, "      mtu: "+localCtxt.configMtu+"\n"...)
	if localCtxt.prevResource != "" {
		*template = append(*template, "    depends_on: "+localCtxt.prevResource+"\n"...)
	}
	localCtxt.prevResource = id
}

func appendSrAndCluster(template *[]byte, prefix string, nodeCnt int, srClusterCount int,
	localCtxt *context, compInfo *util.ComputeInfo) {

	localCtxt.srCount = localCtxt.srCount + 1

	id := compInfo.Compute.Name + "-" + compInfo.Compute.AvailZone + "-sr" + strconv.Itoa(srClusterCount) +
		"-cluster" + strconv.Itoa(srClusterCount)
	availZone := compInfo.Compute.AvailZone + ":" + compInfo.Compute.Name
	clusterFlavor := getClusterFlavorName(compInfo)

	*template = append(*template, ("\n  " + id + ":\n")...)
	*template = append(*template, "    type: "+heatDirPath+"/stack-sr-cluster.yaml\n"...)
	*template = append(*template, "    properties:\n"...)
	*template = append(*template, "      security-group-mgmt: "+localCtxt.securityGroupMgmt+"\n"...)
	*template = append(*template, "      security-group-sdnctrl: "+localCtxt.securityGroupSdnctrl+"\n"...)
	*template = append(*template, "      tmpl-name: "+id+"\n"...)
	*template = append(*template, "      zone: "+availZone+"\n"...)
	*template = append(*template, "      sr-flavor: "+util.FlavorSr+"\n"...)
	*template = append(*template, "      cluster-flavor: "+clusterFlavor+"\n"...) //LEFT
	*template = append(*template, "      base-key: "+keypair+"\n"...)
	*template = append(*template, "      node-passwd: "+localCtxt.nodePasswd+"\n"...)
	*template = append(*template, "      network-data: "+compInfo.NetworkCatNetworksMap["data"][0]+"\n"...)
	*template = append(*template, "      network-sdnctrl: "+compInfo.NetworkCatNetworksMap["sdnctrl"][0]+"\n"...)
	*template = append(*template, "      network-mgmt: "+compInfo.NetworkCatNetworksMap["mgmt"][0]+"\n"...)
	*template = append(*template, "      network-lan: "+compInfo.NetworkCatNetworksMap["cluster"][0]+"\n"...)
	*template = append(*template, "      mtu: "+localCtxt.configMtu+"\n"...)
	*template = append(*template, "      lan-cidr: "+localCtxt.configCidr+"\n"...)
	*template = append(*template, "      lan-dns-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      lan-gw-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      infra-sdn-controller-ip: "+localCtxt.infraServiceSdnCtrl+"\n"...)
	*template = append(*template, "      enable-ipv4-rules: "+localCtxt.configEnableIpv4Rules+"\n"...)
	*template = append(*template, "      lan-dhcp-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      lan-sr-ip-prefix: "+localCtxt.lanPrefix+strconv.Itoa(localCtxt.srCount+2)+"."+"\n"...)
	*template = append(*template, "      lan-sr-ip-base: "+localCtxt.lanPrefix+strconv.Itoa(localCtxt.srCount+2)+".0"+"\n"...)
	*template = append(*template, "      lan-sr-ip-mask: 255.255.255.0\n"...)
	*template = append(*template, "      lan-sr-ip-osk-min: "+localCtxt.lanPrefix+strconv.Itoa(localCtxt.srCount+2)+".2"+"\n"...)
	*template = append(*template, "      lan-sr-ip-osk-max: "+localCtxt.lanPrefix+strconv.Itoa(localCtxt.srCount+2)+"."+
		localCtxt.lanSrIpOskMax+"\n"...)
	*template = append(*template, "      sfid-parent-domain: "+localCtxt.configParentDomain+"\n"...)
	if localCtxt.prevResource != "" {
		*template = append(*template, "    depends_on: "+localCtxt.prevResource+"\n"...)
	}
	localCtxt.prevResource = id
}

func appendSrPoa(template *[]byte, prefix string, nodeCnt int, srPoaCount int,
	localCtxt *context, compInfo *util.ComputeInfo) {

	localCtxt.srCount = localCtxt.srCount + 1

	id := compInfo.Compute.Name + "-" + compInfo.Compute.AvailZone + "-srpoa" + strconv.Itoa(srPoaCount)
	availZone := compInfo.Compute.AvailZone + ":" + compInfo.Compute.Name

	*template = append(*template, ("\n  " + id + ":\n")...)
	*template = append(*template, "    type: "+heatDirPath+"/stack-sr.yaml\n"...)
	*template = append(*template, "    properties:\n"...)
	*template = append(*template, "      security-group-mgmt: "+localCtxt.securityGroupMgmt+"\n"...)
	*template = append(*template, "      security-group-sdnctrl: "+localCtxt.securityGroupSdnctrl+"\n"...)
	*template = append(*template, "      tmpl-name: "+id+"\n"...)
	*template = append(*template, "      zone: "+availZone+"\n"...)
	*template = append(*template, "      sr-flavor: "+util.FlavorSr+"\n"...)
	*template = append(*template, "      base-key: "+keypair+"\n"...)
	*template = append(*template, "      node-passwd: "+localCtxt.nodePasswd+"\n"...)
	*template = append(*template, "      network-data: "+compInfo.NetworkCatNetworksMap["data"][0]+"\n"...)
	*template = append(*template, "      network-sdnctrl: "+compInfo.NetworkCatNetworksMap["sdnctrl"][0]+"\n"...)
	*template = append(*template, "      network-mgmt: "+compInfo.NetworkCatNetworksMap["mgmt"][0]+"\n"...)
	*template = append(*template, "      network-access: "+compInfo.NetworkCatNetworksMap["access"][srPoaCount-1]+"\n"...)
	*template = append(*template, "      mtu: "+localCtxt.configMtu+"\n"...)
	*template = append(*template, "      lan-cidr: "+localCtxt.configCidr+"\n"...)
	*template = append(*template, "      lan-dns-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      lan-gw-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      infra-sdn-controller-ip: "+localCtxt.infraServiceSdnCtrl+"\n"...)
	*template = append(*template, "      lan-dhcp-ip: "+localCtxt.lanPrefix+"1.1"+"\n"...)
	*template = append(*template, "      lan-sr-ip-prefix: "+localCtxt.lanPrefix+strconv.Itoa(localCtxt.srCount+2)+"."+"\n"...)
	*template = append(*template, "      lan-sr-ip-base: "+localCtxt.lanPrefix+strconv.Itoa(localCtxt.srCount+2)+".0"+"\n"...)
	*template = append(*template, "      lan-sr-ip-mask: 255.255.255.0\n"...)
	*template = append(*template, "      lan-sr-ip-osk-min: "+localCtxt.lanPrefix+strconv.Itoa(localCtxt.srCount+2)+".2"+"\n"...)
	*template = append(*template, "      lan-sr-ip-osk-max: "+localCtxt.lanPrefix+strconv.Itoa(localCtxt.srCount+2)+"."+
		localCtxt.lanSrIpOskMax+"\n"...)
	*template = append(*template, "      sfid-parent-domain: "+localCtxt.configParentDomain+"\n"...)
	*template = append(*template, "      enable-ipv4-rules: "+localCtxt.configEnableIpv4Rules+"\n"...)
	if localCtxt.prevResource != "" {
		*template = append(*template, "    depends_on: "+localCtxt.prevResource+"\n"...)
	}
	localCtxt.prevResource = id
}

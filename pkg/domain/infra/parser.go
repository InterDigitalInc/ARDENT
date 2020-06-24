package infra

import (
	"errors"
	"io/ioutil"
	"net"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

type yml struct {
	ComputeNodes   map[string]models.Compute       `yaml:"compute_nodes"`
	Networks       map[string]models.Network       `yaml:"networks"`
	Subnets        map[string]models.Subnet        `yaml:"subnets"`
	SecurityGroups map[string]models.SecurityGroup `yaml:"security_groups"`
	InfraServices  models.InfraServices            `yaml:"infrastructure_services"`
	Metadata       models.Metadata                 `yaml:"metadata"`
}

type baseYml struct {
	ComputeNodes struct {
		Required    bool `yaml:"required"`
		ComputeNode struct {
			AvailabilityZone struct {
				Required    bool   `yaml:"required"`
				Type        string `yaml:"type"`
				Description string `yaml:"description"`
			} `yaml:"availability_zone"`
			Name struct {
				Required    bool   `yaml:"required"`
				Type        string `yaml:"type"`
				Description string `yaml:"description"`
			} `yaml:"name"`
			Vcpus struct {
				Required bool   `yaml:"required"`
				Type     string `yaml:"type"`
			} `yaml:"vcpus"`
			RAM struct {
				Required    bool   `yaml:"required"`
				Type        string `yaml:"type"`
				Description string `yaml:"description"`
			} `yaml:"ram"`
			Disk struct {
				Required    bool   `yaml:"required"`
				Type        string `yaml:"type"`
				Description string `yaml:"description"`
			} `yaml:"disk"`
			Networks struct {
				Required    bool   `yaml:"required"`
				Type        string `yaml:"type"`
				TypeSchema  string `yaml:"type_schema"`
				Description string `yaml:"description"`
			} `yaml:"networks"`
			Tier struct {
				Required    bool     `yaml:"required"`
				Type        string   `yaml:"type"`
				Description string   `yaml:"description"`
				ValidValues []string `yaml:"valid_values"`
			} `yaml:"tier"`
		} `yaml:"compute_node"`
	} `yaml:"compute_nodes"`
	Networks struct {
		Required bool `yaml:"required"`
		Network  struct {
			Identifier struct {
				Required bool   `yaml:"required"`
				Type     string `yaml:"type"`
			} `yaml:"identifier"`
			Category struct {
				Required    bool     `yaml:"required"`
				Type        string   `yaml:"type"`
				ValidValues []string `yaml:"valid_values"`
			} `yaml:"category"`
		} `yaml:"network"`
	} `yaml:"networks"`
	Subnets struct {
		Required bool `yaml:"required"`
		Subnet   struct {
			Identifier struct {
				Required bool   `yaml:"required"`
				Type     string `yaml:"type"`
			} `yaml:"identifier"`
			Category struct {
				Required    bool     `yaml:"required"`
				Type        string   `yaml:"type"`
				ValidValues []string `yaml:"valid_values"`
				Description string   `yaml:"description"`
			} `yaml:"category"`
		} `yaml:"subnet"`
	} `yaml:"subnets"`
	SecurityGroups struct {
		Required      bool `yaml:"required"`
		SecurityGroup struct {
			Identifier struct {
				Required bool   `yaml:"required"`
				Type     string `yaml:"type"`
			} `yaml:"identifier"`
			Category struct {
				Required    bool     `yaml:"required"`
				Type        string   `yaml:"type"`
				ValidValues []string `yaml:"valid_values"`
			} `yaml:"category"`
		} `yaml:"security_group"`
	} `yaml:"security_groups"`
	InfrastructureServices struct {
		Required bool `yaml:"required"`
		DNS      struct {
			Required    bool   `yaml:"required"`
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
		} `yaml:"dns"`
		SdnController struct {
			Required    bool   `yaml:"required"`
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
		} `yaml:"sdn_controller"`
	} `yaml:"infrastructure_services"`
	Metadata struct {
		Required bool `yaml:"required"`
		Tenant   struct {
			Required    bool   `yaml:"required"`
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
		} `yaml:"tenant"`
		Cidr struct {
			Required    bool   `yaml:"required"`
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
		} `yaml:"cidr"`
		Mtu struct {
			Required    bool   `yaml:"required"`
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
		} `yaml:"mtu"`
		SiaIPFrontend struct {
			Required    bool   `yaml:"required"`
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
		} `yaml:"sia-ip-frontend"`
		Ipv4Rules struct {
			Required    bool   `yaml:"required"`
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
		} `yaml:"ipv4-rules"`
		DhcpAgents struct {
			Required    bool   `yaml:"required"`
			Type        string `yaml:"type"`
			Description string `yaml:"description"`
		} `yaml:"dhcp_agents"`
	} `yaml:"metadata"`
}

var baseDescriptor baseYml

func parseDefinitions() error {

	// Read infra-descriptor-definitions.yml and initialize baseDescriptor
	if _, err := os.Stat(descDefFilePath); os.IsNotExist(err) {
		logger.Errorf("Error in getting infra-descriptor-definitions.yml file stat. %v", err)
		errMsg := "Failed to get infra-descriptor-definitions.yml file stat"
		return errors.New(errMsg)
	}
	fileBytes, err := ioutil.ReadFile(descDefFilePath)
	if err != nil {
		logger.Errorf("Error in reading infra-descriptor-definitions.yml file. %v", err)
		errMsg := "Failed to read infra-descriptor-definitions.yml file"
		return errors.New(errMsg)
	}
	err = yaml.UnmarshalStrict(fileBytes, &baseDescriptor)
	if err != nil {
		logger.Errorf("Error in unmarshaling infra-descriptor-definitions.yml. %v", err)
		errMsg := "Failed to unmarshal infra-descriptor-definitions.yml"
		return errors.New(errMsg)
	}
	logger.Debug("Initialized baseDescriptor successfully")

	return nil
}

func parseAndValidate(fileBytes []byte, descriptor *yml) error {

	logger.Debug("Going to unmarshal fileBytes into descriptor")
	err := yaml.UnmarshalStrict(fileBytes, descriptor)
	if err != nil {
		logger.Errorf("Error in unmarshaling infra descriptor. %v", err)
		errMsg := "Failed to unmarshal infra descriptor"
		return errors.New(errMsg)
	}
	logger.Debug("Unmarshal successful")

	logger.Debug("Going to validate descriptor against baseDescriptor")
	// Validate descriptor compute_nodes
	err = validateComputeNodeProperties(descriptor.ComputeNodes)
	if err != nil {
		logger.Debugf("Error in descriptor compute_nodes validation: %v", err)
		return err
	}

	// Validate descriptor networks
	err = validateNetworkProperties(descriptor.Networks)
	if err != nil {
		logger.Debugf("Error in descriptor networks validation: %v", err)
		return err
	}
	// Validate descriptor subnets
	err = validateSubnetProperties(descriptor.Subnets)
	if err != nil {
		logger.Debugf("Error in descriptor subnets validation: %v", err)
		return err
	}

	// Validate descriptor security_groups
	err = validateSecurityGroupProperties(descriptor.SecurityGroups)
	if err != nil {
		logger.Debugf("Error in descriptor security_groups validation: %v", err)
		return err
	}

	// Validate descriptor infrastructure_services
	err = validateInfrastructureServiceProperties(descriptor.InfraServices)
	if err != nil {
		logger.Debugf("Error in descriptor infrastructure_services validation: %v", err)
		return err
	}

	// Validate descriptor metadata
	err = validateMetadataProperties(descriptor.Metadata)
	if err != nil {
		logger.Debugf("Error in descriptor metadata validation: %v", err)
		return err
	}
	logger.Debug("Validation Successfull")

	return nil
}

func validateComputeNodeProperties(computeNodes map[string]models.Compute) error {
	if baseDescriptor.ComputeNodes.Required == true {
		if len(computeNodes) == 0 {
			errMsg := "compute_nodes is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
	}

	for _, compNode := range computeNodes {
		if baseDescriptor.ComputeNodes.ComputeNode.Disk.Required == true {
			if compNode.Disk == 0 {
				errMsg := "disk is empty or not present for compute node in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("disk for compute node in infra descriptor is: %d", compNode.Disk)
		}

		if baseDescriptor.ComputeNodes.ComputeNode.AvailabilityZone.Required == true {
			if compNode.AvailZone == "" {
				errMsg := "availability_zone is empty or not present for compute node in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("availability_zone for compute node in infra descriptor is: %s", compNode.AvailZone)
		}

		if baseDescriptor.ComputeNodes.ComputeNode.Name.Required == true {
			if compNode.Name == "" {
				errMsg := "name is empty or not present for compute node in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("name for compute node in infra descriptor is: %s", compNode.Name)
		}

		if baseDescriptor.ComputeNodes.ComputeNode.RAM.Required == true {
			if compNode.RAM == 0 {
				errMsg := "ram is empty or not present for compute node in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("ram for compute node in infra descriptor is: %d", compNode.RAM)
		}

		if baseDescriptor.ComputeNodes.ComputeNode.Vcpus.Required == true {
			if compNode.Vcpus == 0 {
				errMsg := "vcpus is empty or not present for compute node in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("vcpus for compute node in infra descriptor is: %d", compNode.Vcpus)
		}

		if baseDescriptor.ComputeNodes.ComputeNode.Networks.Required == true {
			if len(compNode.Networks) == 0 {
				errMsg := "networks is empty or not present for compute node in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("networks for compute node in infra descriptor is: %s", compNode.Networks)
		}

		if baseDescriptor.ComputeNodes.ComputeNode.Tier.Required == true {
			if compNode.Tier == "" {
				errMsg := "tier is empty or not present for compute node in infra descriptor"
				return errors.New(errMsg)
			}

			validValues := baseDescriptor.ComputeNodes.ComputeNode.Tier.ValidValues
			for j := 0; j < len(validValues); j++ {
				if compNode.Tier == validValues[j] {
					break
				}
				if j == len(validValues)-1 {
					errMsg := "tier is not a valid value for compute node in infra descriptor"
					return errors.New(errMsg)
				}
			}

			logger.Debugf("tier for compute node in infra descriptor is: %s", compNode.Tier)
		} else {
			if compNode.Tier != "" {
				validValues := baseDescriptor.ComputeNodes.ComputeNode.Tier.ValidValues
				for j := 0; j < len(validValues); j++ {
					if compNode.Tier == validValues[j] {
						break
					}
					if j == len(validValues)-1 {
						errMsg := "tier is not a valid value for compute node in infra descriptor"
						return errors.New(errMsg)
					}
				}
			}
		}
	}

	return nil
}

func validateNetworkProperties(networks map[string]models.Network) error {
	if baseDescriptor.Networks.Required == true {
		if len(networks) == 0 {
			errMsg := "networks is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
	}

	for _, netWork := range networks {
		if baseDescriptor.Networks.Network.Category.Required == true {
			if netWork.Category == "" {
				errMsg := "category is empty or not present for network in infra descriptor"
				return errors.New(errMsg)
			}
			validValues := baseDescriptor.Networks.Network.Category.ValidValues
			for j := 0; j < len(validValues); j++ {
				if netWork.Category == validValues[j] {
					break
				}
				if j == len(validValues)-1 {
					errMsg := "category is not a valid value for network in infra descriptor"
					return errors.New(errMsg)
				}
			}

			logger.Debugf("category for network in infra descriptor is: %s", netWork.Category)
		} else {
			if netWork.Category != "" {
				validValues := baseDescriptor.Networks.Network.Category.ValidValues
				for j := 0; j < len(validValues); j++ {
					if netWork.Category == validValues[j] {
						break
					}
					if j == len(validValues)-1 {
						errMsg := "category is not a valid value for network in infra descriptor"
						return errors.New(errMsg)
					}
				}
			}
		}
		if baseDescriptor.Networks.Network.Identifier.Required == true {
			if netWork.Identifier == "" {
				errMsg := "identifier is empty or not present for network in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("identifier for network in infra descriptor is: %s", netWork.Identifier)
		}
	}

	return nil
}

func validateSubnetProperties(subnets map[string]models.Subnet) error {
	if baseDescriptor.Subnets.Required == true {
		if len(subnets) == 0 {
			errMsg := "subnets is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
	}

	for _, subNet := range subnets {
		if baseDescriptor.Subnets.Subnet.Category.Required == true {
			if subNet.Category == "" {
				errMsg := "category is empty or not present for subnet in infra descriptor"
				return errors.New(errMsg)
			}
			validValues := baseDescriptor.Subnets.Subnet.Category.ValidValues
			for j := 0; j < len(validValues); j++ {
				if subNet.Category == validValues[j] {
					break
				}
				if j == len(validValues)-1 {
					errMsg := "category is not a valid value for subnet in infra descriptor"
					return errors.New(errMsg)
				}
			}

			logger.Debugf("category for subnet in infra descriptor is: %s", subNet.Category)
		} else {
			if subNet.Category != "" {
				validValues := baseDescriptor.Subnets.Subnet.Category.ValidValues
				for j := 0; j < len(validValues); j++ {
					if subNet.Category == validValues[j] {
						break
					}
					if j == len(validValues)-1 {
						errMsg := "category is not a valid value for subnet in infra descriptor"
						return errors.New(errMsg)
					}
				}
			}
		}
		if baseDescriptor.Subnets.Subnet.Identifier.Required == true {
			if subNet.Identifier == "" {
				errMsg := "identifier is empty or not present for subnet in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("identifier for subnet in infra descriptor is: %s", subNet.Identifier)
		}
	}

	return nil
}

func validateSecurityGroupProperties(securityGroups map[string]models.SecurityGroup) error {
	if baseDescriptor.SecurityGroups.Required == true {
		if len(securityGroups) == 0 {
			errMsg := "security_groups is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
	}

	for _, securityGrp := range securityGroups {
		if baseDescriptor.SecurityGroups.SecurityGroup.Category.Required == true {
			if securityGrp.Category == "" {
				errMsg := "category is empty or not present for security group in infra descriptor"
				return errors.New(errMsg)
			}
			validValues := baseDescriptor.SecurityGroups.SecurityGroup.Category.ValidValues
			for j := 0; j < len(validValues); j++ {
				if securityGrp.Category == validValues[j] {
					break
				}
				if j == len(validValues)-1 {
					errMsg := "category is not a valid value for security group in infra descriptor"
					return errors.New(errMsg)
				}
			}

			logger.Debugf("category for security group in infra descriptor is: %s", securityGrp.Category)
		} else {
			if securityGrp.Category != "" {
				validValues := baseDescriptor.SecurityGroups.SecurityGroup.Category.ValidValues
				for j := 0; j < len(validValues); j++ {
					if securityGrp.Category == validValues[j] {
						break
					}
					if j == len(validValues)-1 {
						errMsg := "category is not a valid value for security group in infra descriptor"
						return errors.New(errMsg)
					}
				}
			}
		}
		if baseDescriptor.SecurityGroups.SecurityGroup.Identifier.Required == true {
			if securityGrp.Identifier == "" {
				errMsg := "identifier is empty or not present for security group in infra descriptor"
				return errors.New(errMsg)
			}

			logger.Debugf("identifier for security group in infra descriptor is: %s", securityGrp.Identifier)
		}
	}

	return nil
}

func validateInfrastructureServiceProperties(infraServices models.InfraServices) error {
	if baseDescriptor.InfrastructureServices.Required == true {
		if (models.InfraServices{}) == infraServices {
			errMsg := "infrastructure_services is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
	}

	if baseDescriptor.InfrastructureServices.DNS.Required == true {
		if infraServices.DNS == "" {
			errMsg := "dns is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
		ip := net.ParseIP(infraServices.DNS)
		if ip == nil {
			errMsg := "dns is not in proper IP format in infra descriptor"
			return errors.New(errMsg)
		}

		logger.Debugf("dns in infra descriptor is: %s", infraServices.DNS)
	} else {
		if infraServices.DNS != "" {
			ip := net.ParseIP(infraServices.DNS)
			if ip == nil {
				errMsg := "dns is not in proper IP format in infra descriptor"
				return errors.New(errMsg)
			}
		}
	}
	if baseDescriptor.InfrastructureServices.SdnController.Required == true {
		if infraServices.SdnController == "" {
			errMsg := "sdn_controller is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
		ip := net.ParseIP(infraServices.SdnController)
		if ip == nil {
			errMsg := "sdn_controller is not in proper IP format in infra descriptor"
			return errors.New(errMsg)
		}

		logger.Debugf("sdn_controller in infra descriptor is: %s", infraServices.SdnController)
	} else {
		if infraServices.SdnController != "" {
			ip := net.ParseIP(infraServices.SdnController)
			if ip == nil {
				errMsg := "sdn_controller is not in proper IP format in infra descriptor"
				return errors.New(errMsg)
			}
		}
	}

	return nil
}

func validateMetadataProperties(metadata models.Metadata) error {
	if baseDescriptor.Metadata.Required == true {
		if (models.Metadata{}) == metadata {
			errMsg := "metadata is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
	}

	if baseDescriptor.Metadata.Cidr.Required == true {
		if metadata.Cidr == "" {
			errMsg := "cidr is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
		_, _, err := net.ParseCIDR(metadata.Cidr)
		if err != nil {
			errMsg := "cidr is not in proper CIDR format in infra descriptor"
			return errors.New(errMsg)
		}

		logger.Debugf("cidr in infra descriptor is: %s", metadata.Cidr)
	} else {
		if metadata.Cidr != "" {
			_, _, err := net.ParseCIDR(metadata.Cidr)
			if err != nil {
				errMsg := "cidr is not in proper CIDR format in infra descriptor"
				return errors.New(errMsg)
			}
		}
	}
	if baseDescriptor.Metadata.Tenant.Required == true {
		if metadata.Tenant == "" {
			errMsg := "tenant is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}

		logger.Debugf("tenant in infra descriptor is: %s", metadata.Tenant)
	}
	if baseDescriptor.Metadata.Mtu.Required == true {
		if metadata.Mtu == 0 {
			errMsg := "mtu is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}

		logger.Debugf("mtu in infra descriptor is: %d", metadata.Mtu)
	}
	if baseDescriptor.Metadata.SiaIPFrontend.Required == true {
		if metadata.SiaIpFrontend == "" {
			errMsg := "sia-ip-frontend is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
		ip := net.ParseIP(metadata.SiaIpFrontend)
		if ip == nil {
			errMsg := "sia-ip-frontend is not in proper IP format in infra descriptor"
			return errors.New(errMsg)
		}

		logger.Debugf("sia-ip-frontend in infra descriptor is: %s", metadata.SiaIpFrontend)
	} else {
		if metadata.SiaIpFrontend != "" {
			ip := net.ParseIP(metadata.SiaIpFrontend)
			if ip == nil {
				errMsg := "sia-ip-frontend is not in proper IP format in infra descriptor"
				return errors.New(errMsg)
			}
		}
	}
	if baseDescriptor.Metadata.Ipv4Rules.Required == true {
		if metadata.Ipv4Rules == "" {
			errMsg := "ipv4-rules is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}
		_, err := strconv.ParseBool(metadata.Ipv4Rules)
		if err != nil {
			errMsg := "ipv4-rules is not a valid value in infra descriptor"
			return errors.New(errMsg)
		}
	} else {
		if metadata.Ipv4Rules != "" {
			_, err := strconv.ParseBool(metadata.Ipv4Rules)
			if err != nil {
				errMsg := "ipv4-rules is not a valid value in infra descriptor"
				return errors.New(errMsg)
			}
		}
	}
	if baseDescriptor.Metadata.DhcpAgents.Required == true {
		if metadata.DhcpAgents == 0 {
			errMsg := "dhcp_agents is empty or not present in infra descriptor"
			return errors.New(errMsg)
		}

		logger.Debugf("dhcp_agents in infra descriptor is: %d", metadata.DhcpAgents)
	}

	return nil
}

func isDescriptorAlreadyProcessed(repo models.Repository) (bool, error) {

	isProcessed := false

	compute := models.Compute{Vcpus: -1, RAM: -1, Disk: -1}
	q := models.Query{Entity: compute}

	comps, err := repo.Get(&q)
	if err != nil {
		logger.Errorf("Error returned by Get() Interface. %v", err)
		return isProcessed, err
	}
	logger.Debugf("comps : %+v", comps)
	if comps != nil {
		logger.Debugf("Compute Nodes retrieved from Storage")
		isProcessed = true
	}

	return isProcessed, nil
}

package infra

import (
	"testing"

	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

func TestNewService(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	_, err := NewService(logger, nil)
	if err != nil {
		t.Errorf("infra service returned unexpected error msg: "+
			"received - %s. expected - nil", err.Error())
	}
}

func TestParseAndValidate(t *testing.T) {

	t.Run("parseNilBytes", func(t *testing.T) {
		descriptor := yml{}
		err := parseAndValidate(nil, &descriptor)
		expected := "compute_nodes is empty in infra descriptor."
		if err.Error() != expected {
			t.Errorf("parser returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
}

func TestValidateComputeNodeProperties(t *testing.T) {

	t.Run("successful", func(t *testing.T) {
		computeNodes := map[string]models.Compute{
			"os-edge-1": {
				Name:      "cn0",
				AvailZone: "nova",
				Vcpus:     4,
				RAM:       2,
				Disk:      500,
				Networks:  []string{"mgmt"},
				Tier:      "edge",
			},
		}
		err := validateComputeNodeProperties(computeNodes)
		if err != nil {
			t.Errorf("parser returned unexpected error msg: "+
				"received - %s. expected - nil", err.Error())
		}
	})
}

func TestValidateNetworkProperties(t *testing.T) {

	t.Run("successful", func(t *testing.T) {
		networks := map[string]models.Network{
			"flame-mgmt": {
				Identifier: "43e39fb0-626f-45f0-a35f-73ec0ca62e80",
				Category:   "mgmt",
			},
		}
		err := validateNetworkProperties(networks)
		if err != nil {
			t.Errorf("parser returned unexpected error msg: "+
				"received - %s. expected - nil", err.Error())
		}
	})
}

func TestValidateSubnetProperties(t *testing.T) {

	t.Run("successful", func(t *testing.T) {
		subnets := map[string]models.Subnet{
			"flame-sia": {
				Identifier: "43e39fb0-626f-45f0-a35f-73ec0ca62e80",
				Category:   "sia",
			},
		}
		err := validateSubnetProperties(subnets)
		if err != nil {
			t.Errorf("parser returned unexpected error msg: "+
				"received - %s. expected - nil", err.Error())
		}
	})
}

func TestValidateSecurityGroupProperties(t *testing.T) {

	t.Run("successful", func(t *testing.T) {
		securityGroups := map[string]models.SecurityGroup{
			"flame-mgmt": {
				Identifier: "43e39fb0-626f-45f0-a35f-73ec0ca62e80",
				Category:   "mgmt",
			},
		}
		err := validateSecurityGroupProperties(securityGroups)
		if err != nil {
			t.Errorf("parser returned unexpected error msg: "+
				"received - %s. expected - nil", err.Error())
		}
	})
}

func TestValidateInfrastructureServiceProperties(t *testing.T) {

	t.Run("successful", func(t *testing.T) {
		infraServices := models.InfraServices{
			DNS:           "192.168.121.2",
			SdnController: "192.168.121.3",
		}
		err := validateInfrastructureServiceProperties(infraServices)
		if err != nil {
			t.Errorf("parser returned unexpected error msg: "+
				"received - %s. expected - nil", err.Error())
		}
	})
}

func TestValidateMetadataProperties(t *testing.T) {

	t.Run("successful", func(t *testing.T) {
		metadata := models.Metadata{
			Tenant:        "43e39fb0-626f-45f0-a35f-73ec0ca62e80",
			Cidr:          "192.168.121.5/24",
			Mtu:           1500,
			SiaIpFrontend: "192.168.1.3",
		}
		err := validateMetadataProperties(metadata)
		if err != nil {
			t.Errorf("parser returned unexpected error msg: "+
				"received - %s. expected - nil", err.Error())
		}
	})
}

package mysql

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

type testAdd struct {
	name string
	in   interface{}
	err  error
	out  interface{}
}

type testQuery struct {
	name string
	in   models.Query
	out  interface{}
}

type testRemove struct {
	name          string
	deleteAllRows interface{}
	insertRows    interface{}
	deleteRows    interface{}
	getRows       models.Query
	out           interface{}
}

var s *Storage = nil

var er error = errors.New("X")

func TestWrongDBCred(t *testing.T) {
	logger = logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	_, err := NewStorage(logger, "root111", "hsc321", "ardent_test")
	if err == nil {
		t.Errorf("Storage pointer is nil")
		t.Failed()
	}
}

func TestWrongDBName(t *testing.T) {
	logger = logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	_, err := NewStorage(logger, "root", "hsc321", "ardent_test_1")
	if err == nil {
		t.Errorf("Storage pointer is nil")
		t.Failed()
	}
}

func TestNewStorage(t *testing.T) {
	logger = logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	slocal, err := NewStorage(logger, "root", "hsc321", "ardent_test")
	if err != nil {
		t.Errorf("Storage pointer is nil")
		t.Failed()
	}
	s = slocal
}

func TestAddNil(t *testing.T) {
	err := s.Add(nil)

	if err == nil {
		t.Errorf("TestAddNil: Case Failed")
		t.Failed()
	}
}

func TestAddGetCompNw(t *testing.T) {

	if s == nil {
		t.Errorf("TestAddGetCompNw: can't execute, because Storage pointer is nil")
		t.Failed()
		return
	}

	i := make([]interface{}, 0)

	// Populate Networks
	n := make([]models.Network, 0)
	for i := 0; i < 2; i++ {
		nname := fmt.Sprintf("n%d", i)
		cat := ""
		if i%2 == 0 {
			cat = "access"
		} else {
			cat = "wan"
		}
		nx := models.Network{
			Identifier: nname,
			Category:   cat,
		}
		n = append(n, nx)
	}

	// Populate Computes
	c := make([]models.Compute, 0)
	for i := 0; i < 10; i++ {

		networks := []string{}
		for j := 0; j < (i+1) && j < 2; j++ {
			networks = append(networks, n[j].Identifier)
		}

		cname := fmt.Sprintf("cn%d", i)
		cx := models.Compute{
			Name:      cname,
			AvailZone: "nova",
			Tier:      "edge",
			Vcpus:     i + 1,
			Disk:      i*2 + 2048,
			RAM:       i + 15,
			Networks:  networks,
		}
		c = append(c, cx)
	}

	i = append(i, c)
	i = append(i, n)

	err := s.Add(i)

	if err != nil {
		t.Errorf("TestAddGetCompNw: failed, because Add() failed")
		return
	}

	// Query Computes
	var cq models.Query
	cq.Entity = models.Compute{Vcpus: -1, RAM: -1, Disk: -1}
	out, err := s.Get(&cq)
	if err != nil {
		t.Errorf("TestAddGetCompNw: failed, because Get() failed for Compute")
		return
	}
	t.Logf("Out: %v, Err: %v", out, err)
	// Compare input with output
	out_p := out.([]models.Compute)
	for i, _ := range c {
		// Setting Networks as nil because query will not return networks.
		c[i].Networks = nil
		if !compare(c[i], out_p[i]) {
			t.Errorf("TestAddGetCompNw: failed, because of mismatch in Compute output")
			return
		}
	}

	// Query Networks
	var nq models.Query
	nq.Entity = models.Network{}
	out, err = s.Get(&nq)
	if err != nil {
		t.Errorf("TestAddGetCompNw: failed, because Get() failed for Network")
		return
	}
	t.Logf("Out: %v, Err: %v", out, err)
	// Compare input with output
	out_n := out.([]models.Network)
	for i, _ := range n {
		if !compare(n[i], out_n[i]) {
			t.Errorf("TestAddGetCompNw: failed, because of mismatch in Network output")
			return
		}
	}

	// Query Networks connected with Compute node 'cn0'
	var cnq models.Query
	cnq.Entity = models.Network{}
	comp := models.Compute{Name: "cn0", Vcpus: -1, RAM: -1, Disk: -1}
	cnq.And(comp)

	out, err = s.Get(&cnq)
	if err != nil {
		t.Errorf("TestAddGetCompNw: failed, because Get() failed for Network")
		return
	}
	t.Logf("Networks Connected with Compute Node:'cn0' are: %v, error: %v", out, err)

	// Find all Compute node connected to N/W of type 'wan'
	var cnq1 models.Query
	cnq1.Entity = models.Compute{Vcpus: -1, RAM: -1, Disk: -1}
	net := models.Network{Category: "wan"}
	cnq1.And(net)

	out, err = s.Get(&cnq1)
	if err != nil {
		t.Errorf("TestAddGetCompNw: failed, because Get() failed for Computes with network 'wan'")
		return
	}
	t.Logf("Compute Nodes connected with 'wan' are: %v, error: %v", out, err)

}

var testInserts = []testAdd{
	testAdd{
		"addGetSubnets",
		[]models.Subnet{
			models.Subnet{
				Identifier: "s1",
				Category:   "sia",
			},
			models.Subnet{
				Identifier: "s2",
				Category:   "sia",
			},
		},
		nil,
		nil,
	},
	testAdd{
		"addDuplicateSubnets",
		[]models.Subnet{
			models.Subnet{
				Identifier: "s3",
				Category:   "sia",
			},
			models.Subnet{
				Identifier: "s3",
				Category:   "sia",
			},
		},
		er,
		nil,
	},
	testAdd{
		"addGetSecurityGroup",
		[]models.SecurityGroup{
			models.SecurityGroup{
				Identifier: "sg1",
				Category:   "mgmt",
			},
			models.SecurityGroup{
				Identifier: "sg2",
				Category:   "msp",
			},
		},
		nil,
		nil,
	},
	testAdd{
		"addDuplicateSecurityGroup",
		[]models.SecurityGroup{
			models.SecurityGroup{
				Identifier: "sg1",
				Category:   "mgmt",
			},
			models.SecurityGroup{
				Identifier: "sg1",
				Category:   "msp",
			},
		},
		er,
		nil,
	},
	testAdd{
		"addGetInfraService",
		[]models.InfraService{
			models.InfraService{
				ServiceType: "dns",
				Value:       "192.168.121.2",
			},
			models.InfraService{
				ServiceType: "sdn_controller",
				Value:       "192.168.121.3",
			},
		},
		nil,
		nil,
	},
	testAdd{
		"duplicateInfraService",
		[]models.InfraService{
			models.InfraService{
				ServiceType: "dns",
				Value:       "192.168.121.2",
			},
			models.InfraService{
				ServiceType: "dns",
				Value:       "192.168.121.5",
			},
		},
		er,
		nil,
	},
	testAdd{
		"addGetConfig",
		[]models.Config{
			models.Config{
				ConfKey: "cidr",
				Value:   "192.168.121.0/24",
			},
			models.Config{
				ConfKey: "mtu",
				Value:   "1500",
			},
			models.Config{
				ConfKey: "os-tenant-id",
				Value:   "aaaa",
			},
			models.Config{
				ConfKey: "sia-ip-frontend",
				Value:   "192.168.121.10",
			},
		},
		nil,
		[]models.Config{
			models.Config{
				ConfKey: "cidr",
				Value:   "192.168.121.0/24",
			},
			models.Config{
				ConfKey: "enable-ipv4-rules",
				Value:   "0",
			},
			models.Config{
				ConfKey: "mtu",
				Value:   "1500",
			},
			models.Config{
				ConfKey: "os-tenant-id",
				Value:   "aaaa",
			},
			models.Config{
				ConfKey: "parent-domain",
				Value:   "ict-flame.eu",
			},
			models.Config{
				ConfKey: "sia-ip-frontend",
				Value:   "192.168.121.10",
			},
		},
	},
	testAdd{
		"duplicateConfig",
		[]models.Config{
			models.Config{
				ConfKey: "mtu",
				Value:   "1500",
			},
			models.Config{
				ConfKey: "mtu",
				Value:   "1600",
			},
		},
		er,
		nil,
	},
}

func TestAddGetEntities(t *testing.T) {
	if s == nil {
		t.Errorf("TestAddGet: can't execute, because Storage pointer is nil")
		t.Failed()
		return
	}

	for _, tc := range testInserts {
		t.Run(tc.name, func(t *testing.T) {
			i := make([]interface{}, 0)
			i = append(i, tc.in)

			err := s.Add(i)
			if tc.err != nil {
				if err == nil {
					t.Errorf("TestAddGet: failed, because Add() failed")
					return
				}
				return
			}

			if err != nil {
				t.Errorf("TestAddGet: failed, because Add() failed")
				return
			}

			// Query
			var sq models.Query

			ename := reflect.TypeOf(tc.in).Elem().Name()

			switch ename {
			case "Network":
				sq.Entity = models.Network{}
			case "Subnet":
				sq.Entity = models.Subnet{}
			case "InfraService":
				sq.Entity = models.InfraService{}
			case "SecurityGroup":
				sq.Entity = models.SecurityGroup{}
			case "Config":
				sq.Entity = models.Config{}
			}

			out, err := s.Get(&sq)
			if err != nil {
				t.Errorf("TestAddGet: failed, because Get() failed")
				return
			}
			t.Logf("Input: %v", tc.in)
			t.Logf("Actual: %v", out)
			t.Logf("Expected: %v", tc.out)

			exp := tc.in
			if tc.out != nil {
				exp = tc.out
			}
			// Compare input with output
			if !compare(exp, out) {
				t.Errorf("TestAddGet: failed, because of mismatch in output")
				return
			}
			err = s.Remove(sq.Entity)
			if err != nil {
				t.Errorf("TestAddGet: failed, because Remove() failed")
				return
			}
		})
	}
}

var testQueries = []testQuery{
	testQuery{
		"getFlavors",
		models.Query{
			models.Flavor{Vcpus: -1, RAM: -1, Disk: -1},
			nil,
		},
		[]models.Flavor{
			models.Flavor{
				Name:  "clmc",
				Vcpus: 4,
				RAM:   32768,
				Disk:  100,
			},
			models.Flavor{
				Name:  "frontend",
				Vcpus: 1,
				RAM:   1024,
				Disk:  5,
			},
			models.Flavor{
				Name:  "moose",
				Vcpus: 1,
				RAM:   1536,
				Disk:  15,
			},
			models.Flavor{
				Name:  "pce",
				Vcpus: 1,
				RAM:   1536,
				Disk:  15,
			},
			models.Flavor{
				Name:  "ps",
				Vcpus: 1,
				RAM:   1024,
				Disk:  100,
			},
			models.Flavor{
				Name:  "sfemc",
				Vcpus: 1,
				RAM:   1024,
				Disk:  10,
			},
			models.Flavor{
				Name:  "sr",
				Vcpus: 1,
				RAM:   1536,
				Disk:  10,
			},
		},
	},
	testQuery{
		"getSecurityGrpRules",
		models.Query{
			models.SecurityGrpRule{Port: -1},
			nil,
		},
		[]models.SecurityGrpRule{
			models.SecurityGrpRule{
				Name:     "clmc",
				Protocol: "tcp",
				Port:     80,
			},
			models.SecurityGrpRule{
				Name:     "mgmt",
				Protocol: "icmp",
				Port:     0,
			},
			models.SecurityGrpRule{
				Name:     "mgmt",
				Protocol: "tcp",
				Port:     22,
			},
			models.SecurityGrpRule{
				Name:     "mgmt",
				Protocol: "tcp",
				Port:     80,
			},
			models.SecurityGrpRule{
				Name:     "mgmt",
				Protocol: "tcp",
				Port:     8080,
			},
			models.SecurityGrpRule{
				Name:     "msp",
				Protocol: "tcp",
				Port:     80,
			},
			models.SecurityGrpRule{
				Name:     "msp",
				Protocol: "tcp",
				Port:     7687,
			},
			models.SecurityGrpRule{
				Name:     "msp",
				Protocol: "tcp",
				Port:     8080,
			},
			models.SecurityGrpRule{
				Name:     "ps",
				Protocol: "tcp",
				Port:     80,
			},
			models.SecurityGrpRule{
				Name:     "sdnctrl",
				Protocol: "tcp",
				Port:     2016,
			},
			models.SecurityGrpRule{
				Name:     "sdnctrl",
				Protocol: "tcp",
				Port:     2017,
			},
			models.SecurityGrpRule{
				Name:     "sdnctrl",
				Protocol: "tcp",
				Port:     6633,
			},
			models.SecurityGrpRule{
				Name:     "sdnctrl",
				Protocol: "tcp",
				Port:     6653,
			},
			models.SecurityGrpRule{
				Name:     "sdnctrl",
				Protocol: "tcp",
				Port:     8080,
			},
			models.SecurityGrpRule{
				Name:     "sia",
				Protocol: "tcp",
				Port:     80,
			},
			models.SecurityGrpRule{
				Name:     "wan",
				Protocol: "icmp",
				Port:     0,
			},
			models.SecurityGrpRule{
				Name:     "wan",
				Protocol: "tcp",
				Port:     22,
			},
		},
	},
}

func TestQueryGetEntities(t *testing.T) {
	if s == nil {
		t.Errorf("TestQueryGet: can't execute, because Storage pointer is nil")
		t.Failed()
		return
	}

	for _, tc := range testQueries {
		t.Run(tc.name, func(t *testing.T) {
			out, err := s.Get(&tc.in)
			if err != nil {
				t.Errorf("TestQueryGet: failed, because Get() failed")
				return
			}
			t.Logf("Expected: %v", tc.out)
			t.Logf("Actual: %v", out)

			//Compare input with output
			if !compare(tc.out, out) {
				t.Errorf("TestQueryGet: failed, because of mismatch in output")
				return
			}
		})
	}
}

var testRemoval = []testRemove{
	testRemove{
		name:          "removeRowsMatchingStringField",
		deleteAllRows: models.Compute{Vcpus: -1, RAM: -1, Disk: -1},
		insertRows: []models.Compute{
			models.Compute{
				Name:      "cn10",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     11,
				RAM:       25,
				Disk:      2068,
			},
			models.Compute{
				Name:      "cn11",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     12,
				RAM:       26,
				Disk:      2070,
			},
			models.Compute{
				Name:      "cn12",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     13,
				RAM:       27,
				Disk:      2072,
			},
			models.Compute{
				Name:      "cn13",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     14,
				RAM:       28,
				Disk:      2074,
			},
		},
		deleteRows: models.Compute{Name: "cn13", Vcpus: -1, RAM: -1, Disk: -1},
		getRows: models.Query{
			models.Compute{Vcpus: -1, RAM: -1, Disk: -1},
			nil,
		},
		out: []models.Compute{
			models.Compute{
				Name:      "cn10",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     11,
				RAM:       25,
				Disk:      2068,
			},
			models.Compute{
				Name:      "cn11",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     12,
				RAM:       26,
				Disk:      2070,
			},
			models.Compute{
				Name:      "cn12",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     13,
				RAM:       27,
				Disk:      2072,
			},
		},
	},
	testRemove{
		name:          "removeRowsMatchingIntField",
		deleteAllRows: models.Compute{Vcpus: -1, RAM: -1, Disk: -1},
		insertRows: []models.Compute{
			models.Compute{
				Name:      "cn10",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     11,
				RAM:       25,
				Disk:      2068,
			},
			models.Compute{
				Name:      "cn11",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     12,
				RAM:       26,
				Disk:      2070,
			},
			models.Compute{
				Name:      "cn12",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     12,
				RAM:       27,
				Disk:      2072,
			},
			models.Compute{
				Name:      "cn13",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     14,
				RAM:       28,
				Disk:      2074,
			},
		},
		deleteRows: models.Compute{Vcpus: 12, RAM: -1, Disk: -1},
		getRows: models.Query{
			models.Compute{Vcpus: -1, RAM: -1, Disk: -1},
			nil,
		},
		out: []models.Compute{
			models.Compute{
				Name:      "cn10",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     11,
				RAM:       25,
				Disk:      2068,
			},
			models.Compute{
				Name:      "cn13",
				AvailZone: "nova",
				Tier:      "edge",
				Vcpus:     14,
				RAM:       28,
				Disk:      2074,
			},
		},
	},
}

func TestRemoveEntities(t *testing.T) {
	if s == nil {
		t.Errorf("TestRemoveEntities(): can't execute, because Storage pointer is nil")
		t.Failed()
		return
	}

	for _, tc := range testRemoval {
		t.Run(tc.name, func(t *testing.T) {
			// Delete all rows from table in consideration
			err := s.Remove(tc.deleteAllRows)
			if err != nil {
				t.Errorf("TestRemoveEntities(): FAILED, because Remove() FAILED")
				return
			}

			// Insert rows into the table
			insertInterface := make([]interface{}, 0)
			insertInterface = append(insertInterface, tc.insertRows)
			err = s.Add(insertInterface)
			if err != nil {
				t.Errorf("TestAddEntities(): FAILED, because Add() FAILED")
				return
			}

			// Delete row(s) from the table on the basis of condition(s) if any
			err = s.Remove(tc.deleteRows)
			if err != nil {
				t.Errorf("TestRemoveEntities(): FAILED, because Remove() FAILED")
				return
			}

			// Get rows from the table after deletion
			out, errGet := s.Get(&tc.getRows)
			if errGet != nil {
				t.Errorf("TestQueryGet(): FAILED, because Get() FAILED")
				return
			}
			t.Logf("Desired Rows in the table after deletion: %v", tc.out)
			t.Logf("Actual Rows in the table after deletion: %v", out)

			// Compare desired output with actual output after deletion
			if !compare(tc.out, out) {
				t.Errorf("TestRemoveEntities(): FAILED, because of mismatch in output")
				return
			}
		})
	}
}

func compare(in interface{}, out interface{}) bool {
	return reflect.DeepEqual(in, out)
}

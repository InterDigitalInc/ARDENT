package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/hot"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/infra"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/sanity"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/stack"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/http/rest"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/orchestrator"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/storage/mysql"
)

type TestOutput struct {
	HttpCode          int    `json:"httpCode"`
	WriteSanityResult bool   `json:"writeSanityResult"`
	StatusId          int    `json:"statusId"`
	StatusStr         string `json:"statusStr"`
	HeatYml           string `json:"heatYml"`
	CompareResponse   bool   `json:"compareResponse"`
}

type Test struct {
	TestName string
	Input    string
	Output   TestOutput
}

type TestParams struct {
	TestName           string     `json:"testName"`
	EndPoint           string     `json:"endPoint"`
	Method             string     `json:"method"`
	Input              string     `json:"input"`
	RemoveSanityResult bool       `json:"removeSanityResult"`
	Output             TestOutput `json:"output"`
	TestSleep          string     `json:"testSleep"`
}

// struct for Sanity Warning
type Warn struct {
	Category    string `json:"category"`
	Description string `json:"description"`
}

// struct for Sanity Result
type Res struct {
	Id  models.StatusId  `json:"status_id"`
	Msg models.StatusMsg `json:"status_str"`
}

// Sanity Result will contain all the Sanity Warnings
type SanityResult struct {
	Result  *Res
	Warning []*Warn
}

const (
	sanityResultFile   = "sanity-result"
	defaultHeatDirPath = "../../heat"
	adminOpenRC        = "admin-openrc"
	tenantOpenRC       = "tenant-openrc"
)

var (
	router *mux.Router
	logger *logrus.Logger
	sLocal *mysql.Storage

	descDefFilePath = "../../util/infra-descriptor-definitions.yml"

	// Pass following as command line arguments:
	// mysqlusr
	// mysqlpwd
	// mysqlserverip
	// dbname
	// e.g.: CGO_ENABLED=0 go test ./... -v -cover -mysqlusr=user -mysqlpwd=password mysqlserverip=192.168.121.12 dbname=ardent_test
	mysqlUsr      = flag.String("mysqlusr", "", "MySQL Username")
	mysqlPwd      = flag.String("mysqlpwd", "", "MySQL Password")
	mysqlServerIp = flag.String("mysqlserverip", "", "MySQL Server IP")
	storeName     = flag.String("dbname", "", "Database Name")
	heatDirPath   = flag.String("heatdir", defaultHeatDirPath, "heat directory path")

	orchGetStackList = orchestrator.GetStackList
)

func TestHandler(t *testing.T) {

	if *mysqlUsr == "" || *mysqlPwd == "" || *mysqlServerIp == "" || *storeName == "" {
		log.Fatalf("\nUsage: CGO_ENABLED=0 go test ./... -v -cover -mysqlusr=<required> -mysqlpwd=<required> " +
			"-mysqlserverip=<required> -dbname=<required> -heatDirPath=<optional>")
		os.Exit(1)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	s, err := mysql.NewStorage(logger, *mysqlUsr, *mysqlPwd, *mysqlServerIp, *storeName)
	if err != nil {
		log.Fatalf("Error in initializing Store. %v", err)
		os.Exit(1)
	}
	sLocal = s

	sn, err := sanity.NewService(logger, s, *heatDirPath)
	if err != nil {
		log.Fatalf("Error in initializing Sanity Service . %v", err)
		os.Exit(1)
	}

	ht, err := hot.NewService(logger, s, sn, "../../heat")
	if err != nil {
		log.Fatalf("Error in initializing Hot Service. %v", err)
		os.Exit(1)
	}

	st, err := stack.NewService(logger, s)
	if err != nil {
		log.Fatalf("Error in initializing Stack service. %v", err)
		os.Exit(1)
	}

	in, err := infra.NewService(logger, s, ht, sn, st, descDefFilePath)
	if err != nil {
		log.Fatalf("Error in initializing Infra Service . %v", err)
		os.Exit(1)
	}

	err = orchestrator.Intialize(logger)
	if err != nil {
		log.Fatalf("Error in initializing Orchestrator. %v", err)
		os.Exit(1)
	}

	r, err := rest.Handler(logger, in, ht, st, sn)
	if err != nil {
		log.Fatalf("Error in initializing HTTP REST Service. %v", err)
		os.Exit(1)
	}

	if r == nil {
		t.Errorf("Router has no request to handle")
		return
	}

	router = r
}

func TestOpenRC(t *testing.T) {

	// Read test cases from file and run one by one
	file, err := os.Open("testcases/openRC.json")
	if err != nil {
		t.Errorf("Couldn't run test. Error in opening 'openRC.json' file. %v", err)
		return
	}
	defer file.Close()
	fileBytes, err := ioutil.ReadAll(file)

	tests := make([]TestParams, 0)
	err = json.Unmarshal(fileBytes, &tests)
	if err != nil {
		t.Errorf("Error in unmarshalling openRC. %v", err)
	}

	for _, test := range tests {
		t.Run(test.TestName, func(t *testing.T) {
			var req *http.Request

			if test.Input == "" {
				r, err := http.NewRequest(test.Method, test.EndPoint, nil)
				req = r
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
			} else {
				file, err := os.Open(test.Input)
				if err != nil {
					t.Errorf("Couldn't run test. Error in opening %s file. %v", test.Input, err)
					return
				}
				defer file.Close()
				fileBytes, err := ioutil.ReadAll(file)

				r, err := http.NewRequest(test.Method, test.EndPoint, bytes.NewBuffer(fileBytes))
				req = r
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
			}

			body := new(bytes.Buffer)
			writer := multipart.NewWriter(body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != test.Output.HttpCode {
				t.Errorf("handler returned wrong status code: received: %v expected: %v",
					w.Code, test.Output.HttpCode)
				return
			}

			received := strings.TrimSuffix(w.Body.String(), "\n")
			expected := `{"status_id":` + strconv.Itoa(test.Output.StatusId) + `,"status_str":"` +
				test.Output.StatusStr + `"}`

			if received != expected {
				t.Errorf("handler returned unexpected body: received: %v expected: %v",
					received, expected)
				return
			}

			if test.Output.CompareResponse == true {
				initialFile, err := os.Open(test.Input)
				if err != nil {
					t.Errorf("Couldn't run test. Error in opening '%s' file. %v", test.Input, err)
					return
				}
				defer initialFile.Close()

				initialFileBytes, err := ioutil.ReadAll(initialFile)
				if err != nil {
					t.Errorf("Error in reading '%s' as byte array. %v", test.Input, err)
				}

				if strings.Contains(test.TestName, "AdminRC") {
					adminFile, err := os.Open(adminOpenRC)
					if err != nil {
						t.Errorf("Couldn't run test. Error in opening '%s' file. %v", adminOpenRC, err)
						return
					}
					defer adminFile.Close()

					adminFileBytes, err := ioutil.ReadAll(adminFile)
					if err != nil {
						t.Errorf("Error in reading '%s' as byte array. %v", adminOpenRC, err)
					}

					if !compare(initialFileBytes, adminFileBytes) {
						t.Errorf("Existing & posted Admin OpenRC do not match")
						return
					}
					t.Log("Existing & posted Admin OpenRC match")
				} else {
					tenantFile, err := os.Open(tenantOpenRC)
					if err != nil {
						t.Errorf("Couldn't run test. Error in opening '%s' file. %v", tenantOpenRC, err)
						return
					}
					defer tenantFile.Close()

					tenantFileBytes, err := ioutil.ReadAll(tenantFile)
					if err != nil {
						t.Errorf("Error in reading '%s' as byte array. %v", tenantOpenRC, err)
					}

					if !compare(initialFileBytes, tenantFileBytes) {
						t.Errorf("Existing & posted Tenant OpenRC do not match")
						return
					}
					t.Log("Existing & posted Tenant OpenRC match")
				}
			}
		})
	}
}

func TestInfraDescriptor(t *testing.T) {

	// Read test cases from file and run one by one
	file, err := os.Open("testcases/infraDescriptor.json")
	if err != nil {
		t.Errorf("Couldn't run test. Error in opening 'infraDescriptor.json' file. %v", err)
		return
	}
	defer file.Close()
	fileBytes, err := ioutil.ReadAll(file)

	tests := make([]TestParams, 0)
	json.Unmarshal(fileBytes, &tests)

	for _, test := range tests {
		t.Run(test.TestName, func(t *testing.T) {
			var req *http.Request

			if test.Input == "" {
				r, err := http.NewRequest(test.Method, test.EndPoint, nil)
				req = r
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
			} else {
				file, err := os.Open("testdata/" + test.Input)
				if err != nil {
					t.Errorf("Couldn't run test. Error in opening %s file. %v", test.Input, err)
					return
				}
				defer file.Close()
				fileBytes, err := ioutil.ReadAll(file)

				r, err := http.NewRequest(test.Method, test.EndPoint, bytes.NewBuffer(fileBytes))
				req = r
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
			}

			body := new(bytes.Buffer)
			writer := multipart.NewWriter(body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			var httpCodePointer *int
			httpCodePointer = &test.Output.HttpCode
			if *httpCodePointer != 0 {
				if w.Code != test.Output.HttpCode {
					t.Errorf("handler returned wrong status code: received: %v expected: %v",
						w.Code, test.Output.HttpCode)
					return
				}

				var statusIdPointer *int
				var received, expected string
				statusIdPointer = &test.Output.StatusId
				if statusIdPointer != nil && test.Output.StatusStr != "" {
					if test.Output.CompareResponse == true {
						received = strings.TrimSuffix(w.Body.String(), "\n")
						expected = test.Output.StatusStr
					} else {
						received = strings.TrimSuffix(w.Body.String(), "\n")
						expected = `{"status_id":` + strconv.Itoa(test.Output.StatusId) + `,"status_str":"` +
							test.Output.StatusStr + `"}`
					}

					if received != expected {
						t.Errorf("handler returned unexpected body: received: %v expected: %v",
							received, expected)
						return
					}
				} else {
					if test.TestSleep == "yes" {
						//Check sanity-result file's existence
						for {
							r, err := http.NewRequest("GET", "/sanity-check/status", nil)
							if err != nil {
								t.Errorf("Error in creating HTTP Request for getting sanity-check status. %v", err)
								return
							}

							w = httptest.NewRecorder()
							router.ServeHTTP(w, r)

							received = strings.TrimSuffix(w.Body.String(), "\n")
							if received != "{\"status_id\":0,\"status_str\":\"Sanity-Check completed\"}" {
								time.Sleep(10 * time.Second)
							} else {
								break
							}
						}
					}
				}

				var computeNodesAfterAdd interface{}

				if test.TestName == "Add Infra Descriptor" {
					// Verify from Storage
					computeNode := models.Compute{Vcpus: -1, RAM: -1, Disk: -1}
					query := models.Query{Entity: computeNode}
					computeNodesAfterAdd, err = sLocal.Get(&query)
					if err != nil {
						t.Errorf("Error in retrieving Compute Nodes from Storage. %v", err)
						return
					}
				}

				var computeNodesAfterUpdate interface{}

				if test.TestName == "Add Infra Descriptor again" {
					// Verify from Storage
					computeNode := models.Compute{Vcpus: -1, RAM: -1, Disk: -1}
					query := models.Query{Entity: computeNode}
					computeNodesAfterUpdate, err = sLocal.Get(&query)
					if err != nil {
						t.Errorf("Error in retrieving Compute Nodes from Storage. %v", err)
						return
					}

					// Displaying compute nodes before and after adding
					// infra descriptor again
					t.Logf("Compute nodes before adding infra descriptor again: %v", computeNodesAfterAdd)
					t.Logf("Compute nodes before adding infra descriptor again: %v", computeNodesAfterUpdate)

					// Check if Storage is updated
					if compare(computeNodesAfterAdd, computeNodesAfterUpdate) {
						t.Errorf("Storage is not updated with new rows")
						return
					}
				}

				if test.TestName == "Delete Infra Descriptor without Payload" {
					// Check for Cluster-Flavors' presence in Storage
					flavor := models.Flavor{Vcpus: -1, RAM: -1, Disk: -1}
					query := models.Query{Entity: flavor}
					flavors, err := sLocal.Get(&query)
					if err != nil {
						t.Errorf("Error in retrieving Flavors from Storage. %v", err)
						return
					}
					t.Logf("Flavors from Storage: %v", flavors)

					clusterFlavorFound := false
					flavorsArr := flavors.([]models.Flavor)
					for i, _ := range flavorsArr {
						if strings.HasPrefix(flavorsArr[i].Name, "flame-cluster") {
							clusterFlavorFound = true
							break
						}
					}
					if clusterFlavorFound == true {
						t.Errorf("Error: Cluster Flavors found in Storage")
						return
					}
				}
			}
		})
	}
}

func TestInfraDescriptorToStorage(t *testing.T) {

	// Deleting Infra Descriptor from storage
	r, err := http.NewRequest("DELETE", "/infra/descriptor", nil)
	if err != nil {
		t.Errorf("Error in creating HTTP Request for deleting Infra Descritpor. %v", err)
		return
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handler returned unexpected Status Code: Received. %v, Expected. %v",
			w.Code, http.StatusOK)
		return
	}

	// Read test cases from file and run one by one
	file, err := os.Open("testcases/infraToStorage.json")
	if err != nil {
		t.Errorf("Couldn't run test. Error in opening 'infraToStorage.json' file. %v", err)
		return
	}
	defer file.Close()
	fileBytes, err := ioutil.ReadAll(file)

	tests := make([]Test, 0)
	json.Unmarshal(fileBytes, &tests)

	for _, test := range tests {

		t.Run(test.TestName, func(t *testing.T) {

			file, err := os.Open("testdata/" + test.Input)
			if err != nil {
				t.Errorf("Couldn't run test. Error in opening %s file. %v", test.Input, err)
				return
			}
			defer file.Close()
			fileBytes, err := ioutil.ReadAll(file)

			if test.TestName == "Delete Infra Descriptor" {

				// Deleting Infra Descriptor from storage
				r, err := http.NewRequest("DELETE", "/infra/descriptor", nil)
				if err != nil {
					t.Errorf("Error in creating HTTP Request for deleting Infra Descritpor. %v", err)
					return
				}

				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)

				if w.Code != test.Output.HttpCode {
					t.Errorf("Handler returned unexpected Status Code: Received. %v, Expected. %v",
						w.Code, test.Output.HttpCode)
					return
				}
			} else {
				r, err := http.NewRequest("PUT", "/infra/descriptor", bytes.NewBuffer(fileBytes))
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				r.Header.Set("Content-Type", writer.FormDataContentType())

				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)

				if w.Code != test.Output.HttpCode {
					t.Errorf("handler returned wrong status code: received: %v expected: %v",
						w.Code, test.Output.HttpCode)
					return
				}
				received := strings.TrimSuffix(w.Body.String(), "\n")
				expected := `{"status_id":` + strconv.Itoa(test.Output.StatusId) + `,"status_str":"` +
					test.Output.StatusStr + `"}`

				if received != expected {
					t.Errorf("handler returned unexpected body: received: %v expected: %v",
						received, expected)
					return
				}
			}
		})
	}
}

func TestStorageToHeatTemplate(t *testing.T) {

	files, err := ioutil.ReadDir("./")
	if err != nil {
		t.Errorf("Error in getting list of files. %v", err)
	}
	for _, f := range files {
		if strings.Contains(f.Name(), "actual-map") ||
			strings.Contains(f.Name(), "expected-map") {
			err := os.Remove(f.Name())
			if err != nil {
				t.Errorf("Error in deleting existing Hot map. %v", err)
			}
		}
	}

	// Read test cases from file and run one by one
	file, err := os.Open("testcases/storageToHeatTemplate.json")
	if err != nil {
		t.Errorf("Couldn't run test. Error in opening 'storageToHeatTemplate.json' file. %v", err)
		return
	}
	defer file.Close()
	fileBytes, err := ioutil.ReadAll(file)

	tests := make([]Test, 0)
	json.Unmarshal(fileBytes, &tests)

	for _, test := range tests {

		t.Run(test.TestName, func(t *testing.T) {

			// Delete Infra Descriptor from storage
			r, err := http.NewRequest("DELETE", "/infra/descriptor", nil)
			if err != nil {
				t.Errorf("Error in creating HTTP Request for deleting Infra Descritpor. %v", err)
				return
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("Handler returned unexpected Status Code: Received. %v, Expected. %v",
					w.Code, http.StatusOK)
				return
			}

			// Delete 'sanity-result' file
			if _, err := os.Stat(sanityResultFile); !os.IsNotExist(err) {
				fileDeleteErr := os.Remove(sanityResultFile)
				if fileDeleteErr != nil {
					t.Errorf("Error in removing %s file. %v", sanityResultFile, fileDeleteErr)
					return
				}
			}

			// Check for Cluster-Flavors' presence in Storage
			flavor := models.Flavor{Vcpus: -1, RAM: -1, Disk: -1}
			query := models.Query{Entity: flavor}
			flavors, err := sLocal.Get(&query)
			if err != nil {
				t.Errorf("Error in retrieving Flavors from Storage. %v", err)
				return
			}
			t.Logf("Flavors from Storage: %v", flavors)

			clusterFlavorFound := false
			flavorsArr := flavors.([]models.Flavor)
			for i, _ := range flavorsArr {
				if strings.HasPrefix(flavorsArr[i].Name, "flame-cluster") {
					clusterFlavorFound = true
					break
				}
			}
			if clusterFlavorFound == true {
				t.Errorf("Error: Cluster Flavors found in Storage")
				return
			}

			// Add Infra Descriptor
			file, err := os.Open("testdata/" + test.Input)
			if err != nil {
				t.Errorf("Couldn't run test. Error in opening %s file. %v", test.Input, err)
				return
			}
			defer file.Close()

			fileBytes, err := ioutil.ReadAll(file)
			r, err = http.NewRequest("PUT", "/infra/descriptor", bytes.NewBuffer(fileBytes))
			if err != nil {
				t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
				return
			}

			w = httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("handler returned unexpected status code: received: %v expected: %v",
					w.Code, http.StatusOK)
				return
			}
			received := strings.TrimSuffix(w.Body.String(), "\n")
			expected := `{"status_id":` + strconv.Itoa(test.Output.StatusId) + `,"status_str":"` +
				test.Output.StatusStr + `"}`

			if received != expected {
				t.Errorf("handler returned unexpected body: received: %v expected: %v",
					received, expected)
				return
			}

			// Delete Tenant OpenRC
			if _, err := os.Stat(tenantOpenRC); err == nil {
				r, err := http.NewRequest("DELETE", "/infra/rc/tenant", nil)
				if err != nil {
					t.Errorf("Error in creating HTTP Request for deleting Tenant OpenRC. %v", err)
				}

				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
			}

			// Add Tenant OpenRC
			filePath := "testdata/" + tenantOpenRC + "-final"
			file, err = os.Open(filePath)
			if err != nil {
				t.Errorf("Couldn't run test. Error in opening '%s' file. %v", tenantOpenRC+"-final", err)
				return
			}
			defer file.Close()
			fileBytes, err = ioutil.ReadAll(file)

			r, err = http.NewRequest("PUT", "/infra/rc/tenant", bytes.NewBuffer(fileBytes))
			if err != nil {
				t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
				return
			}

			body := new(bytes.Buffer)
			writer := multipart.NewWriter(body)
			r.Header.Set("Content-Type", writer.FormDataContentType())

			w = httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("handler returned wrong status code: received: %v expected: %v",
					w.Code, http.StatusOK)
				return
			}
			received = strings.TrimSuffix(w.Body.String(), "\n")
			expected = `{"status_id":` + strconv.Itoa(test.Output.StatusId) + `,"status_str":"` +
				test.Output.StatusStr + `"}`

			if received != expected {
				t.Errorf("handler returned unexpected body: received: %v expected: %v",
					received, expected)
				return
			}

			// Write/Generate Sanity Result file
			if test.Output.WriteSanityResult == true {
				res := &SanityResult{}
				res.Warning = []*Warn{}

				sanityFile, err := os.OpenFile(sanityResultFile, os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					t.Errorf("Couldn't run test. Error in opening %s file. %v", sanityResultFile, err)
					return
				}

				res.Result = &Res{
					Id:  models.NO_ERR,
					Msg: models.StatusMsg("successful"),
				}

				// Write Sanity Result file
				encodeSanityResAndWriteToFile(res, sanityFile, t)
				sanityFile.Close()
			} else {
				// Initiate Sanity Check
				r, err = http.NewRequest("POST", "/sanity-check", nil)
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}

				w = httptest.NewRecorder()
				router.ServeHTTP(w, r)

				if w.Code != http.StatusAccepted {
					t.Errorf("Error in initiating Sanity Check: %v", strings.TrimSuffix(w.Body.String(), "\n"))
					return
				}

				//Check sanity-result file's existence
				for {
					r, err = http.NewRequest("GET", "/sanity-check/status", nil)
					if err != nil {
						t.Errorf("Error in creating HTTP Request for getting sanity-check status. %v", err)
						return
					}

					w = httptest.NewRecorder()
					router.ServeHTTP(w, r)
					if w.Code != http.StatusOK {
						t.Errorf("handler returned unexpected status code: received: %v expected: %v",
							w.Code, http.StatusOK)
						return
					}

					received = strings.TrimSuffix(w.Body.String(), "\n")
					if received != "{\"status_id\":0,\"status_str\":\"Sanity-Check completed\"}" {
						time.Sleep(10 * time.Second)
					} else {
						break
					}
				}

			}

			// Generate HEAT Template for comparison
			r, err = http.NewRequest("POST", "/hot/generate", nil)
			if err != nil {
				t.Errorf("Error in creating HTTP Request for generating Heat Template. %v", err)
				return
			}

			w = httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("handler returned unexpected status code: received: %v expected: %v",
					w.Code, http.StatusOK)
				return
			}

			received = strings.TrimSuffix(w.Body.String(), "\n")
			expected = `{"status_id":` + strconv.Itoa(test.Output.StatusId) + `,"status_str":"` +
				test.Output.StatusStr + `"}`

			if received != expected {
				t.Errorf("handler returned unexpected body: received: %v expected: %v",
					received, expected)
				return
			}

			// Check for Cluster-Flavors' presence in Storage
			flavor = models.Flavor{Vcpus: -1, RAM: -1, Disk: -1}
			query = models.Query{Entity: flavor}
			flavors, err = sLocal.Get(&query)
			if err != nil {
				t.Errorf("Error in retrieving Flavors from Storage. %v", err)
				return
			}
			t.Logf("Flavors from Storage: %v", flavors)
			clusterFlavorFound = false
			flavorsArr = flavors.([]models.Flavor)
			for i, _ := range flavorsArr {
				if strings.HasPrefix(flavorsArr[i].Name, "flame-cluster") {
					clusterFlavorFound = true
					break
				}
			}
			if clusterFlavorFound == false {
				t.Errorf("Error: Cluster Flavors not found in Storage")
				return
			}

			// Get 'stack-flame-platform.yaml' via /hot/descriptor GET method
			r, err = http.NewRequest("GET", "/hot/descriptor", nil)
			if err != nil {
				t.Errorf("Error in creating HTTP Request for getting HEAT template. %v", err)
				return
			}

			w = httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("Handler returned unexpected Status Code: Received. %v, Expected. %v",
					w.Code, http.StatusOK)
				return
			}

			heatTemplateBytes := []byte(w.Body.String())

			// Compare gotten HEAT template & generated HEAT template
			generatedHeatTemplate, err := os.Open("stack-flame-platform.yaml")
			if err != nil {
				t.Errorf("Couldn't run test. Error in opening 'stack-flame-platform.yaml' file. %v", err)
				return
			}
			defer generatedHeatTemplate.Close()

			generatedHeatTemplateBytes, err := ioutil.ReadAll(generatedHeatTemplate)
			if err != nil {
				t.Errorf("Error in reading 'stack-flame-platform.yaml' as byte array. %v", err)
			}

			if compare(heatTemplateBytes, generatedHeatTemplateBytes) {
				t.Log("Generated HEAT template & gotten HEAT template match")
			} else {
				t.Errorf("Generated HEAT template & gotten HEAT template do not match")
				return
			}

			// Compare Generated HEAT Template with HEAT Template
			generatedHeatMap, err := processHeatTemplateToMap("stack-flame-platform.yaml", t)
			if err != nil {
				t.Errorf("Error in reading Generated HEAT Template to a map. %v", err)
				return
			}
			if len(generatedHeatMap) == 0 {
				t.Errorf("Failure: generatedHeatMap is empty")
				return
			}

			removeKeysFromMap(&generatedHeatMap, t)

			outputHeatMap, err := processHeatTemplateToMap("testresults/"+test.Output.HeatYml, t)
			if err != nil {
				t.Errorf("Error in reading  HEAT Template to a map. %v", err)
				return
			}
			if len(outputHeatMap) == 0 {
				t.Errorf("Failure: outputHeatMap is empty")
				return
			}

			removeKeysFromMap(&outputHeatMap, t)

			if generatedHeatMap == nil || outputHeatMap == nil {
				t.Logf("node-passwd cannot be nil")
				t.Errorf("Cannot proceed to compare Generated HEAT Template & HEAT Template")
				return
			}

			_, err = validateLANParams(&generatedHeatMap, &outputHeatMap, t)
			if err != nil {
				t.Errorf("Failed to retrieve LAN IPs from Generated HEAT Template. %v", err)
				return
			}

			t.Logf("Generated HEAT Map: %v\n", generatedHeatMap)
			t.Logf("Output HEAT Map: %v\n", outputHeatMap)

			if compare(generatedHeatMap, outputHeatMap) {
				t.Log("Success: Generated HEAT Template & Output HEAT Template match")
			} else {
				t.Errorf("Failure: Generated HEAT Template & Output HEAT Template do not match")

				generatedHeatMapBytes := new(bytes.Buffer)
				for key, value := range generatedHeatMap {
					fmt.Fprintf(generatedHeatMapBytes, "%s=%s\n", key, value)
				}
				ioutil.WriteFile("actual-map-"+test.Output.HeatYml, generatedHeatMapBytes.Bytes(), 0)

				outputHeatMapBytes := new(bytes.Buffer)
				for key, value := range outputHeatMap {
					fmt.Fprintf(outputHeatMapBytes, "%s=%s\n", key, value)
				}
				ioutil.WriteFile("expected-map-"+test.Output.HeatYml, outputHeatMapBytes.Bytes(), 0)
			}

			// Delete 'stack-flame-platform.yaml' via /hot/descriptor DELETE method
			r, err = http.NewRequest("DELETE", "/hot/descriptor", nil)
			if err != nil {
				t.Errorf("Error in creating HTTP Request for deleting generated HEAT template. %v", err)
				return
			}

			w = httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("Handler returned unexpected Status Code: Received. %v, Expected. %v",
					w.Code, http.StatusOK)
				return
			}

			// Check HEAT template's existence
			if _, err := os.Stat("stack-flame-platform.yaml"); os.IsNotExist(err) {
				t.Log("Successfully removed 'stack-flame-platform.yaml'")
			} else {
				t.Errorf("Error in removing 'stack-flame-platform.yaml'. %v", err)
				return
			}

		})
	}
}

func TestHeatTemplateToLaunchStack(t *testing.T) {

	// Read test cases from file and run one by one
	file, err := os.Open("testcases/heatTemplateToLaunchStack.json")
	if err != nil {
		t.Errorf("Couldn't run test. Error in opening 'heatTemplateToLaunchStack.json' file. %v", err)
		return
	}
	defer file.Close()
	fileBytes, err := ioutil.ReadAll(file)

	tests := make([]TestParams, 0)
	json.Unmarshal(fileBytes, &tests)

	for _, test := range tests {
		t.Run(test.TestName, func(t *testing.T) {
			if test.TestName == "Launch stack with invalid heat template" {
				//Create invalid HEAT template
				from, err := os.Open("testdata/stack-flame-platform-invalid.yaml")
				if err != nil {
					t.Errorf("Error in opening 'stack-flame-platform-invalid.yaml' file. %v", err)
				}
				defer from.Close()

				to, err := os.OpenFile("stack-flame-platform.yaml", os.O_RDWR|os.O_CREATE, 0666)
				if err != nil {
					t.Errorf("Error in creating invalid 'stack-flame-platform.yaml' file. %v", err)
				}
				defer to.Close()

				_, err = io.Copy(to, from)
				if err != nil {
					t.Errorf("Error in copying content to invalid HEAT template. %v", err)
				}
			} else {
				if test.TestName == "Launch stack without heat template" {
					fileDeleteErr := os.Remove("stack-flame-platform.yaml")
					if fileDeleteErr != nil {
						t.Errorf("Error in removing 'stack-flame-platform.yaml' file. %v", fileDeleteErr)
						return
					}
				} else {
					//Create invalid HEAT template
					from, err := os.Open("testdata/stack-flame-platform-valid.yaml")
					if err != nil {
						t.Errorf("Error in opening 'stack-flame-platform-valid.yaml' file. %v", err)
					}
					defer from.Close()

					to, err := os.OpenFile("stack-flame-platform.yaml", os.O_RDWR|os.O_CREATE, 0666)
					if err != nil {
						t.Errorf("Error in creating invalid 'stack-flame-platform.yaml' file. %v", err)
					}
					defer to.Close()

					_, err = io.Copy(to, from)
					if err != nil {
						t.Errorf("Error in copying content to valid HEAT template. %v", err)
					}
				}
			}

			//Remove tenant-openrc file
			if test.RemoveSanityResult == true {
				if _, err := os.Stat(tenantOpenRC); !os.IsNotExist(err) {
					// Delete tenant-openrc file
					fileDeleteErr := os.Remove(tenantOpenRC)
					if fileDeleteErr != nil {
						t.Errorf("Error in removing '%s' file. %v", tenantOpenRC, fileDeleteErr)
						return
					}
				}
			}

			var req *http.Request

			if test.Input != "" {
				r, err := http.NewRequest(test.Method, test.EndPoint, bytes.NewBuffer([]byte(test.Input)))
				req = r
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
			} else {
				r, err := http.NewRequest(test.Method, test.EndPoint, nil)
				req = r
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
			}

			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != test.Output.HttpCode {
				t.Errorf("handler returned unexpected status code: received: %v expected: %v",
					w.Code, test.Output.HttpCode)
				return
			}

			var statusIdPointer *int
			statusIdPointer = &test.Output.StatusId
			if statusIdPointer != nil && test.Output.StatusStr != "" {

				received := strings.TrimSuffix(w.Body.String(), "\n")
				expected := `{"status_id":` + strconv.Itoa(test.Output.StatusId) + `,"status_str":"` +
					test.Output.StatusStr + `"}`

				if received != expected {
					t.Errorf("handler returned unexpected body: received: %v expected: %v",
						received, expected)
					return
				}
			}

			//Check for 'tenant-openrc' file
			//If absent, create it
			if _, err := os.Stat(tenantOpenRC); os.IsNotExist(err) {
				filePath := "testdata/" + tenantOpenRC + "-final"
				from, err := os.Open(filePath)
				if err != nil {
					t.Errorf("Error in opening '%s' file. %v", tenantOpenRC+"-final", err)
				}
				defer from.Close()

				to, err := os.OpenFile(tenantOpenRC, os.O_RDWR|os.O_CREATE, 0666)
				if err != nil {
					t.Errorf("Error in creating '%s' file. %v", tenantOpenRC, err)
				}
				defer to.Close()

				_, err = io.Copy(to, from)
				if err != nil {
					t.Errorf("Error in copying content to '%s' file. %v", tenantOpenRC, err)
				}
			}
		})
	}
}

func TestHotGenerateWithoutInfraDescriptor(t *testing.T) {

	// Delete Infra Descriptor from Storage
	r, err := http.NewRequest("DELETE", "/infra/descriptor", nil)
	if err != nil {
		t.Errorf("Error in creating HTTP Request for deleting Infra Descritpor. %v", err)
		return
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handler returned unexpected Status Code: Received. %v, Expected. %v",
			w.Code, http.StatusOK)
		return
	}

	res := &SanityResult{}
	res.Warning = []*Warn{}

	sanityFile, err := os.OpenFile(sanityResultFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("Couldn't run test. Error in opening %s file. %v", sanityResultFile, err)
		return
	}

	res.Result = &Res{
		Id:  models.NO_ERR,
		Msg: models.StatusMsg("successful"),
	}

	// Write Sanity Result file
	encodeSanityResAndWriteToFile(res, sanityFile, t)
	sanityFile.Close()

	// Generate HEAT Template for comparison
	r, err = http.NewRequest("POST", "/hot/generate", nil)
	if err != nil {
		t.Errorf("Error in creating HTTP Request for generating Heat Template. %v", err)
		return
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("handler returned unexpected status code: received: %v expected: %v",
			w.Code, http.StatusForbidden)
		return
	}

	received := strings.TrimSuffix(w.Body.String(), "\n")
	expected := `{"status_id":` + strconv.Itoa(301) + `,"status_str":"` +
		"Incomplete infra descriptor found: compute-nodes are not present" + `"}`

	if received != expected {
		t.Errorf("handler returned unexpected body: received: %v expected: %v",
			received, expected)
		return
	}
}

func TestSanityCheck(t *testing.T) {

	// Read test cases from file and run one by one
	file, err := os.Open("testcases/sanityCheck.json")
	if err != nil {
		t.Errorf("Couldn't run test. Error in opening 'sanityCheck.json' file. %v", err)
		return
	}
	defer file.Close()
	fileBytes, err := ioutil.ReadAll(file)

	tests := make([]TestParams, 0)
	json.Unmarshal(fileBytes, &tests)

	for _, test := range tests {
		if test.RemoveSanityResult == true {
			if _, err := os.Stat(sanityResultFile); !os.IsNotExist(err) {
				// Delete 'sanity-result' file
				fileDeleteErr := os.Remove(sanityResultFile)
				if fileDeleteErr != nil {
					t.Errorf("Error in removing %s. %v", sanityResultFile, fileDeleteErr)
					return
				}
			}
		}

		if test.TestName == "Get Sanity Check Status after initiating Sanity Check" ||
			test.TestName == "Get Sanity Check Status after initiating Sanity Check without TenantRC" {
			//Check sanity-result file's existence
			for {
				r, err := http.NewRequest("GET", "/sanity-check/status", nil)
				if err != nil {
					t.Errorf("Error in creating HTTP Request for getting sanity-check status. %v", err)
					return
				}

				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				if w.Code != http.StatusOK {
					t.Errorf("handler returned unexpected status code: received: %v expected: %v",
						w.Code, http.StatusOK)
					return
				}

				received := strings.TrimSuffix(w.Body.String(), "\n")
				if received != "{\"status_id\":0,\"status_str\":\"Sanity-Check completed\"}" &&
					received != "{\"status_id\":452,\"status_str\":\"Sanity-Check failed\"}" {
					time.Sleep(10 * time.Second)
				} else {
					break
				}
			}
		}

		t.Run(test.TestName, func(t *testing.T) {
			// Write/Generate Sanity Result file
			if test.Output.WriteSanityResult == true {
				sanityFile, err := os.OpenFile(sanityResultFile, os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					t.Errorf("Couldn't run test. Error in opening %s file. %v", sanityResultFile, err)
					return
				}

				_, err = sanityFile.WriteString("{\"Result\": {\"status_id\": 0, \"status_str\": \"successful\" \"Warning\": []}")
				if err != nil {
					t.Errorf("Error in writing invalid Sanity Result file: %v. ", err)
					sanityFile.Close()
					return
				}
			}

			var req *http.Request

			if test.Input == "" {
				r, err := http.NewRequest(test.Method, test.EndPoint, nil)
				req = r
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
			} else {
				file, err := os.Open("testdata/" + test.Input)
				if err != nil {
					t.Errorf("Couldn't run test. Error in opening %s file. %v", test.Input, err)
					return
				}
				defer file.Close()
				fileBytes, err := ioutil.ReadAll(file)

				r, err := http.NewRequest(test.Method, test.EndPoint, bytes.NewBuffer(fileBytes))
				req = r
				if err != nil {
					t.Errorf("Couldn't run test. Error in creating HTTP Request. %v", err)
					return
				}
			}

			body := new(bytes.Buffer)
			writer := multipart.NewWriter(body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != test.Output.HttpCode {
				t.Errorf("handler returned wrong status code: received: %v expected: %v",
					w.Code, test.Output.HttpCode)
				return
			}

			var statusIdPointer *int
			var received, expected string
			statusIdPointer = &test.Output.StatusId
			if statusIdPointer != nil && test.Output.StatusStr != "" {
				if test.Output.CompareResponse == true {
					received = strings.TrimSuffix(w.Body.String(), "\n")
					expected = test.Output.StatusStr
				} else {
					received = strings.TrimSuffix(w.Body.String(), "\n")
					expected = `{"status_id":` + strconv.Itoa(test.Output.StatusId) + `,"status_str":"` +
						test.Output.StatusStr + `"}`
				}

				if received != expected {
					t.Errorf("handler returned unexpected body: received: %v expected: %v",
						received, expected)
					return
				}
			} else if statusIdPointer != nil && test.Output.StatusStr == "" {
				received := strings.TrimSuffix(w.Body.String(), "\n")
				if received != "" {
					t.Logf("Response body: %v", received)
				}
			}

			if test.TestSleep == "yes" {
				//Check sanity-result file's existence
				for {
					r, err := http.NewRequest("GET", "/sanity-check/status", nil)
					if err != nil {
						t.Errorf("Error in creating HTTP Request for getting sanity-check status. %v", err)
						return
					}

					w = httptest.NewRecorder()
					router.ServeHTTP(w, r)
					if w.Code != http.StatusOK {
						t.Errorf("handler returned unexpected status code: received: %v expected: %v",
							w.Code, http.StatusOK)
						return
					}

					received = strings.TrimSuffix(w.Body.String(), "\n")
					if received != "{\"status_id\":0,\"status_str\":\"Sanity-Check completed\"}" {
						time.Sleep(10 * time.Second)
					} else {
						break
					}
				}
			}

			// move sanity-result file to sanity-result-case
			if strings.Contains(test.TestName, "Get Sanity Check Results for") {
				sanityResultFileRename := "sanity-result-" + strings.Split(test.TestName, "for ")[1]
				sanityResultFileRename = strings.Replace(sanityResultFileRename, " ", "-", -1)
				err := os.Rename(sanityResultFile, sanityResultFileRename)
				if err != nil {
					t.Errorf("Error in renaming %s file. %v", sanityResultFile, err)
				}
			}
		})
	}
}

func encodeSanityResAndWriteToFile(r *SanityResult, file *os.File, t *testing.T) {

	// encoding sanity result to json
	//resJson, err := json.Marshal(r)
	resJson, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		t.Errorf("Error in marshalling sanity result in json. %v", err)
	}

	// writting encoded result in file
	_, err = file.WriteString(string(resJson))
	if err != nil {
		t.Errorf("Error in writing sanity result in file. %v", err)
	}
}

func processHeatTemplateToMap(file string, t *testing.T) (map[string]interface{}, error) {

	fileMap := make(map[string]interface{})

	if _, err := os.Stat(file); os.IsNotExist(err) {
		t.Errorf("Error in getting '%s' file stat. %v", file, err)
		return nil, errors.New(err.Error())
	}
	fileBytes, err := ioutil.ReadFile(file)
	if err != nil {
		t.Errorf("Error in reading '%s' file. %v", file, err)
		return nil, errors.New(err.Error())
	}
	err = yaml.UnmarshalStrict(fileBytes, &fileMap)
	if err != nil {
		t.Errorf("Error in unmarshaling '%s'. %v", file, err)
		return nil, errors.New(err.Error())
	}
	return fileMap, nil
}

func removeKeysFromMap(fileMap *map[string]interface{}, t *testing.T) {

	for k, v := range *fileMap {
		if k != "resources" {
			continue
		}
		val := v.(map[interface{}]interface{})

		for key, _ := range val {
			osNode := val[key].(map[interface{}]interface{})
			_, ok := osNode["depends_on"]
			if ok {
				delete(osNode, "depends_on")
			}
			_, ok = osNode["properties"]
			if ok {
				properties := osNode["properties"].(map[interface{}]interface{})
				_, ok = properties["node-passwd"]
				if ok {
					if properties["node-passwd"] != nil {
						delete(properties, "node-passwd")
					} else {
						*fileMap = nil
					}
				}
			}
		}
	}
}

func validateLANParams(generatedHeatMap *map[string]interface{}, outputHeatMap *map[string]interface{}, t *testing.T) (bool, error) {

	var lanIPsMapGeneratedHeatTemplate = map[string]map[string]bool{}
	var lanIPsMapOutputHeatTemplate = map[string]map[string]bool{}

	addLANIPsSuccess, err := processLANParams(generatedHeatMap, &lanIPsMapGeneratedHeatTemplate, "add")
	if err != nil {
		t.Errorf("Failed to add LAN Params from Generated HEAT Template into a map")
		return addLANIPsSuccess, errors.New(err.Error())
	}
	t.Logf("LAN IPs Map for Generated HEAT Template after adding lan params: %v", lanIPsMapGeneratedHeatTemplate)

	addLANIPsSuccess, err = processLANParams(outputHeatMap, &lanIPsMapOutputHeatTemplate, "add")
	if err != nil {
		t.Errorf("Failed to add LAN Params from Output HEAT Template into a map")
		return addLANIPsSuccess, errors.New(err.Error())
	}
	t.Logf("LAN IPs Map for Output HEAT Template after adding lan params: %v", lanIPsMapOutputHeatTemplate)

	removeLANIPsSuccess, err := processLANParams(generatedHeatMap, &lanIPsMapGeneratedHeatTemplate, "remove")
	if err != nil {
		t.Errorf("Failed to remove LAN Params from Generated HEAT Template")
		return removeLANIPsSuccess, errors.New(err.Error())
	}
	t.Logf("LAN IPs Map for Generated HEAT Template after deleting lan params: %v", lanIPsMapGeneratedHeatTemplate)

	removeLANIPsSuccess, err = processLANParams(outputHeatMap, &lanIPsMapOutputHeatTemplate, "remove")
	if err != nil {
		t.Errorf("Failed to remove LAN Params from Output HEAT Template")
		return removeLANIPsSuccess, errors.New(err.Error())
	}
	t.Logf("LAN IPs Map for Output HEAT Template after deleting lan params: %v", lanIPsMapOutputHeatTemplate)

	return true, nil
}

func processLANParams(heatMap *map[string]interface{}, lanIPsMapGeneratedHeatTemplate *map[string]map[string]bool, operation string) (bool, error) {

	var t *testing.T

	for key, val := range *heatMap {
		if key != "resources" {
			continue
		}

		resources := val.(map[interface{}]interface{})

		for resourcesKey, _ := range resources {
			osNode := resources[resourcesKey].(map[interface{}]interface{})
			properties := osNode["properties"].(map[interface{}]interface{})

			var lan_sr_ip_prefix, lan_sr_ip_base, lan_sr_ip_osk_min, lan_sr_ip_osk_max string

			_, ok := properties["lan-sr-ip-prefix"]
			if ok {
				switch operation {
				case "add":
					_, ok1 := (*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-prefix"]
					if !ok1 {
						(*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-prefix"] = make(map[string]bool)
					}
					lan_sr_ip_prefix = properties["lan-sr-ip-prefix"].(string)
					(*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-prefix"][lan_sr_ip_prefix] = true
				case "remove":
					lan_sr_ip_prefix = properties["lan-sr-ip-prefix"].(string)
					_, ok1 := (*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-prefix"][lan_sr_ip_prefix]
					if !ok1 {
						t.Errorf("lan-sr-ip-prefix. %v does not exist in Generated Heat Yaml", lan_sr_ip_prefix)
						return false, errors.New("lan-sr-ip-prefix missing in Generated Heat Yaml")
					} else {
						delete((*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-prefix"], lan_sr_ip_prefix)
						delete(properties, "lan-sr-ip-prefix")
					}
				}
			}

			_, ok = properties["lan-sr-ip-base"]
			if ok {
				switch operation {
				case "add":
					_, ok1 := (*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-base"]
					if !ok1 {
						(*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-base"] = make(map[string]bool)
					}
					lan_sr_ip_base = properties["lan-sr-ip-base"].(string)
					(*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-base"][lan_sr_ip_base] = true
				case "remove":
					lan_sr_ip_base = properties["lan-sr-ip-base"].(string)
					_, ok1 := (*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-base"][lan_sr_ip_base]
					if !ok1 {
						t.Errorf("lan-sr-ip-base. %v does not exist in Generated Heat Yaml", lan_sr_ip_base)
						return false, errors.New("lan-sr-ip-base missing in Generated Heat Yaml")
					} else {
						delete((*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-base"], lan_sr_ip_base)
						delete(properties, "lan-sr-ip-base")
					}
				}
			}

			_, ok = properties["lan-sr-ip-osk-min"]
			if ok {
				switch operation {
				case "add":
					_, ok1 := (*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-min"]
					if !ok1 {
						(*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-min"] = make(map[string]bool)
					}
					lan_sr_ip_osk_min = properties["lan-sr-ip-osk-min"].(string)
					(*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-min"][lan_sr_ip_osk_min] = true
				case "remove":
					lan_sr_ip_osk_min = properties["lan-sr-ip-osk-min"].(string)
					_, ok1 := (*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-min"][lan_sr_ip_osk_min]
					if !ok1 {
						t.Errorf("lan-sr-ip-osk-min. %v does not exist in Generated Heat Yaml", lan_sr_ip_osk_min)
						return false, errors.New("lan-sr-ip-osk-min missing in Generated Heat Yaml")
					} else {
						delete((*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-min"], lan_sr_ip_osk_min)
						delete(properties, "lan-sr-ip-osk-min")
					}
				}
			}

			_, ok = properties["lan-sr-ip-osk-max"]
			if ok {
				switch operation {
				case "add":
					_, ok1 := (*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-max"]
					if !ok1 {
						(*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-max"] = make(map[string]bool)
					}
					lan_sr_ip_osk_max = properties["lan-sr-ip-osk-max"].(string)
					(*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-max"][lan_sr_ip_osk_max] = true
				case "remove":
					lan_sr_ip_osk_max = properties["lan-sr-ip-osk-max"].(string)
					_, ok1 := (*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-max"][lan_sr_ip_osk_max]
					if !ok1 {
						t.Errorf("lan-sr-ip-osk-max. %v does not exist in Generated Heat Yaml", lan_sr_ip_osk_max)
						return false, errors.New("lan-sr-ip-osk-max missing in Generated Heat Yaml")
					} else {
						delete((*lanIPsMapGeneratedHeatTemplate)["lan-sr-ip-osk-max"], lan_sr_ip_osk_max)
						delete(properties, "lan-sr-ip-osk-max")
					}
				}
			}
		}
	}

	return true, nil
}

func compare(in interface{}, out interface{}) bool {

	return reflect.DeepEqual(in, out)
}

func deleteOpenRcIfExist() {

	// check if tenant openrc exists
	// if yes, remove the file
	if _, err := os.Stat(tenantOpenRC); err == nil {
        _ = os.Remove(tenantOpenRC)
    }
	// check if admin openrc exists
    // if yes, remove the file
    if _, err := os.Stat(adminOpenRC); err == nil {
        _ = os.Remove(adminOpenRC)
    }
}

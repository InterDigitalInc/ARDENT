// Package rest handles http requests and routes them to their corresponding
// handler functions with specific service instance based on the URI.
// These handler functions further invoke service functions to perform the
// requested operations.
package rest

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/hot"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/infra"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/sanity"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/stack"
)

// A Payload defines service response status code and status message.
type Payload struct {
	Id  models.StatusId  `json:"status_id"`
	Msg models.StatusMsg `json:"status_str"`
}

const (
	// HEAT template file name
	heatTemplate = "stack-flame-platform.yaml"
)

// to store logger instance
var logger *logrus.Logger

// Handler routes http requests to their corresponding handler functions with provided
// infra, hot, stack and sanity service handles depending on the request URI.
//
// Parameters:
//  glogger: Logger instance.
//  i: Infra service handle.
//  ht: Hot service handle.
//  st: Stack service handle.
//  sn: Sanity service handle.
//
// Returns:
//  Service: Mux router instance.
//  error: Error(if any), otherwise nil.
func Handler(glogger *logrus.Logger, i infra.Service, ht hot.Service, st stack.Service, sn sanity.Service) (*mux.Router, error) {

	logger = glogger

	r := mux.NewRouter()

	// These handlers require more work - to ensure HTTP header is present,
	// in case content is expected etc.
	// Register all the handlers

	infra := r.PathPrefix("/infra").Subrouter()
	hot := r.PathPrefix("/hot").Subrouter()
	stack := r.PathPrefix("/stack").Subrouter()

	r.HandleFunc("/sanity-check", initiateSanityCheck(sn)).
		Methods("POST")
	r.HandleFunc("/sanity-check/status", getSanityCheckStatus(sn)).
		Methods("GET")
	r.HandleFunc("/sanity-check/results", getSanityCheckResults(sn)).
		Methods("GET")

	infra.Path("/descriptor").
		Methods("PUT").
		HandlerFunc(processInfraDescriptor(i))
	infra.Path("/descriptor").
		Methods("DELETE").
		HandlerFunc(deleteInfraDescriptor(i))
	infra.Path("/rc/admin").
		Methods("PUT").
		HandlerFunc(processAdminRC(i))
	infra.Path("/rc/admin").
		Methods("DELETE").
		HandlerFunc(deleteAdminRC(i))
	infra.Path("/rc/tenant").
		Methods("PUT").
		HandlerFunc(processTenantRC(i))
	infra.Path("/rc/tenant").
		Methods("DELETE").
		HandlerFunc(deleteTenantRC(i))

	hot.Path("/generate").
		Methods("POST").
		HandlerFunc(generateHot(ht))
	hot.Path("/descriptor").
		Methods("GET").
		HandlerFunc(getHot(ht))
	hot.Path("/descriptor").
		Methods("DELETE").
		HandlerFunc(deleteHot(ht))

	stack.Path("/create").
		Methods("POST").
		Headers("Content-Type", "application/json").
		HandlerFunc(createStack(st))
	stack.Path("/delete").
		Methods("POST").
		Headers("Content-Type", "application/json").
		HandlerFunc(deleteStack(st))
	stack.Path("/status/{stack_name}").
		Methods("GET").
		HandlerFunc(getStackStatus(st))

	return r, nil
}

func processInfraDescriptor(s infra.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Successfully hit /infra/descriptor endpoint!")

		if r.ContentLength == 0 {
			errMsg := "Payload expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		fileBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Errorf("Error in reading bytes from infra descriptor file. %v", err)

			// return internal server error
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			errMsg := "Failed to read bytes from infra-descriptor file"
			payload := Payload{models.INT_SERVER_ERR, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}
		logger.Debugf("Received Infra Descriptor: %v", string(fileBytes))

		// Calling infra Service ProcessDescriptor() to process infra decsriptor
		statusCode, statusMsg := s.ProcessDescriptor(fileBytes)
		statusCode, httpStatusCode := getStatusCode(statusCode, models.INFRA_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func deleteInfraDescriptor(s infra.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("/infra/descriptor end-point hit successfully for DELETE Method")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		statusCode, statusMsg := s.DeleteDescriptor()
		statusCode, httpStatusCode := getStatusCode(statusCode, models.INFRA_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func processAdminRC(s infra.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Successfully hit /infra/rc/admin endpoint!")

		if r.ContentLength == 0 {
			errMsg := "Payload expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		fileBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Errorf("Error in reading bytes from admin-openrc file. %v", err)

			// return internal server error
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			errMsg := "Failed to read bytes from 'admin-openrc' file"
			payload := Payload{models.INT_SERVER_ERR, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Calling infra Service ProcessAdminRC() to process Admin OpenRC
		statusCode, statusMsg := s.ProcessAdminRC(fileBytes)
		statusCode, httpStatusCode := getStatusCode(statusCode, models.INFRA_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func deleteAdminRC(s infra.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Successfully hit /infra/rc/admin end-point for DELETE Method")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Call Infra Service DeleteAdminRC() to delete Admin OpenRC
		statusCode, statusMsg := s.DeleteAdminRC()
		statusCode, httpStatusCode := getStatusCode(statusCode, models.INFRA_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func processTenantRC(s infra.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Successfully hit /infra/rc/tenant endpoint!")

		if r.ContentLength == 0 {
			errMsg := "Payload expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		fileBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Errorf("Error in reading bytes from tenant-openrc file. %v", err)

			// return internal server error
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			errMsg := "Failed to read bytes from 'tenant-openrc' file"
			payload := Payload{models.INT_SERVER_ERR, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Calling infra Service ProcessTenantRC() to process tenant OpenRC
		statusCode, statusMsg := s.ProcessTenantRC(fileBytes)
		statusCode, httpStatusCode := getStatusCode(statusCode, models.INFRA_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func deleteTenantRC(s infra.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Successfully hit /infra/rc/tenant end-point for DELETE Method")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Call Infra Service DeleteTenantRC() to delete Tenant OpenRC
		statusCode, statusMsg := s.DeleteTenantRC()
		statusCode, httpStatusCode := getStatusCode(statusCode, models.INFRA_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func generateHot(s hot.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("/hot/generate endpoint hit successfully!")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Call HOT Service Generate() to generate HOT
		statusCode, statusMsg := s.Generate()

		statusCode, httpStatusCode := getStatusCode(statusCode, models.HOT_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func getHot(s hot.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Successfully hit /hot/descriptor endpoint for GET Method")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Call HOT Service GetDescriptor() to get generated HOT
		statusCode, statusMsg, fd := s.GetDescriptor()
		if statusCode != models.NO_ERR {

			statusCode, httpStatusCode := getStatusCode(statusCode, models.HOT_SERVICE)
			w.WriteHeader(httpStatusCode)

			w.Header().Set("Content-Type", "application/json")
			payload := Payload{statusCode, statusMsg}
			json.NewEncoder(w).Encode(payload)
			return
		}
		defer fd.Close()

		// get file stat
		fileStat, _ := fd.Stat()
		fileSize := fileStat.Size()

		// create a buffer to store the header of the file
		// and copy headers into it
		fileHeader := make([]byte, fileSize)
		fd.Read(fileHeader)
		logger.Debugf("fileHeader : %s", fileHeader)

		// get content type of file
		fileContentType := http.DetectContentType(fileHeader)
		logger.Debugf("fileContentType : %s", fileContentType)

		// send the headers
		w.Header().Set("Content-Disposition", "attachment; filename="+heatTemplate)
		w.Header().Set("Content-Type", fileContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
		fd.Seek(0, 0)
		io.Copy(w, fd)
	}
}

func deleteHot(s hot.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Successfully hit /hot/descriptor endpoint for DELETE Method")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Call HOT Service DeleteDescriptor() to delete existing HOT
		statusCode, statusMsg := s.DeleteDescriptor()
		statusCode, httpStatusCode := getStatusCode(statusCode, models.HOT_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func createStack(s stack.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("/stack/create endpoint hit successfully!")

		if r.ContentLength == 0 {
			errMsg := "Payload is expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		var newStack stack.Create

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&newStack)
		if err != nil {
			logger.Errorf("Error in decoding json received in request body. %v", err)

			// return internal server error
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			errMsg := "Failed to decode json received in request body"
			payload := Payload{models.INT_SERVER_ERR, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Calling stack Service Create() to create stack
		statusCode, statusMsg := s.Create(newStack)
		if statusCode != models.NO_ERR {
			statusCode, httpStatusCode := getStatusCode(statusCode, models.STACK_SERVICE)
			w.WriteHeader(httpStatusCode)

			w.Header().Set("Content-Type", "application/json")
			payload := Payload{statusCode, statusMsg}
			json.NewEncoder(w).Encode(payload)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func deleteStack(s stack.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("/stack/delete endpoint hit successfully!")

		if r.ContentLength == 0 {
			errMsg := "Payload is expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		var newStack stack.Delete

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&newStack)
		if err != nil {
			logger.Errorf("Error in decoding json received in request body. %v", err)

			// return internal server error
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			errMsg := "Failed to decode json received in request body"
			payload := Payload{models.INT_SERVER_ERR, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Calling stack Service Delete() to delete stack
		statusCode, statusMsg := s.Delete(newStack)
		if statusCode != models.NO_ERR {
			statusCode, httpStatusCode := getStatusCode(statusCode, models.STACK_SERVICE)
			w.WriteHeader(httpStatusCode)

			w.Header().Set("Content-Type", "application/json")
			payload := Payload{statusCode, statusMsg}
			json.NewEncoder(w).Encode(payload)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func getStackStatus(s stack.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("/stack/status/<NAME> endpoint hit successfully!")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		vars := mux.Vars(r)
		stackName := vars["stack_name"]

		statusCode, statusMsg := s.Status(stackName)
		statusCode, httpStatusCode := getStatusCode(statusCode, models.STACK_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func initiateSanityCheck(s sanity.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("/sanity-check endpoint hit successfully!")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		// Calling sanity Service Initiate() to initiate sanity-check
		statusCode, statusMsg := s.Initiate()
		if statusCode != models.NO_ERR {
			statusCode, httpStatusCode := getStatusCode(statusCode, models.SANITY_SERVICE)
			w.WriteHeader(httpStatusCode)

			w.Header().Set("Content-Type", "application/json")
			payload := Payload{statusCode, statusMsg}
			json.NewEncoder(w).Encode(payload)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func getSanityCheckStatus(s sanity.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("/sanity-check/status endpoint hit successfully!")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		statusCode, statusMsg := s.Status()
		statusCode, httpStatusCode := getStatusCode(statusCode, models.SANITY_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		payload := Payload{statusCode, statusMsg}
		json.NewEncoder(w).Encode(payload)
	}
}

func getSanityCheckResults(s sanity.Service) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Successfully hit /sanity-check/results end-point")

		if r.ContentLength != 0 {
			errMsg := "Payload not expected in the Request"
			logger.Debugf(errMsg)

			// return request forbidden
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			payload := Payload{models.REQ_FORBIDDEN, models.StatusMsg(errMsg)}
			json.NewEncoder(w).Encode(payload)
			return
		}

		statusCode, statusMsg, sanityCheckResultPayload := s.Results()
		//statusCode,  statusMsg := s.Results()
		statusCode, httpStatusCode := getStatusCode(statusCode, models.SANITY_SERVICE)
		w.WriteHeader(httpStatusCode)

		w.Header().Set("Content-Type", "application/json")
		if statusCode != models.NO_ERR {
			payload := Payload{statusCode, statusMsg}
			json.NewEncoder(w).Encode(payload)
		} else {
			json.NewEncoder(w).Encode(sanityCheckResultPayload)
		}
	}
}

func getStatusCode(serviceStatusCode models.StatusId, serviceType byte) (models.StatusId, int) {
	httpStatusCode := http.StatusOK

	if serviceStatusCode >= models.REQ_FORBIDDEN_ERR_START && serviceStatusCode <= models.REQ_FORBIDDEN_ERR_END {
		httpStatusCode = http.StatusForbidden
	} else if serviceStatusCode >= models.INT_SERVER_ERR_START && serviceStatusCode <= models.INT_SERVER_ERR_END {
		httpStatusCode = http.StatusInternalServerError
	} else if serviceStatusCode >= models.EXTERNAL_ORCH_ERR_START && serviceStatusCode <= models.EXTERNAL_ORCH_ERR_END {
		httpStatusCode = http.StatusOK
	}

	if serviceStatusCode >= models.COMMON_ERROR_CODE_START && serviceStatusCode <= models.COMMON_ERROR_CODE_END {
		switch serviceType {
		case models.INFRA_SERVICE:
			serviceStatusCode = models.INFRA_SERVICE_COMMON_ERR_START + serviceStatusCode
		case models.SANITY_SERVICE:
			serviceStatusCode = models.SANITY_SERVICE_COMMON_ERR_START + serviceStatusCode
		case models.HOT_SERVICE:
			serviceStatusCode = models.HOT_SERVICE_COMMON_ERR_START + serviceStatusCode
		case models.STACK_SERVICE:
			serviceStatusCode = models.STACK_SERVICE_COMMON_ERR_START + serviceStatusCode
		}
	}
	logger.Debugf("Final StatusId: %v, HTTP Status Code: %v", serviceStatusCode, httpStatusCode)

	return serviceStatusCode, httpStatusCode
}

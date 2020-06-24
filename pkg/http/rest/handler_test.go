package rest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var router *mux.Router

func TestHandler(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	r, err := Handler(logger, nil, nil, nil)
	if err != nil {
		t.Errorf("Error in initializing HTTP REST Service. %v", err)
	}
	router = r
}

func TestProcessDescriptor(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		// Create a request. We don't have any query parameters, so we'll
		// pass 'nil' as the third parameter.
		r, err := http.NewRequest("GET", "/infra/descriptor", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		// We create a ResponseRecorder which satisfies http.ResponseWriter
		// to record the response.
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
	t.Run("missingDescriptorInfra", func(t *testing.T) {
		r, err := http.NewRequest("PUT", "/infra/descriptor", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusOK {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusOK)
		}
		// Check if the response body is what we expected
		received := strings.TrimSuffix(w.Body.String(), "\n")
		expected := `{"status_id":1,"status_str":"Failed to retrieve descriptor-infra key from request"}`
		if received != expected {
			t.Errorf("handler returned unexpected body: received %v expected %v",
				received, expected)
		}
	})
}

func TestProcessAdminRC(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/infra/rc/admin", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
	t.Run("missingOpenrcAdmin", func(t *testing.T) {
		r, err := http.NewRequest("PUT", "/infra/rc/admin", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusOK {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusOK)
		}
		// Check if the response body is what we expected
		received := strings.TrimSuffix(w.Body.String(), "\n")
		expected := `{"status_id":1,"status_str":"Failed to retrieve openrc-admin key from request"}`
		if received != expected {
			t.Errorf("handler returned unexpected body: received %v expected %v",
				received, expected)
		}
	})
}

func TestProcessTenantRC(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/infra/rc/tenant", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
	t.Run("missingTenantRC", func(t *testing.T) {
		r, err := http.NewRequest("PUT", "/infra/rc/tenant", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check the response code is what we expected
		if w.Code != http.StatusOK {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusOK)
		}
		// Check the response body is what we expected
		received := strings.TrimSuffix(w.Body.String(), "\n")
		expected := `{"status_id":1,"status_str":"Failed to retrieve openrc-tenant key from request"}`
		if received != expected {
			t.Errorf("handler returned unexpected body: received %v expected %v",
				received, expected)
		}
	})
}

func TestGenerateStack(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/stack/generate", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
}

func TestCreateStack(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		payload := []byte(`{"name":"stackName"}`)
		r, err := http.NewRequest("GET", "/stack/create", bytes.NewBuffer(payload))
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
	t.Run("missingHeader", func(t *testing.T) {
		r, err := http.NewRequest("POST", "/stack/create", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusNotFound {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusNotFound)
		}
		// Check if the response body is what we expected
		received := strings.TrimSuffix(w.Body.String(), "\n")
		expected := "404 page not found"
		if received != expected {
			t.Errorf("handler returned unexpected body: received %v expected %v",
				received, expected)
		}
	})
	t.Run("invalidPayload", func(t *testing.T) {
		payload := []byte(`{"name""stackName"}`)
		r, err := http.NewRequest("POST", "/stack/create", bytes.NewBuffer(payload))
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusOK {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusOK)
		}
		// Check the response body is what we expected
		received := strings.TrimSuffix(w.Body.String(), "\n")
		expected := `{"status_id":1,"status_str":"Failed to decode json received in request body"}`
		if received != expected {
			t.Errorf("handler returned unexpected body: received %v expected %v",
				received, expected)
		}
	})
}

func TestDeleteStack(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		payload := []byte(`{"name":"stackName"}`)
		r, err := http.NewRequest("GET", "/stack/delete", bytes.NewBuffer(payload))
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
	t.Run("missingHeader", func(t *testing.T) {
		r, err := http.NewRequest("POST", "/stack/delete", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusNotFound {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusNotFound)
		}
		// Check if the response body is what we expected
		received := strings.TrimSuffix(w.Body.String(), "\n")
		expected := "404 page not found"
		if received != expected {
			t.Errorf("handler returned unexpected body: received %v expected %v",
				received, expected)
		}
	})
	t.Run("invalidPayload", func(t *testing.T) {
		payload := []byte(`{"name""stackName}"`)
		r, err := http.NewRequest("POST", "/stack/delete", bytes.NewBuffer(payload))
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusOK {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusOK)
		}
		// Check the response body is what we expected
		received := strings.TrimSuffix(w.Body.String(), "\n")
		expected := `{"status_id":1,"status_str":"Failed to decode json received in request body"}`
		if received != expected {
			t.Errorf("handler returned unexpected body: received %v expected %v",
				received, expected)
		}
	})
}

func TestGetStackStatus(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		r, err := http.NewRequest("POST", "/stack/status/stackName", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
}

func TestInitiateSanityCheck(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/sanity-check", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
}

func TestGetSanityCheckStatus(t *testing.T) {

	t.Run("invalidMethod", func(t *testing.T) {
		r, err := http.NewRequest("POST", "/sanity-check/status", nil)
		if err != nil {
			t.Errorf("Error in creating HTTP Request. %v", err)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		// Check if the response code is what we expected
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code: received %v expected %v",
				w.Code, http.StatusMethodNotAllowed)
		}
	})
}

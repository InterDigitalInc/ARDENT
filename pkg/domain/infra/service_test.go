package infra

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/storage/mysql"
)

func TestDeleteDescriptor(t *testing.T) {
	logger = logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	s, err := mysql.NewStorage(logger, "root", "hsc321", "ardent")
	if err != nil {
		t.Errorf("Error in initializing Store. %v", err)
		return
	}

	t.Log("Initializing Infra Service")
	infra, err := NewService(logger, s)
	if err != nil {
		t.Errorf("Error in initializing Infra Service. %v", err)
		return
	}
	t.Log("Infra Service initialized successfully!")

	orchGetStackList = func() ([]map[string]string, error) {
		t.Log("orchestrator.GetStackListMock")
		out := []byte("[{\"abc\": \"def\"}]")

		stackList := []map[string]string{}

		// Unmarshal or Decode the JSON to the interface
		err := json.Unmarshal(out, &stackList)
		if err != nil {
			t.Logf("orchestrator.GetStackListMock: Error in unmarshalling OpenStack command output. %v", err)
			errMsg := "orchestrator.GetStackListMock: Failed to unmarshal OpenStack command output"
			return nil, errors.New(errMsg)
		}

		return stackList, nil

	}
	_, statusMsg := infra.DeleteDescriptor()
	if statusMsg != "HEAT Stack has already been launched" {
		t.Errorf("Test Failed")
		t.Failed()
	}
}

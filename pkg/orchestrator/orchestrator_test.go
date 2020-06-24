package orchestrator

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestIntialize(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	err := Intialize(logger)
	if err != nil {
		t.Errorf("orchestrator returned unexpected error msg: "+
			"received - %s. expected - nil", err.Error())
	}
}

func TestGetStackList(t *testing.T) {

	t.Run("missingOpenrc", func(t *testing.T) {
		_, err := GetStackList()
		expected := "tenant-openrc is not uploaded"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
	t.Run("successful", func(t *testing.T) {
		_, err := GetStackList()
		if err != nil {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - nil.", err.Error())
		}
	})
}

func TestLaunchHeatStack(t *testing.T) {

	t.Run("missingTemplate", func(t *testing.T) {
		_, err := LaunchHeatStack("testStack")
		expected := "HEAT template is not generated"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
	t.Run("missingOpenrc", func(t *testing.T) {
		_, err := LaunchHeatStack("testStack")
		expected := "tenant-openrc is not uploaded"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
	t.Run("emptyStackName", func(t *testing.T) {
		_, err := LaunchHeatStack("")
		expected := "Failed to create stack"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
}

func TestDeleteHeatStack(t *testing.T) {

	t.Run("missingOpenrc", func(t *testing.T) {
		err := DeleteHeatStack("testStack")
		expected := "tenant-openrc is not uploaded"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
	t.Run("emptyStackName", func(t *testing.T) {
		err := DeleteHeatStack("")
		expected := "Failed to delete stack"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
}

func TestGetStackStatus(t *testing.T) {

	t.Run("missingOpenrc", func(t *testing.T) {
		_, err := GetStackStatus("testStack")
		expected := "tenant-openrc is not uploaded"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
	t.Run("emptyStackName", func(t *testing.T) {
		_, err := GetStackStatus("")
		expected := "Failed to get stack status"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
	t.Run("stackNotCreated", func(t *testing.T) {
		_, err := GetStackStatus("testStack")
		expected := "Stack does not exist"
		if err.Error() != expected {
			t.Errorf("orchestrator returned unexpected error msg: "+
				"received - %s. expected - %s.", err.Error(), expected)
		}
	})
}

func TestPerformSanityCheck(t *testing.T) {

}

package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/solomon-os/go-test/internal/drift"
	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/reporter"
	"github.com/solomon-os/go-test/internal/terraform"
)

type mockAWSClient struct {
	instances   map[string]*models.EC2Instance
	getErr      error
	getMultiErr error
}

func (m *mockAWSClient) GetInstance(
	ctx context.Context,
	instanceID string,
) (*models.EC2Instance, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	inst, ok := m.instances[instanceID]
	if !ok {
		return nil, errors.New("instance not found")
	}
	return inst, nil
}

func (m *mockAWSClient) GetInstances(
	ctx context.Context,
	instanceIDs []string,
) ([]*models.EC2Instance, error) {
	if m.getMultiErr != nil {
		return nil, m.getMultiErr
	}
	var result []*models.EC2Instance
	for _, id := range instanceIDs {
		if inst, ok := m.instances[id]; ok {
			result = append(result, inst)
		}
	}
	return result, nil
}

func TestNewDefaultApp(t *testing.T) {
	app := newDefaultApp()
	if app == nil {
		t.Fatal("newDefaultApp returned nil")
	}
	if app.Output == nil {
		t.Error("Output should not be nil")
	}
	if app.NewAWSClient == nil {
		t.Error("NewAWSClient should not be nil")
	}
}

func TestGetParser(t *testing.T) {
	setupOnce.Do(setup)

	t.Run("returns default parser when not set", func(t *testing.T) {
		defaultApp.Parser = nil
		p := getParser()
		if p == nil {
			t.Error("getParser returned nil")
		}
	})

	t.Run("returns custom parser when set", func(t *testing.T) {
		customParser := terraform.NewParser()
		defaultApp.Parser = customParser
		p := getParser()
		if p != customParser {
			t.Error("getParser should return custom parser")
		}
		defaultApp.Parser = nil
	})
}

func TestGetDetector(t *testing.T) {
	setupOnce.Do(setup)

	t.Run("returns default detector when not set", func(t *testing.T) {
		defaultApp.Detector = nil
		d := getDetector()
		if d == nil {
			t.Error("getDetector returned nil")
		}
	})

	t.Run("returns custom detector when set", func(t *testing.T) {
		customDetector := drift.NewDetector([]string{"instance_type"})
		defaultApp.Detector = customDetector
		d := getDetector()
		if d != customDetector {
			t.Error("getDetector should return custom detector")
		}
		defaultApp.Detector = nil
	})
}

func TestGetReporter(t *testing.T) {
	setupOnce.Do(setup)

	t.Run("returns default reporter when not set", func(t *testing.T) {
		defaultApp.Reporter = nil
		r := getReporter()
		if r == nil {
			t.Error("getReporter returned nil")
		}
	})

	t.Run("returns custom reporter when set", func(t *testing.T) {
		customReporter := reporter.New(&bytes.Buffer{}, reporter.FormatJSON)
		defaultApp.Reporter = customReporter
		r := getReporter()
		if r != customReporter {
			t.Error("getReporter should return custom reporter")
		}
		defaultApp.Reporter = nil
	})
}

func TestGetAWSClient(t *testing.T) {
	setupOnce.Do(setup)

	t.Run("returns custom client when set", func(t *testing.T) {
		mockClient := &mockAWSClient{}
		defaultApp.AWSClient = mockClient
		c, err := getAWSClient(context.Background(), "us-east-1")
		if err != nil {
			t.Errorf("getAWSClient returned error: %v", err)
		}
		if c != mockClient {
			t.Error("getAWSClient should return custom client")
		}
		defaultApp.AWSClient = nil
	})
}

func TestRunListAttributes(t *testing.T) {
	setupOnce.Do(setup)

	var buf bytes.Buffer
	defaultApp.Output = &buf

	runListAttributes(nil, nil)

	output := buf.String()
	if output == "" {
		t.Error("runListAttributes produced no output")
	}

	for _, attr := range []string{"instance_type", "ami", "security_groups", "tags"} {
		if !bytes.Contains([]byte(output), []byte(attr)) {
			t.Errorf("output should contain attribute: %s", attr)
		}
	}

	defaultApp.Output = os.Stdout
}

func TestRunDetector_ParseError(t *testing.T) {
	setupOnce.Do(setup)

	tfStatePath = "/nonexistent/path.tfstate"
	err := runDetector(nil, nil)
	if err == nil {
		t.Error("runDetector should return error for nonexistent file")
	}
}

func TestRunDetector_NoInstances(t *testing.T) {
	setupOnce.Do(setup)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "empty.tfstate")
	emptyState := `{"version": 4, "resources": []}`
	if err := os.WriteFile(statePath, []byte(emptyState), 0o644); err != nil {
		t.Fatalf("Failed to create temp state file: %v", err)
	}

	tfStatePath = statePath
	err := runDetector(nil, nil)
	if err == nil {
		t.Error("runDetector should return error for empty state")
	}
}

func TestRunDetector_Success(t *testing.T) {
	setupOnce.Do(setup)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.tfstate")
	stateContent := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "test",
				"instances": [
					{"attributes": {"id": "i-123", "instance_type": "t2.micro", "ami": "ami-123"}}
				]
			}
		]
	}`
	if err := os.WriteFile(statePath, []byte(stateContent), 0o644); err != nil {
		t.Fatalf("Failed to create temp state file: %v", err)
	}

	tfStatePath = statePath
	region = "us-east-1"
	instanceIDs = nil
	attributes = []string{"instance_type"}
	outputFmt = "json"

	mockClient := &mockAWSClient{
		instances: map[string]*models.EC2Instance{
			"i-123": {
				InstanceID:   "i-123",
				InstanceType: "t2.micro",
				AMI:          "ami-123",
			},
		},
	}
	defaultApp.AWSClient = mockClient

	var buf bytes.Buffer
	defaultApp.Output = &buf
	defaultApp.Reporter = nil

	err := runDetector(nil, nil)
	if err != nil {
		t.Errorf("runDetector returned error: %v", err)
	}

	defaultApp.AWSClient = nil
	defaultApp.Output = os.Stdout
}

func TestRunDetector_AWSClientError(t *testing.T) {
	setupOnce.Do(setup)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.tfstate")
	stateContent := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "test",
				"instances": [{"attributes": {"id": "i-123", "instance_type": "t2.micro"}}]
			}
		]
	}`
	if err := os.WriteFile(statePath, []byte(stateContent), 0o644); err != nil {
		t.Fatalf("Failed to create temp state file: %v", err)
	}

	tfStatePath = statePath
	mockClient := &mockAWSClient{
		getMultiErr: errors.New("AWS API error"),
	}
	defaultApp.AWSClient = mockClient

	err := runDetector(nil, nil)
	if err == nil {
		t.Error("runDetector should return error on AWS client failure")
	}

	defaultApp.AWSClient = nil
}

func TestRunSingleDetect_ParseError(t *testing.T) {
	setupOnce.Do(setup)

	tfStatePath = "/nonexistent/path.tfstate"
	err := runSingleDetect(nil, []string{"i-123"})
	if err == nil {
		t.Error("runSingleDetect should return error for nonexistent file")
	}
}

func TestRunSingleDetect_InstanceNotInTerraform(t *testing.T) {
	setupOnce.Do(setup)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.tfstate")
	stateContent := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "test",
				"instances": [{"attributes": {"id": "i-123", "instance_type": "t2.micro"}}]
			}
		]
	}`
	if err := os.WriteFile(statePath, []byte(stateContent), 0o644); err != nil {
		t.Fatalf("Failed to create temp state file: %v", err)
	}

	tfStatePath = statePath
	err := runSingleDetect(nil, []string{"i-nonexistent"})
	if err == nil {
		t.Error("runSingleDetect should return error for nonexistent instance")
	}
}

func TestRunSingleDetect_AWSClientError(t *testing.T) {
	setupOnce.Do(setup)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.tfstate")
	stateContent := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "test",
				"instances": [{"attributes": {"id": "i-123", "instance_type": "t2.micro"}}]
			}
		]
	}`
	if err := os.WriteFile(statePath, []byte(stateContent), 0o644); err != nil {
		t.Fatalf("Failed to create temp state file: %v", err)
	}

	tfStatePath = statePath
	mockClient := &mockAWSClient{
		getErr: errors.New("AWS API error"),
	}
	defaultApp.AWSClient = mockClient

	err := runSingleDetect(nil, []string{"i-123"})
	if err == nil {
		t.Error("runSingleDetect should return error on AWS client failure")
	}

	defaultApp.AWSClient = nil
}

func TestRunSingleDetect_Success(t *testing.T) {
	setupOnce.Do(setup)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.tfstate")
	stateContent := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "test",
				"instances": [{"attributes": {"id": "i-123", "instance_type": "t2.micro", "ami": "ami-123"}}]
			}
		]
	}`
	if err := os.WriteFile(statePath, []byte(stateContent), 0o644); err != nil {
		t.Fatalf("Failed to create temp state file: %v", err)
	}

	tfStatePath = statePath
	attributes = []string{"instance_type"}
	outputFmt = "json"

	mockClient := &mockAWSClient{
		instances: map[string]*models.EC2Instance{
			"i-123": {
				InstanceID:   "i-123",
				InstanceType: "t2.micro",
				AMI:          "ami-123",
			},
		},
	}
	defaultApp.AWSClient = mockClient

	var buf bytes.Buffer
	defaultApp.Output = &buf
	defaultApp.Reporter = nil

	err := runSingleDetect(nil, []string{"i-123"})
	if err != nil {
		t.Errorf("runSingleDetect returned error: %v", err)
	}

	defaultApp.AWSClient = nil
	defaultApp.Output = os.Stdout
}

func TestMust(t *testing.T) {
	t.Run("does not panic on nil error", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Error("must panicked on nil error")
			}
		}()
		must(nil)
	})

	t.Run("panics on non-nil error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("must should panic on non-nil error")
			}
		}()
		must(errors.New("test error"))
	})
}

func TestRun(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"drift-detector", "--help"}

	err := Run()
	if err != nil {
		t.Errorf("Run returned error: %v", err)
	}
}

// Package terraform provides functionality to parse Terraform state and HCL files.
package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/solomon-os/go-test/internal/logger"
	"github.com/solomon-os/go-test/internal/models"
)

// StateParser defines the interface for parsing Terraform state and HCL files.
type StateParser interface {
	ParseFile(filePath string) (map[string]*models.EC2Instance, error)
	ParseStateFile(filePath string) (map[string]*models.EC2Instance, error)
	ParseStateJSON(data []byte) (map[string]*models.EC2Instance, error)
	ParseHCLFile(filePath string) (map[string]*models.EC2Instance, error)
	ParseHCL(data []byte, filename string) (map[string]*models.EC2Instance, error)
	GetInstanceByID(instances map[string]*models.EC2Instance, instanceID string) (*models.EC2Instance, error)
}

// Parser handles parsing of Terraform configuration files.
type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

// State represents the structure of a Terraform state file.
type State struct {
	Version   int             `json:"version"`
	Resources []StateResource `json:"resources"`
}

// StateResource represents a resource in the Terraform state.
type StateResource struct {
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Provider  string          `json:"provider"`
	Instances []StateInstance `json:"instances"`
}

// StateInstance represents an instance of a resource.
type StateInstance struct {
	Attributes json.RawMessage `json:"attributes"`
}

// EC2Attributes represents the attributes of an EC2 instance in Terraform state.
type EC2Attributes struct {
	ID                  string                `json:"id"`
	AMI                 string                `json:"ami"`
	InstanceType        string                `json:"instance_type"`
	AvailabilityZone    string                `json:"availability_zone"`
	SubnetID            string                `json:"subnet_id"`
	VpcSecurityGroupIDs []string              `json:"vpc_security_group_ids"`
	SecurityGroups      []string              `json:"security_groups"`
	KeyName             string                `json:"key_name"`
	PrivateIP           string                `json:"private_ip"`
	PublicIP            string                `json:"public_ip"`
	EBSOptimized        bool                  `json:"ebs_optimized"`
	Monitoring          bool                  `json:"monitoring"`
	IAMInstanceProfile  string                `json:"iam_instance_profile"`
	Tags                map[string]string     `json:"tags"`
	TagsAll             map[string]string     `json:"tags_all"`
	RootBlockDevice     []RootBlockDeviceAttr `json:"root_block_device"`
}

// RootBlockDeviceAttr represents root block device attributes.
type RootBlockDeviceAttr struct {
	VolumeSize          int    `json:"volume_size"`
	VolumeType          string `json:"volume_type"`
	DeleteOnTermination bool   `json:"delete_on_termination"`
	Encrypted           bool   `json:"encrypted"`
	IOPS                int    `json:"iops"`
	Throughput          int    `json:"throughput"`
}

func (p *Parser) ParseStateFile(filePath string) (map[string]*models.EC2Instance, error) {
	logger.Debug("reading Terraform state file", "path", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error("failed to read state file", "path", filePath, "error", err)
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	return p.ParseStateJSON(data)
}

func (p *Parser) ParseStateJSON(data []byte) (map[string]*models.EC2Instance, error) {
	logger.Debug("parsing Terraform state JSON", "bytes", len(data))
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		logger.Error("failed to parse state JSON", "error", err)
		return nil, fmt.Errorf("failed to parse state JSON: %w", err)
	}

	instances := make(map[string]*models.EC2Instance)

	for _, resource := range state.Resources {
		if resource.Type != "aws_instance" {
			continue
		}

		for _, inst := range resource.Instances {
			ec2Inst, err := p.parseEC2Attributes(inst.Attributes)
			if err != nil {
				logger.Error("failed to parse EC2 attributes", "resource", resource.Name, "error", err)
				return nil, fmt.Errorf("failed to parse EC2 attributes for %s: %w", resource.Name, err)
			}
			instances[ec2Inst.InstanceID] = ec2Inst
		}
	}

	logger.Info("parsed Terraform state", "instance_count", len(instances))
	return instances, nil
}

func (p *Parser) parseEC2Attributes(data json.RawMessage) (*models.EC2Instance, error) {
	var attrs EC2Attributes
	if err := json.Unmarshal(data, &attrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal EC2 attributes: %w", err)
	}

	instance := &models.EC2Instance{
		InstanceID:         attrs.ID,
		InstanceType:       attrs.InstanceType,
		AMI:                attrs.AMI,
		AvailabilityZone:   attrs.AvailabilityZone,
		SubnetID:           attrs.SubnetID,
		PrivateIP:          attrs.PrivateIP,
		PublicIP:           attrs.PublicIP,
		KeyName:            attrs.KeyName,
		EBSOptimized:       attrs.EBSOptimized,
		Monitoring:         attrs.Monitoring,
		IAMInstanceProfile: attrs.IAMInstanceProfile,
		Tags:               attrs.Tags,
	}

	if len(instance.Tags) == 0 && len(attrs.TagsAll) > 0 {
		instance.Tags = attrs.TagsAll
	}

	if len(attrs.VpcSecurityGroupIDs) > 0 {
		instance.SecurityGroups = attrs.VpcSecurityGroupIDs
	} else {
		instance.SecurityGroups = attrs.SecurityGroups
	}

	if len(attrs.RootBlockDevice) > 0 {
		rbd := attrs.RootBlockDevice[0]
		instance.RootBlockDevice = models.BlockDevice{
			VolumeSize:          rbd.VolumeSize,
			VolumeType:          rbd.VolumeType,
			DeleteOnTermination: rbd.DeleteOnTermination,
			Encrypted:           rbd.Encrypted,
			IOPS:                rbd.IOPS,
			Throughput:          rbd.Throughput,
		}
	}

	return instance, nil
}

func (p *Parser) ParseFile(filePath string) (map[string]*models.EC2Instance, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	logger.Debug("parsing Terraform file", "path", filePath, "extension", ext)

	switch ext {
	case ".tfstate", ".json":
		return p.ParseStateFile(filePath)
	case ".tf":
		return p.ParseHCLFile(filePath)
	default:
		logger.Error("unsupported file type", "path", filePath, "extension", ext)
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
}

func (p *Parser) GetInstanceByID(instances map[string]*models.EC2Instance, instanceID string) (*models.EC2Instance, error) {
	instance, ok := instances[instanceID]
	if !ok {
		return nil, fmt.Errorf("instance %s not found in Terraform configuration", instanceID)
	}
	return instance, nil
}

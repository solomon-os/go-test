// Package models defines the data structures used for EC2 drift detection.
package models

// EC2Instance represents a normalized EC2 instance configuration
// that can be compared between AWS and Terraform sources.
type EC2Instance struct {
	InstanceID         string            `json:"instance_id"`
	InstanceType       string            `json:"instance_type"`
	AMI                string            `json:"ami"`
	AvailabilityZone   string            `json:"availability_zone"`
	SubnetID           string            `json:"subnet_id"`
	VpcID              string            `json:"vpc_id"`
	PrivateIP          string            `json:"private_ip"`
	PublicIP           string            `json:"public_ip"`
	KeyName            string            `json:"key_name"`
	SecurityGroups     []string          `json:"security_groups"`
	Tags               map[string]string `json:"tags"`
	RootBlockDevice    BlockDevice       `json:"root_block_device"`
	EBSOptimized       bool              `json:"ebs_optimized"`
	Monitoring         bool              `json:"monitoring"`
	IAMInstanceProfile string            `json:"iam_instance_profile"`
}

// BlockDevice represents an EBS block device configuration.
type BlockDevice struct {
	VolumeSize          int    `json:"volume_size"`
	VolumeType          string `json:"volume_type"`
	DeleteOnTermination bool   `json:"delete_on_termination"`
	Encrypted           bool   `json:"encrypted"`
	IOPS                int    `json:"iops"`
	Throughput          int    `json:"throughput"`
}

// DriftResult contains the results of a drift detection comparison.
type DriftResult struct {
	InstanceID   string        `json:"instance_id"`
	HasDrift     bool          `json:"has_drift"`
	DriftedAttrs []DriftedAttr `json:"drifted_attributes,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// DriftedAttr represents a single attribute that has drifted.
type DriftedAttr struct {
	Path           string      `json:"path"`
	AWSValue       interface{} `json:"aws_value"`
	TerraformValue interface{} `json:"terraform_value"`
}

// DriftReport contains the complete drift detection report for multiple instances.
type DriftReport struct {
	TotalInstances   int           `json:"total_instances"`
	DriftedInstances int           `json:"drifted_instances"`
	Results          []DriftResult `json:"results"`
}

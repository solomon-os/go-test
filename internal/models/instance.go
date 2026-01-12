// Package models defines the data structures used for EC2 drift detection.
//
// The models package provides normalized representations of EC2 instances
// that can be compared between AWS (actual state) and Terraform (desired state).
// These structures are designed to capture the essential configuration attributes
// that are commonly managed through Infrastructure as Code.
//
// Core types:
//   - EC2Instance: Represents an EC2 instance configuration
//   - BlockDevice: Represents EBS block device configuration
//   - DriftResult: Contains comparison results for a single instance
//   - DriftReport: Aggregates results for multiple instances
//
// Example usage:
//
//	instance := &models.EC2Instance{
//	    InstanceID:   "i-1234567890abcdef0",
//	    InstanceType: "t2.micro",
//	    AMI:          "ami-0123456789abcdef0",
//	}
package models

// EC2Instance represents a normalized EC2 instance configuration
// that can be compared between AWS and Terraform sources.
//
// Fields are mapped from both AWS DescribeInstances API responses and
// Terraform aws_instance resource attributes. The normalization ensures
// consistent comparison regardless of the data source.
//
// Security groups are stored as group IDs (sg-xxx) for consistency,
// as Terraform primarily uses VPC security group IDs.
type EC2Instance struct {
	// InstanceID is the unique EC2 instance identifier (e.g., "i-1234567890abcdef0").
	InstanceID string `json:"instance_id"`

	// InstanceType is the EC2 instance type (e.g., "t2.micro", "m5.large").
	InstanceType string `json:"instance_type"`

	// AMI is the Amazon Machine Image ID used to launch the instance.
	AMI string `json:"ami"`

	// AvailabilityZone is the AWS availability zone (e.g., "us-east-1a").
	AvailabilityZone string `json:"availability_zone"`

	// SubnetID is the VPC subnet ID where the instance is launched.
	SubnetID string `json:"subnet_id"`

	// VpcID is the VPC ID where the instance resides.
	VpcID string `json:"vpc_id"`

	// PrivateIP is the private IPv4 address assigned to the instance.
	PrivateIP string `json:"private_ip"`

	// PublicIP is the public IPv4 address, if assigned.
	PublicIP string `json:"public_ip"`

	// KeyName is the name of the key pair used for SSH access.
	KeyName string `json:"key_name"`

	// SecurityGroups contains the security group IDs attached to the instance.
	SecurityGroups []string `json:"security_groups"`

	// Tags contains the instance's resource tags as key-value pairs.
	Tags map[string]string `json:"tags"`

	// RootBlockDevice contains the root volume configuration.
	RootBlockDevice BlockDevice `json:"root_block_device"`

	// EBSOptimized indicates if EBS optimization is enabled.
	EBSOptimized bool `json:"ebs_optimized"`

	// Monitoring indicates if detailed monitoring is enabled.
	Monitoring bool `json:"monitoring"`

	// IAMInstanceProfile is the ARN of the IAM instance profile attached.
	IAMInstanceProfile string `json:"iam_instance_profile"`
}

// BlockDevice represents an EBS block device configuration.
// It captures the essential volume attributes that can drift between
// AWS and Terraform configurations.
type BlockDevice struct {
	// VolumeSize is the size of the volume in GiB.
	VolumeSize int `json:"volume_size"`

	// VolumeType is the EBS volume type (e.g., "gp2", "gp3", "io1").
	VolumeType string `json:"volume_type"`

	// DeleteOnTermination indicates if the volume is deleted when the instance terminates.
	DeleteOnTermination bool `json:"delete_on_termination"`

	// Encrypted indicates if the volume is encrypted.
	Encrypted bool `json:"encrypted"`

	// IOPS is the provisioned IOPS for io1/io2/gp3 volumes.
	IOPS int `json:"iops"`

	// Throughput is the provisioned throughput in MiB/s for gp3 volumes.
	Throughput int `json:"throughput"`
}

// DriftResult contains the results of a drift detection comparison for a single instance.
// It indicates whether drift was detected and provides details about which attributes
// have different values between AWS and Terraform.
type DriftResult struct {
	// InstanceID is the EC2 instance ID that was checked.
	InstanceID string `json:"instance_id"`

	// HasDrift indicates whether any configuration drift was detected.
	HasDrift bool `json:"has_drift"`

	// DriftedAttrs contains details about each attribute that has drifted.
	// Empty if HasDrift is false.
	DriftedAttrs []DriftedAttr `json:"drifted_attributes,omitempty"`

	// Error contains any error message if the check failed.
	// This may be set even if HasDrift is true (e.g., instance not in TF state).
	Error string `json:"error,omitempty"`
}

// DriftedAttr represents a single attribute that has drifted.
// It captures the attribute path and both the AWS and Terraform values
// for easy comparison and reporting.
type DriftedAttr struct {
	// Path is the attribute path (e.g., "instance_type", "root_block_device.volume_size").
	Path string `json:"path"`

	// AWSValue is the current value from AWS.
	AWSValue any `json:"aws_value"`

	// TerraformValue is the expected value from Terraform configuration.
	TerraformValue any `json:"terraform_value"`
}

// DriftReport contains the complete drift detection report for multiple instances.
// It provides summary statistics and detailed results for each instance checked.
type DriftReport struct {
	// TotalInstances is the total number of instances checked.
	TotalInstances int `json:"total_instances"`

	// DriftedInstances is the count of instances with detected drift.
	DriftedInstances int `json:"drifted_instances"`

	// Results contains the detailed drift result for each instance.
	Results []DriftResult `json:"results"`
}

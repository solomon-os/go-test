// Package aws provides functionality to interact with AWS EC2 service.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/solomon-os/go-test/internal/models"
)

// EC2Client defines the interface for EC2 operations.
type EC2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// Client wraps the AWS EC2 client with helper methods.
type Client struct {
	ec2Client EC2Client
}

// NewClient creates a new AWS EC2 client with default configuration.
func NewClient(ctx context.Context, region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		ec2Client: ec2.NewFromConfig(cfg),
	}, nil
}

// NewClientWithEC2 creates a new Client with a custom EC2 client (useful for testing).
func NewClientWithEC2(client EC2Client) *Client {
	return &Client{ec2Client: client}
}

// GetInstance retrieves EC2 instance configuration by instance ID.
func (c *Client) GetInstance(ctx context.Context, instanceID string) (*models.EC2Instance, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	output, err := c.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	return convertEC2Instance(&output.Reservations[0].Instances[0]), nil
}

// GetInstances retrieves multiple EC2 instances by their IDs.
func (c *Client) GetInstances(ctx context.Context, instanceIDs []string) ([]*models.EC2Instance, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	}

	output, err := c.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []*models.EC2Instance
	for _, reservation := range output.Reservations {
		for i := range reservation.Instances {
			instances = append(instances, convertEC2Instance(&reservation.Instances[i]))
		}
	}

	return instances, nil
}

func convertEC2Instance(instance *types.Instance) *models.EC2Instance {
	ec2Inst := &models.EC2Instance{
		InstanceID:     derefString(instance.InstanceId),
		InstanceType:   string(instance.InstanceType),
		AMI:            derefString(instance.ImageId),
		SubnetID:       derefString(instance.SubnetId),
		VpcID:          derefString(instance.VpcId),
		PrivateIP:      derefString(instance.PrivateIpAddress),
		PublicIP:       derefString(instance.PublicIpAddress),
		KeyName:        derefString(instance.KeyName),
		EBSOptimized:   derefBool(instance.EbsOptimized),
		Tags:           make(map[string]string),
		SecurityGroups: make([]string, 0),
	}

	if instance.Placement != nil {
		ec2Inst.AvailabilityZone = derefString(instance.Placement.AvailabilityZone)
	}

	if instance.Monitoring != nil {
		ec2Inst.Monitoring = instance.Monitoring.State == types.MonitoringStateEnabled
	}

	if instance.IamInstanceProfile != nil {
		ec2Inst.IAMInstanceProfile = derefString(instance.IamInstanceProfile.Arn)
	}

	for _, sg := range instance.SecurityGroups {
		if sg.GroupId != nil {
			ec2Inst.SecurityGroups = append(ec2Inst.SecurityGroups, *sg.GroupId)
		}
	}

	for _, tag := range instance.Tags {
		if tag.Key != nil && tag.Value != nil {
			ec2Inst.Tags[*tag.Key] = *tag.Value
		}
	}

	for _, bdm := range instance.BlockDeviceMappings {
		if bdm.DeviceName != nil && *bdm.DeviceName == derefString(instance.RootDeviceName) {
			if bdm.Ebs != nil {
				ec2Inst.RootBlockDevice = models.BlockDevice{
					DeleteOnTermination: derefBool(bdm.Ebs.DeleteOnTermination),
				}
			}
			break
		}
	}

	return ec2Inst
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

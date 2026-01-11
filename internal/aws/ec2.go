// Package aws provides functionality to interact with AWS EC2 service.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/solomon-os/go-test/internal/logger"
	"github.com/solomon-os/go-test/internal/models"
)

// EC2Client defines the interface for EC2 operations.
type EC2Client interface {
	DescribeInstances(
		ctx context.Context,
		params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeInstancesOutput, error)
}

// Client wraps the AWS EC2 client with helper methods.
type Client struct {
	ec2Client EC2Client
}

func NewClient(ctx context.Context, region string) (*Client, error) {
	logger.Debug("creating AWS client", "region", region)
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		logger.Error("failed to load AWS config", "error", err, "region", region)
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	logger.Info("AWS client created successfully", "region", region)
	return &Client{
		ec2Client: ec2.NewFromConfig(cfg),
	}, nil
}

func NewClientWithEC2(client EC2Client) *Client {
	return &Client{ec2Client: client}
}

func (c *Client) GetInstance(ctx context.Context, instanceID string) (*models.EC2Instance, error) {
	logger.Debug("fetching EC2 instance", "instance_id", instanceID)
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	output, err := c.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		logger.Error("failed to describe instance", "instance_id", instanceID, "error", err)
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		logger.Warn("instance not found", "instance_id", instanceID)
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	logger.Debug("successfully fetched EC2 instance", "instance_id", instanceID)
	return convertEC2Instance(&output.Reservations[0].Instances[0]), nil
}

func (c *Client) GetInstances(
	ctx context.Context,
	instanceIDs []string,
) ([]*models.EC2Instance, error) {
	logger.Debug("fetching multiple EC2 instances", "count", len(instanceIDs))
	input := &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	}

	output, err := c.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		logger.Error("failed to describe instances", "error", err, "count", len(instanceIDs))
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []*models.EC2Instance
	for _, reservation := range output.Reservations {
		for i := range reservation.Instances {
			instances = append(instances, convertEC2Instance(&reservation.Instances[i]))
		}
	}

	logger.Info("fetched EC2 instances", "requested", len(instanceIDs), "returned", len(instances))
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

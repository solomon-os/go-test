// Package aws provides functionality to interact with AWS EC2 service.
//
// This package wraps the AWS SDK v2 for EC2 operations, providing a simplified
// interface for fetching EC2 instance details. It includes built-in retry logic
// with exponential backoff for handling transient AWS API failures.
//
// Example usage:
//
//	client, err := aws.NewClient(ctx, "us-east-1")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	instance, err := client.GetInstance(ctx, "i-1234567890abcdef0")
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/solomon-os/go-test/internal/logger"
	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/retry"
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
// It includes built-in retry logic for handling transient AWS API failures.
type Client struct {
	ec2Client   EC2Client
	retryConfig retry.Config
}

// NewClient creates a new AWS EC2 client with the specified region.
// It uses the default AWS credential chain to authenticate.
func NewClient(ctx context.Context, region string, opts ...ClientOption) (*Client, error) {
	logger.Debug("creating AWS client", "region", region)

	options := &clientOptions{
		retryConfig: retry.AWSConfig.WithShouldRetry(IsRetryableError),
	}
	for _, opt := range opts {
		opt(options)
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		logger.Error("failed to load AWS config", "error", err, "region", region)
		return nil, NewAWSError("LoadDefaultConfig", err)
	}

	logger.Info("AWS client created successfully", "region", region)
	return &Client{
		ec2Client:   ec2.NewFromConfig(cfg),
		retryConfig: options.retryConfig,
	}, nil
}

// NewClientWithEC2 creates a Client with a custom EC2 client implementation.
// This is primarily used for testing with mock clients.
func NewClientWithEC2(client EC2Client) *Client {
	return &Client{
		ec2Client:   client,
		retryConfig: retry.AWSConfig.WithShouldRetry(IsRetryableError),
	}
}

// NewClientWithEC2AndRetry creates a Client with a custom EC2 client and retry config.
func NewClientWithEC2AndRetry(client EC2Client, retryConfig retry.Config) *Client {
	return &Client{
		ec2Client:   client,
		retryConfig: retryConfig,
	}
}

// GetInstance retrieves a single EC2 instance by its ID.
// It includes automatic retry logic for transient AWS API failures.
func (c *Client) GetInstance(ctx context.Context, instanceID string) (*models.EC2Instance, error) {
	logger.Debug("fetching EC2 instance", "instance_id", instanceID)

	return retry.Do(ctx, c.retryConfig, func(ctx context.Context) (*models.EC2Instance, error) {
		input := &ec2.DescribeInstancesInput{
			InstanceIds: []string{instanceID},
		}

		output, err := c.ec2Client.DescribeInstances(ctx, input)
		if err != nil {
			logger.Warn("AWS API call failed, may retry",
				"instance_id", instanceID,
				"error", err,
				"retryable", IsRetryableError(err))
			return nil, NewAWSError("DescribeInstances", err, WithInstanceID(instanceID))
		}

		if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
			logger.Warn("instance not found", "instance_id", instanceID)
			return nil, NewAWSError("DescribeInstances",
				fmt.Errorf("instance not found"),
				WithInstanceID(instanceID))
		}

		logger.Debug("successfully fetched EC2 instance", "instance_id", instanceID)
		return convertEC2Instance(&output.Reservations[0].Instances[0]), nil
	})
}

// GetInstances retrieves multiple EC2 instances by their IDs.
// It includes automatic retry logic for transient AWS API failures.
func (c *Client) GetInstances(
	ctx context.Context,
	instanceIDs []string,
) ([]*models.EC2Instance, error) {
	logger.Debug("fetching multiple EC2 instances", "count", len(instanceIDs))

	return retry.Do(ctx, c.retryConfig, func(ctx context.Context) ([]*models.EC2Instance, error) {
		input := &ec2.DescribeInstancesInput{
			InstanceIds: instanceIDs,
		}

		output, err := c.ec2Client.DescribeInstances(ctx, input)
		if err != nil {
			logger.Warn("AWS API call failed, may retry",
				"count", len(instanceIDs),
				"error", err,
				"retryable", IsRetryableError(err))
			return nil, NewAWSError("DescribeInstances", err)
		}

		var instances []*models.EC2Instance
		for _, reservation := range output.Reservations {
			for i := range reservation.Instances {
				instances = append(instances, convertEC2Instance(&reservation.Instances[i]))
			}
		}

		logger.Info(
			"fetched EC2 instances",
			"requested",
			len(instanceIDs),
			"returned",
			len(instances),
		)
		return instances, nil
	})
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

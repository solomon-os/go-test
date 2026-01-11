package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockEC2Client implements EC2Client for testing.
type mockEC2Client struct {
	DescribeInstancesFunc func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func (m *mockEC2Client) DescribeInstances(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	return m.DescribeInstancesFunc(ctx, params, optFns...)
}

func TestNewClientWithEC2(t *testing.T) {
	mock := &mockEC2Client{}
	client := NewClientWithEC2(mock)

	if client == nil {
		t.Fatal("NewClientWithEC2() returned nil")
	}
	if client.ec2Client == nil {
		t.Fatal("ec2Client is nil")
	}
}

func TestClient_GetInstance(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		mockOutput *ec2.DescribeInstancesOutput
		mockErr    error
		wantErr    bool
		wantType   string
	}{
		{
			name:       "successful retrieval",
			instanceID: "i-123456",
			mockOutput: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:   aws.String("i-123456"),
								InstanceType: types.InstanceTypeT2Micro,
								ImageId:      aws.String("ami-abc123"),
								SubnetId:     aws.String("subnet-123"),
								VpcId:        aws.String("vpc-456"),
								KeyName:      aws.String("my-key"),
								EbsOptimized: aws.Bool(true),
								Placement: &types.Placement{
									AvailabilityZone: aws.String("us-east-1a"),
								},
								Monitoring: &types.Monitoring{
									State: types.MonitoringStateEnabled,
								},
								SecurityGroups: []types.GroupIdentifier{
									{GroupId: aws.String("sg-111")},
									{GroupId: aws.String("sg-222")},
								},
								Tags: []types.Tag{
									{Key: aws.String("Name"), Value: aws.String("test-instance")},
									{Key: aws.String("Environment"), Value: aws.String("dev")},
								},
								IamInstanceProfile: &types.IamInstanceProfile{
									Arn: aws.String("arn:aws:iam::123456789:instance-profile/role"),
								},
							},
						},
					},
				},
			},
			wantErr:  false,
			wantType: "t2.micro",
		},
		{
			name:       "instance not found - empty reservations",
			instanceID: "i-notfound",
			mockOutput: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{},
			},
			wantErr: true,
		},
		{
			name:       "instance not found - empty instances",
			instanceID: "i-notfound",
			mockOutput: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{Instances: []types.Instance{}},
				},
			},
			wantErr: true,
		},
		{
			name:       "API error",
			instanceID: "i-error",
			mockErr:    errors.New("API error"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockEC2Client{
				DescribeInstancesFunc: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					return tt.mockOutput, nil
				},
			}

			client := NewClientWithEC2(mock)
			instance, err := client.GetInstance(context.Background(), tt.instanceID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetInstance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && instance.InstanceType != tt.wantType {
				t.Errorf("InstanceType = %s, want %s", instance.InstanceType, tt.wantType)
			}
		})
	}
}

func TestClient_GetInstance_FullMapping(t *testing.T) {
	mock := &mockEC2Client{
		DescribeInstancesFunc: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-full"),
								InstanceType:     types.InstanceTypeT3Medium,
								ImageId:          aws.String("ami-full123"),
								SubnetId:         aws.String("subnet-full"),
								VpcId:            aws.String("vpc-full"),
								PrivateIpAddress: aws.String("10.0.0.100"),
								PublicIpAddress:  aws.String("54.1.2.3"),
								KeyName:          aws.String("full-key"),
								EbsOptimized:     aws.Bool(true),
								Placement: &types.Placement{
									AvailabilityZone: aws.String("us-west-2b"),
								},
								Monitoring: &types.Monitoring{
									State: types.MonitoringStateEnabled,
								},
								SecurityGroups: []types.GroupIdentifier{
									{GroupId: aws.String("sg-a")},
									{GroupId: aws.String("sg-b")},
								},
								Tags: []types.Tag{
									{Key: aws.String("Name"), Value: aws.String("full-test")},
								},
								IamInstanceProfile: &types.IamInstanceProfile{
									Arn: aws.String("arn:aws:iam::123:instance-profile/test"),
								},
								RootDeviceName: aws.String("/dev/xvda"),
								BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
									{
										DeviceName: aws.String("/dev/xvda"),
										Ebs: &types.EbsInstanceBlockDevice{
											DeleteOnTermination: aws.Bool(true),
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	client := NewClientWithEC2(mock)
	instance, err := client.GetInstance(context.Background(), "i-full")
	if err != nil {
		t.Fatalf("GetInstance() error = %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"InstanceID", instance.InstanceID, "i-full"},
		{"InstanceType", instance.InstanceType, "t3.medium"},
		{"AMI", instance.AMI, "ami-full123"},
		{"SubnetID", instance.SubnetID, "subnet-full"},
		{"VpcID", instance.VpcID, "vpc-full"},
		{"PrivateIP", instance.PrivateIP, "10.0.0.100"},
		{"PublicIP", instance.PublicIP, "54.1.2.3"},
		{"KeyName", instance.KeyName, "full-key"},
		{"EBSOptimized", instance.EBSOptimized, true},
		{"AvailabilityZone", instance.AvailabilityZone, "us-west-2b"},
		{"Monitoring", instance.Monitoring, true},
		{"SecurityGroups count", len(instance.SecurityGroups), 2},
		{"Tags count", len(instance.Tags), 1},
		{
			"IAMInstanceProfile",
			instance.IAMInstanceProfile,
			"arn:aws:iam::123:instance-profile/test",
		},
		{"RootBlockDevice.DeleteOnTermination", instance.RootBlockDevice.DeleteOnTermination, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestClient_GetInstances(t *testing.T) {
	tests := []struct {
		name        string
		instanceIDs []string
		mockOutput  *ec2.DescribeInstancesOutput
		mockErr     error
		wantCount   int
		wantErr     bool
	}{
		{
			name:        "multiple instances",
			instanceIDs: []string{"i-1", "i-2", "i-3"},
			mockOutput: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:   aws.String("i-1"),
								InstanceType: types.InstanceTypeT2Micro,
							},
							{
								InstanceId:   aws.String("i-2"),
								InstanceType: types.InstanceTypeT2Small,
							},
						},
					},
					{
						Instances: []types.Instance{
							{
								InstanceId:   aws.String("i-3"),
								InstanceType: types.InstanceTypeT2Medium,
							},
						},
					},
				},
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:        "no instances found",
			instanceIDs: []string{"i-notfound"},
			mockOutput: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{},
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:        "API error",
			instanceIDs: []string{"i-1"},
			mockErr:     errors.New("API error"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockEC2Client{
				DescribeInstancesFunc: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					return tt.mockOutput, nil
				},
			}

			client := NewClientWithEC2(mock)
			instances, err := client.GetInstances(context.Background(), tt.instanceIDs)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetInstances() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(instances) != tt.wantCount {
				t.Errorf(
					"GetInstances() returned %d instances, want %d",
					len(instances),
					tt.wantCount,
				)
			}
		})
	}
}

func TestConvertEC2Instance_NilFields(t *testing.T) {
	// Test handling of nil fields
	instance := types.Instance{
		InstanceId:   aws.String("i-nil"),
		InstanceType: types.InstanceTypeT2Micro,
		// All other fields nil
	}

	result := convertEC2Instance(&instance)

	if result.InstanceID != "i-nil" {
		t.Errorf("InstanceID = %s, want i-nil", result.InstanceID)
	}
	if result.AvailabilityZone != "" {
		t.Errorf("AvailabilityZone should be empty, got %s", result.AvailabilityZone)
	}
	if result.Monitoring != false {
		t.Error("Monitoring should be false when nil")
	}
	if len(result.SecurityGroups) != 0 {
		t.Error("SecurityGroups should be empty")
	}
	if len(result.Tags) != 0 {
		t.Error("Tags should be empty")
	}
}

func TestDerefString(t *testing.T) {
	tests := []struct {
		name  string
		input *string
		want  string
	}{
		{"nil pointer", nil, ""},
		{"non-nil pointer", aws.String("test"), "test"},
		{"empty string pointer", aws.String(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := derefString(tt.input); got != tt.want {
				t.Errorf("derefString() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDerefBool(t *testing.T) {
	tests := []struct {
		name  string
		input *bool
		want  bool
	}{
		{"nil pointer", nil, false},
		{"true pointer", aws.Bool(true), true},
		{"false pointer", aws.Bool(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := derefBool(tt.input); got != tt.want {
				t.Errorf("derefBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertEC2Instance_MonitoringStates(t *testing.T) {
	tests := []struct {
		name  string
		state types.MonitoringState
		want  bool
	}{
		{"enabled", types.MonitoringStateEnabled, true},
		{"disabled", types.MonitoringStateDisabled, false},
		{"pending", types.MonitoringStatePending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := types.Instance{
				InstanceId: aws.String("i-test"),
				Monitoring: &types.Monitoring{State: tt.state},
			}
			result := convertEC2Instance(&instance)
			if result.Monitoring != tt.want {
				t.Errorf("Monitoring = %v, want %v", result.Monitoring, tt.want)
			}
		})
	}
}

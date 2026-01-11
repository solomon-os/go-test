package terraform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Error("NewParser returned nil")
	}
}

func TestParser_ParseStateJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid state with one instance",
			json: `{
				"version": 4,
				"resources": [
					{
						"type": "aws_instance",
						"name": "web",
						"provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
						"instances": [
							{
								"attributes": {
									"id": "i-123456",
									"ami": "ami-0123456789",
									"instance_type": "t2.micro",
									"availability_zone": "us-east-1a",
									"subnet_id": "subnet-123",
									"vpc_security_group_ids": ["sg-123", "sg-456"],
									"key_name": "my-key",
									"ebs_optimized": false,
									"monitoring": true,
									"tags": {"Name": "web-server"},
									"root_block_device": [
										{
											"volume_size": 50,
											"volume_type": "gp2",
											"delete_on_termination": true,
											"encrypted": false
										}
									]
								}
							}
						]
					}
				]
			}`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "valid state with multiple instances",
			json: `{
				"version": 4,
				"resources": [
					{
						"type": "aws_instance",
						"name": "web",
						"instances": [
							{"attributes": {"id": "i-111", "instance_type": "t2.micro"}},
							{"attributes": {"id": "i-222", "instance_type": "t2.small"}}
						]
					},
					{
						"type": "aws_instance",
						"name": "api",
						"instances": [
							{"attributes": {"id": "i-333", "instance_type": "t2.large"}}
						]
					}
				]
			}`,
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "state with non-instance resources",
			json: `{
				"version": 4,
				"resources": [
					{
						"type": "aws_s3_bucket",
						"name": "bucket",
						"instances": [{"attributes": {"id": "my-bucket"}}]
					},
					{
						"type": "aws_instance",
						"name": "web",
						"instances": [{"attributes": {"id": "i-123", "instance_type": "t2.micro"}}]
					}
				]
			}`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "empty state",
			json: `{
				"version": 4,
				"resources": []
			}`,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "invalid JSON",
			json:      `{invalid}`,
			wantCount: 0,
			wantErr:   true,
		},
	}

	p := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instances, err := p.ParseStateJSON([]byte(tt.json))

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStateJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(instances) != tt.wantCount {
				t.Errorf("ParseStateJSON() returned %d instances, want %d", len(instances), tt.wantCount)
			}
		})
	}
}

func TestParser_ParseStateJSON_AttributeMapping(t *testing.T) {
	json := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "web",
				"instances": [
					{
						"attributes": {
							"id": "i-123456",
							"ami": "ami-abc123",
							"instance_type": "t3.medium",
							"availability_zone": "us-west-2a",
							"subnet_id": "subnet-xyz",
							"vpc_security_group_ids": ["sg-111", "sg-222"],
							"key_name": "production-key",
							"private_ip": "10.0.1.100",
							"public_ip": "54.32.10.1",
							"ebs_optimized": true,
							"monitoring": true,
							"iam_instance_profile": "arn:aws:iam::123456789:instance-profile/role",
							"tags": {"Name": "production", "Environment": "prod"},
							"root_block_device": [
								{
									"volume_size": 100,
									"volume_type": "gp3",
									"delete_on_termination": true,
									"encrypted": true,
									"iops": 3000,
									"throughput": 125
								}
							]
						}
					}
				]
			}
		]
	}`

	p := NewParser()
	instances, err := p.ParseStateJSON([]byte(json))
	if err != nil {
		t.Fatalf("ParseStateJSON() error = %v", err)
	}

	inst, ok := instances["i-123456"]
	if !ok {
		t.Fatal("Instance i-123456 not found")
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"InstanceID", inst.InstanceID, "i-123456"},
		{"AMI", inst.AMI, "ami-abc123"},
		{"InstanceType", inst.InstanceType, "t3.medium"},
		{"AvailabilityZone", inst.AvailabilityZone, "us-west-2a"},
		{"SubnetID", inst.SubnetID, "subnet-xyz"},
		{"KeyName", inst.KeyName, "production-key"},
		{"PrivateIP", inst.PrivateIP, "10.0.1.100"},
		{"PublicIP", inst.PublicIP, "54.32.10.1"},
		{"EBSOptimized", inst.EBSOptimized, true},
		{"Monitoring", inst.Monitoring, true},
		{"IAMInstanceProfile", inst.IAMInstanceProfile, "arn:aws:iam::123456789:instance-profile/role"},
		{"SecurityGroups count", len(inst.SecurityGroups), 2},
		{"Tags count", len(inst.Tags), 2},
		{"RootBlockDevice.VolumeSize", inst.RootBlockDevice.VolumeSize, 100},
		{"RootBlockDevice.VolumeType", inst.RootBlockDevice.VolumeType, "gp3"},
		{"RootBlockDevice.Encrypted", inst.RootBlockDevice.Encrypted, true},
		{"RootBlockDevice.IOPS", inst.RootBlockDevice.IOPS, 3000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestParser_ParseStateFile(t *testing.T) {
	// Create a temporary state file
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "terraform.tfstate")

	stateContent := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "test",
				"instances": [
					{"attributes": {"id": "i-test123", "instance_type": "t2.micro"}}
				]
			}
		]
	}`

	if err := os.WriteFile(statePath, []byte(stateContent), 0644); err != nil {
		t.Fatalf("Failed to create temp state file: %v", err)
	}

	p := NewParser()
	instances, err := p.ParseStateFile(statePath)
	if err != nil {
		t.Fatalf("ParseStateFile() error = %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("ParseStateFile() returned %d instances, want 1", len(instances))
	}

	if _, ok := instances["i-test123"]; !ok {
		t.Error("Instance i-test123 not found")
	}
}

func TestParser_ParseStateFile_NotFound(t *testing.T) {
	p := NewParser()
	_, err := p.ParseStateFile("/nonexistent/path/terraform.tfstate")
	if err == nil {
		t.Error("ParseStateFile() expected error for nonexistent file")
	}
}

func TestParser_ParseFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  bool
	}{
		{
			name:     "tfstate extension",
			filename: "terraform.tfstate",
			content:  `{"version": 4, "resources": []}`,
			wantErr:  false,
		},
		{
			name:     "json extension",
			filename: "state.json",
			content:  `{"version": 4, "resources": []}`,
			wantErr:  false,
		},
		{
			name:     "unsupported extension",
			filename: "config.txt",
			content:  `some content`,
			wantErr:  true,
		},
	}

	p := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			_, err := p.ParseFile(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParser_GetInstanceByID(t *testing.T) {
	p := NewParser()
	instances := map[string]*struct {
		ID string
	}{} // Using different type to simulate

	// Test with actual EC2Instance map
	json := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "test",
				"instances": [
					{"attributes": {"id": "i-123", "instance_type": "t2.micro"}},
					{"attributes": {"id": "i-456", "instance_type": "t2.small"}}
				]
			}
		]
	}`

	ec2Instances, _ := p.ParseStateJSON([]byte(json))

	t.Run("found", func(t *testing.T) {
		inst, err := p.GetInstanceByID(ec2Instances, "i-123")
		if err != nil {
			t.Errorf("GetInstanceByID() error = %v", err)
		}
		if inst == nil {
			t.Fatal("GetInstanceByID() returned nil instance")
		}
		if inst.InstanceID != "i-123" {
			t.Fatalf("GetInstanceByID() InstanceID = %s, want i-123", inst.InstanceID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := p.GetInstanceByID(ec2Instances, "i-nonexistent")
		if err == nil {
			t.Error("GetInstanceByID() expected error for nonexistent instance")
		}
	})

	_ = instances // Suppress unused variable warning
}

func TestParser_TagsAllFallback(t *testing.T) {
	json := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_instance",
				"name": "test",
				"instances": [
					{
						"attributes": {
							"id": "i-123",
							"instance_type": "t2.micro",
							"tags": {},
							"tags_all": {"Name": "from-tags-all", "Managed": "terraform"}
						}
					}
				]
			}
		]
	}`

	p := NewParser()
	instances, err := p.ParseStateJSON([]byte(json))
	if err != nil {
		t.Fatalf("ParseStateJSON() error = %v", err)
	}

	inst := instances["i-123"]
	if len(inst.Tags) != 2 {
		t.Errorf("Expected 2 tags from tags_all, got %d", len(inst.Tags))
	}
	if inst.Tags["Name"] != "from-tags-all" {
		t.Errorf("Expected Name tag to be 'from-tags-all', got '%s'", inst.Tags["Name"])
	}
}

func TestParser_SecurityGroupsFallback(t *testing.T) {
	t.Run("prefer vpc_security_group_ids", func(t *testing.T) {
		json := `{
			"version": 4,
			"resources": [
				{
					"type": "aws_instance",
					"name": "test",
					"instances": [
						{
							"attributes": {
								"id": "i-123",
								"vpc_security_group_ids": ["sg-vpc-1", "sg-vpc-2"],
								"security_groups": ["sg-classic-1"]
							}
						}
					]
				}
			]
		}`

		p := NewParser()
		instances, _ := p.ParseStateJSON([]byte(json))
		inst := instances["i-123"]

		if len(inst.SecurityGroups) != 2 {
			t.Errorf("Expected 2 security groups, got %d", len(inst.SecurityGroups))
		}
		if inst.SecurityGroups[0] != "sg-vpc-1" {
			t.Errorf("Expected first SG to be 'sg-vpc-1', got '%s'", inst.SecurityGroups[0])
		}
	})

	t.Run("fallback to security_groups", func(t *testing.T) {
		json := `{
			"version": 4,
			"resources": [
				{
					"type": "aws_instance",
					"name": "test",
					"instances": [
						{
							"attributes": {
								"id": "i-123",
								"vpc_security_group_ids": [],
								"security_groups": ["sg-classic-1", "sg-classic-2"]
							}
						}
					]
				}
			]
		}`

		p := NewParser()
		instances, _ := p.ParseStateJSON([]byte(json))
		inst := instances["i-123"]

		if len(inst.SecurityGroups) != 2 {
			t.Errorf("Expected 2 security groups, got %d", len(inst.SecurityGroups))
		}
	})
}

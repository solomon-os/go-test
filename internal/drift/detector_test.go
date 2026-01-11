package drift

import (
	"context"
	"testing"

	"github.com/solomon-os/go-test/internal/models"
)

func TestNewDetector(t *testing.T) {
	t.Run("with default attributes", func(t *testing.T) {
		d := NewDetector(nil)
		if len(d.attributes) == 0 {
			t.Error("expected default attributes to be set")
		}
		if len(d.attributes) != len(DefaultAttributes) {
			t.Errorf("expected %d attributes, got %d", len(DefaultAttributes), len(d.attributes))
		}
	})

	t.Run("with custom attributes", func(t *testing.T) {
		attrs := []string{"instance_type", "ami"}
		d := NewDetector(attrs)
		if len(d.attributes) != 2 {
			t.Errorf("expected 2 attributes, got %d", len(d.attributes))
		}
	})
}

func TestDetector_Detect(t *testing.T) {
	tests := []struct {
		name       string
		aws        *models.EC2Instance
		tf         *models.EC2Instance
		attributes []string
		wantDrift  bool
		wantAttrs  int
	}{
		{
			name: "no drift - identical instances",
			aws: &models.EC2Instance{
				InstanceID:   "i-123",
				InstanceType: "t2.micro",
				AMI:          "ami-123",
			},
			tf: &models.EC2Instance{
				InstanceID:   "i-123",
				InstanceType: "t2.micro",
				AMI:          "ami-123",
			},
			attributes: []string{"instance_type", "ami"},
			wantDrift:  false,
			wantAttrs:  0,
		},
		{
			name: "drift detected - instance type changed",
			aws: &models.EC2Instance{
				InstanceID:   "i-123",
				InstanceType: "t2.large",
				AMI:          "ami-123",
			},
			tf: &models.EC2Instance{
				InstanceID:   "i-123",
				InstanceType: "t2.micro",
				AMI:          "ami-123",
			},
			attributes: []string{"instance_type", "ami"},
			wantDrift:  true,
			wantAttrs:  1,
		},
		{
			name: "drift detected - multiple attributes",
			aws: &models.EC2Instance{
				InstanceID:   "i-123",
				InstanceType: "t2.large",
				AMI:          "ami-456",
			},
			tf: &models.EC2Instance{
				InstanceID:   "i-123",
				InstanceType: "t2.micro",
				AMI:          "ami-123",
			},
			attributes: []string{"instance_type", "ami"},
			wantDrift:  true,
			wantAttrs:  2,
		},
		{
			name: "drift detected - security groups",
			aws: &models.EC2Instance{
				InstanceID:     "i-123",
				SecurityGroups: []string{"sg-123", "sg-456"},
			},
			tf: &models.EC2Instance{
				InstanceID:     "i-123",
				SecurityGroups: []string{"sg-123"},
			},
			attributes: []string{"security_groups"},
			wantDrift:  true,
			wantAttrs:  1,
		},
		{
			name: "no drift - security groups same order different",
			aws: &models.EC2Instance{
				InstanceID:     "i-123",
				SecurityGroups: []string{"sg-456", "sg-123"},
			},
			tf: &models.EC2Instance{
				InstanceID:     "i-123",
				SecurityGroups: []string{"sg-123", "sg-456"},
			},
			attributes: []string{"security_groups"},
			wantDrift:  false,
			wantAttrs:  0,
		},
		{
			name: "drift detected - tags",
			aws: &models.EC2Instance{
				InstanceID: "i-123",
				Tags:       map[string]string{"Name": "prod", "Env": "production"},
			},
			tf: &models.EC2Instance{
				InstanceID: "i-123",
				Tags:       map[string]string{"Name": "prod"},
			},
			attributes: []string{"tags"},
			wantDrift:  true,
			wantAttrs:  1,
		},
		{
			name: "no drift - tags identical",
			aws: &models.EC2Instance{
				InstanceID: "i-123",
				Tags:       map[string]string{"Name": "prod"},
			},
			tf: &models.EC2Instance{
				InstanceID: "i-123",
				Tags:       map[string]string{"Name": "prod"},
			},
			attributes: []string{"tags"},
			wantDrift:  false,
			wantAttrs:  0,
		},
		{
			name: "drift detected - nested root block device",
			aws: &models.EC2Instance{
				InstanceID: "i-123",
				RootBlockDevice: models.BlockDevice{
					VolumeSize: 100,
					VolumeType: "gp3",
				},
			},
			tf: &models.EC2Instance{
				InstanceID: "i-123",
				RootBlockDevice: models.BlockDevice{
					VolumeSize: 50,
					VolumeType: "gp2",
				},
			},
			attributes: []string{"root_block_device.volume_size", "root_block_device.volume_type"},
			wantDrift:  true,
			wantAttrs:  2,
		},
		{
			name: "drift detected - boolean attribute",
			aws: &models.EC2Instance{
				InstanceID:   "i-123",
				EBSOptimized: true,
			},
			tf: &models.EC2Instance{
				InstanceID:   "i-123",
				EBSOptimized: false,
			},
			attributes: []string{"ebs_optimized"},
			wantDrift:  true,
			wantAttrs:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDetector(tt.attributes)
			result := d.Detect(tt.aws, tt.tf)

			if result.HasDrift != tt.wantDrift {
				t.Errorf("HasDrift = %v, want %v", result.HasDrift, tt.wantDrift)
			}

			if len(result.DriftedAttrs) != tt.wantAttrs {
				t.Errorf("DriftedAttrs count = %d, want %d", len(result.DriftedAttrs), tt.wantAttrs)
			}

			if result.InstanceID != tt.aws.InstanceID {
				t.Errorf("InstanceID = %s, want %s", result.InstanceID, tt.aws.InstanceID)
			}
		})
	}
}

func TestDetector_DetectMultiple(t *testing.T) {
	awsInstances := map[string]*models.EC2Instance{
		"i-123": {
			InstanceID:   "i-123",
			InstanceType: "t2.large", // drifted
		},
		"i-456": {
			InstanceID:   "i-456",
			InstanceType: "t2.micro", // no drift
		},
		"i-789": {
			InstanceID:   "i-789",
			InstanceType: "t2.small",
		},
	}

	tfInstances := map[string]*models.EC2Instance{
		"i-123": {
			InstanceID:   "i-123",
			InstanceType: "t2.micro",
		},
		"i-456": {
			InstanceID:   "i-456",
			InstanceType: "t2.micro",
		},
		// i-789 missing from TF - should show drift
	}

	d := NewDetector([]string{"instance_type"})
	ctx := context.Background()
	report := d.DetectMultiple(ctx, awsInstances, tfInstances)

	if report.TotalInstances != 3 {
		t.Errorf("TotalInstances = %d, want 3", report.TotalInstances)
	}

	if report.DriftedInstances != 2 {
		t.Errorf("DriftedInstances = %d, want 2", report.DriftedInstances)
	}

	if len(report.Results) != 3 {
		t.Errorf("Results count = %d, want 3", len(report.Results))
	}
}

func TestDetector_DetectMultiple_ContextCancelled(t *testing.T) {
	awsInstances := map[string]*models.EC2Instance{
		"i-123": {InstanceID: "i-123", InstanceType: "t2.micro"},
	}
	tfInstances := map[string]*models.EC2Instance{
		"i-123": {InstanceID: "i-123", InstanceType: "t2.micro"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	d := NewDetector([]string{"instance_type"})
	report := d.DetectMultiple(ctx, awsInstances, tfInstances)

	// Should still get a result (either canceled or completed before cancellation)
	if len(report.Results) != 1 {
		t.Errorf("Results count = %d, want 1", len(report.Results))
	}
}

func TestDetector_valuesEqual(t *testing.T) {
	d := NewDetector(nil)

	tests := []struct {
		name string
		a    any
		b    any
		want bool
	}{
		{"nil values", nil, nil, true},
		{"one nil", "test", nil, false},
		{"equal strings", "test", "test", true},
		{"different strings", "test1", "test2", false},
		{"equal ints", 100, 100, true},
		{"different ints", 100, 200, false},
		{"equal bools", true, true, true},
		{"different bools", true, false, false},
		{"empty slices", []string{}, []string{}, true},
		{"equal slices", []string{"a", "b"}, []string{"a", "b"}, true},
		{"slices different order", []string{"b", "a"}, []string{"a", "b"}, true},
		{"different slices", []string{"a"}, []string{"b"}, false},
		{"equal maps", map[string]string{"a": "1"}, map[string]string{"a": "1"}, true},
		{"different maps", map[string]string{"a": "1"}, map[string]string{"b": "2"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := d.valuesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("valuesEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_extractValue(t *testing.T) {
	d := NewDetector(nil)
	instance := &models.EC2Instance{
		InstanceID:   "i-123",
		InstanceType: "t2.micro",
		AMI:          "ami-123",
		Tags:         map[string]string{"Name": "test"},
		RootBlockDevice: models.BlockDevice{
			VolumeSize: 100,
			VolumeType: "gp3",
		},
	}

	tests := []struct {
		name    string
		path    []string
		want    any
		wantErr bool
	}{
		{"instance_type", []string{"instance_type"}, "t2.micro", false},
		{"ami", []string{"ami"}, "ami-123", false},
		{"tags", []string{"tags"}, map[string]string{"Name": "test"}, false},
		{"specific tag", []string{"tags", "Name"}, "test", false},
		{"root_block_device", []string{"root_block_device"}, instance.RootBlockDevice, false},
		{"root_block_device.volume_size", []string{"root_block_device", "volume_size"}, 100, false},
		{"unknown attribute", []string{"unknown"}, nil, true},
		{"empty path", []string{}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.extractValue(instance, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !d.valuesEqual(got, tt.want) {
				t.Errorf("extractValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_GetAttributes(t *testing.T) {
	attrs := []string{"instance_type", "ami"}
	d := NewDetector(attrs)

	got := d.GetAttributes()
	if len(got) != len(attrs) {
		t.Errorf("GetAttributes() returned %d attributes, want %d", len(got), len(attrs))
	}
}

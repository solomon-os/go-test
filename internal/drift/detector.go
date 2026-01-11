// Package drift provides functionality to detect configuration drift between AWS and Terraform.
package drift

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/solomon-os/go-test/internal/logger"
	"github.com/solomon-os/go-test/internal/models"
)

var DefaultAttributes = []string{
	"instance_type",
	"ami",
	"availability_zone",
	"subnet_id",
	"security_groups",
	"tags",
	"key_name",
	"ebs_optimized",
	"monitoring",
	"iam_instance_profile",
	"root_block_device.volume_size",
	"root_block_device.volume_type",
	"root_block_device.encrypted",
}

// Detector defines the interface for drift detection operations.
type Detector interface {
	Detect(awsInstance, tfInstance *models.EC2Instance) *models.DriftResult
	DetectMultiple(ctx context.Context, awsInstances, tfInstances map[string]*models.EC2Instance) *models.DriftReport
	GetAttributes() []string
}

// DefaultDetector performs drift detection between AWS and Terraform configurations.
type DefaultDetector struct {
	attributes []string
}

func NewDetector(attributes []string) *DefaultDetector {
	if len(attributes) == 0 {
		attributes = DefaultAttributes
	}
	return &DefaultDetector{attributes: attributes}
}

func (d *DefaultDetector) Detect(awsInstance, tfInstance *models.EC2Instance) *models.DriftResult {
	logger.Debug("detecting drift for instance", "instance_id", awsInstance.InstanceID, "attributes", len(d.attributes))
	result := &models.DriftResult{
		InstanceID:   awsInstance.InstanceID,
		HasDrift:     false,
		DriftedAttrs: make([]models.DriftedAttr, 0),
	}

	for _, attr := range d.attributes {
		awsValue, tfValue, err := d.getAttributeValues(awsInstance, tfInstance, attr)
		if err != nil {
			logger.Debug("skipping attribute", "instance_id", awsInstance.InstanceID, "attribute", attr, "error", err)
			continue
		}

		if !d.valuesEqual(awsValue, tfValue) {
			logger.Debug("drift detected", "instance_id", awsInstance.InstanceID, "attribute", attr)
			result.HasDrift = true
			result.DriftedAttrs = append(result.DriftedAttrs, models.DriftedAttr{
				Path:           attr,
				AWSValue:       awsValue,
				TerraformValue: tfValue,
			})
		}
	}

	if result.HasDrift {
		logger.Info("drift detected", "instance_id", awsInstance.InstanceID, "drifted_attributes", len(result.DriftedAttrs))
	} else {
		logger.Debug("no drift detected", "instance_id", awsInstance.InstanceID)
	}

	return result
}

func (d *DefaultDetector) DetectMultiple(ctx context.Context, awsInstances, tfInstances map[string]*models.EC2Instance) *models.DriftReport {
	logger.Info("starting drift detection", "aws_instances", len(awsInstances), "tf_instances", len(tfInstances))
	report := &models.DriftReport{
		TotalInstances: len(awsInstances),
		Results:        make([]models.DriftResult, 0),
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results = make(chan models.DriftResult, len(awsInstances))
	)

	for instanceID, awsInst := range awsInstances {
		wg.Add(1)
		go func(id string, aws *models.EC2Instance) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				logger.Warn("context canceled during drift detection", "instance_id", id)
				results <- models.DriftResult{
					InstanceID: id,
					Error:      "context canceled",
				}
				return
			default:
			}

			tfInst, ok := tfInstances[id]
			if !ok {
				logger.Warn("instance not found in Terraform state", "instance_id", id)
				results <- models.DriftResult{
					InstanceID: id,
					HasDrift:   true,
					Error:      "instance not found in Terraform state",
				}
				return
			}

			result := d.Detect(aws, tfInst)
			results <- *result
		}(instanceID, awsInst)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		mu.Lock()
		report.Results = append(report.Results, result)
		if result.HasDrift {
			report.DriftedInstances++
		}
		mu.Unlock()
	}

	sort.Slice(report.Results, func(i, j int) bool {
		return report.Results[i].InstanceID < report.Results[j].InstanceID
	})

	logger.Info("drift detection complete", "total", report.TotalInstances, "drifted", report.DriftedInstances)

	return report
}

func (d *DefaultDetector) getAttributeValues(aws, tf *models.EC2Instance, attr string) (awsVal, tfVal interface{}, err error) {
	parts := strings.Split(attr, ".")

	awsValue, err := d.extractValue(aws, parts)
	if err != nil {
		return nil, nil, err
	}

	tfValue, err := d.extractValue(tf, parts)
	if err != nil {
		return nil, nil, err
	}

	return awsValue, tfValue, nil
}

func (d *DefaultDetector) extractValue(instance *models.EC2Instance, path []string) (interface{}, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	fieldMap := map[string]func(*models.EC2Instance) interface{}{
		"instance_type":        func(i *models.EC2Instance) interface{} { return i.InstanceType },
		"ami":                  func(i *models.EC2Instance) interface{} { return i.AMI },
		"availability_zone":    func(i *models.EC2Instance) interface{} { return i.AvailabilityZone },
		"subnet_id":            func(i *models.EC2Instance) interface{} { return i.SubnetID },
		"vpc_id":               func(i *models.EC2Instance) interface{} { return i.VpcID },
		"private_ip":           func(i *models.EC2Instance) interface{} { return i.PrivateIP },
		"public_ip":            func(i *models.EC2Instance) interface{} { return i.PublicIP },
		"key_name":             func(i *models.EC2Instance) interface{} { return i.KeyName },
		"security_groups":      func(i *models.EC2Instance) interface{} { return i.SecurityGroups },
		"tags":                 func(i *models.EC2Instance) interface{} { return i.Tags },
		"ebs_optimized":        func(i *models.EC2Instance) interface{} { return i.EBSOptimized },
		"monitoring":           func(i *models.EC2Instance) interface{} { return i.Monitoring },
		"iam_instance_profile": func(i *models.EC2Instance) interface{} { return i.IAMInstanceProfile },
	}

	if path[0] == "root_block_device" {
		if len(path) == 1 {
			return instance.RootBlockDevice, nil
		}
		return d.extractBlockDeviceValue(&instance.RootBlockDevice, path[1])
	}

	if path[0] == "tags" && len(path) > 1 {
		return instance.Tags[path[1]], nil
	}

	getter, ok := fieldMap[path[0]]
	if !ok {
		return nil, fmt.Errorf("unknown attribute: %s", path[0])
	}

	return getter(instance), nil
}

func (d *DefaultDetector) extractBlockDeviceValue(bd *models.BlockDevice, field string) (interface{}, error) {
	switch field {
	case "volume_size":
		return bd.VolumeSize, nil
	case "volume_type":
		return bd.VolumeType, nil
	case "delete_on_termination":
		return bd.DeleteOnTermination, nil
	case "encrypted":
		return bd.Encrypted, nil
	case "iops":
		return bd.IOPS, nil
	case "throughput":
		return bd.Throughput, nil
	default:
		return nil, fmt.Errorf("unknown block device attribute: %s", field)
	}
}

func (d *DefaultDetector) valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	if aVal.Kind() == reflect.Slice && bVal.Kind() == reflect.Slice {
		return d.slicesEqual(a, b)
	}

	if aVal.Kind() == reflect.Map && bVal.Kind() == reflect.Map {
		return d.mapsEqual(a, b)
	}

	return reflect.DeepEqual(a, b)
}

func (d *DefaultDetector) slicesEqual(a, b interface{}) bool {
	aSlice, ok := a.([]string)
	if !ok {
		return reflect.DeepEqual(a, b)
	}
	bSlice, ok := b.([]string)
	if !ok {
		return false
	}

	if len(aSlice) != len(bSlice) {
		return false
	}

	aSorted := make([]string, len(aSlice))
	bSorted := make([]string, len(bSlice))
	copy(aSorted, aSlice)
	copy(bSorted, bSlice)
	sort.Strings(aSorted)
	sort.Strings(bSorted)

	for i := range aSorted {
		if aSorted[i] != bSorted[i] {
			return false
		}
	}
	return true
}

func (d *DefaultDetector) mapsEqual(a, b interface{}) bool {
	aMap, ok := a.(map[string]string)
	if !ok {
		return reflect.DeepEqual(a, b)
	}
	bMap, ok := b.(map[string]string)
	if !ok {
		return false
	}

	if len(aMap) != len(bMap) {
		return false
	}

	for k, v := range aMap {
		if bMap[k] != v {
			return false
		}
	}
	return true
}

func (d *DefaultDetector) GetAttributes() []string {
	return d.attributes
}

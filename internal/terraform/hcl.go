package terraform

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"

	"github.com/solomon-os/go-test/internal/logger"
	"github.com/solomon-os/go-test/internal/models"
)

func (p *Parser) ParseHCLFile(filePath string) (map[string]*models.EC2Instance, error) {
	logger.Debug("reading HCL file", "path", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error("failed to read HCL file", "path", filePath, "error", err)
		return nil, fmt.Errorf("failed to read HCL file: %w", err)
	}

	return p.ParseHCL(data, filePath)
}

func (p *Parser) ParseHCL(data []byte, filename string) (map[string]*models.EC2Instance, error) {
	logger.Debug("parsing HCL content", "filename", filename, "bytes", len(data))
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(data, filename)
	if diags.HasErrors() {
		logger.Error("failed to parse HCL", "filename", filename, "error", diags.Error())
		return nil, fmt.Errorf("failed to parse HCL: %s", diags.Error())
	}

	instances := make(map[string]*models.EC2Instance)
	content, diags := file.Body.Content(terraformSchema)
	if diags.HasErrors() {
		logger.Error("failed to decode HCL content", "filename", filename, "error", diags.Error())
		return nil, fmt.Errorf("failed to decode HCL content: %s", diags.Error())
	}

	for _, block := range content.Blocks {
		if block.Type != "resource" {
			continue
		}

		if len(block.Labels) < 2 || block.Labels[0] != "aws_instance" {
			continue
		}

		resourceName := block.Labels[1]
		instance, err := p.parseHCLResource(block, resourceName)
		if err != nil {
			logger.Error("failed to parse HCL resource", "resource", resourceName, "error", err)
			return nil, fmt.Errorf("failed to parse resource %s: %w", resourceName, err)
		}

		if instance.InstanceID == "" {
			instance.InstanceID = resourceName
		}
		instances[instance.InstanceID] = instance
	}

	logger.Info("parsed HCL file", "filename", filename, "instance_count", len(instances))
	return instances, nil
}

var terraformSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "terraform"},
		{Type: "provider", LabelNames: []string{"name"}},
		{Type: "resource", LabelNames: []string{"type", "name"}},
		{Type: "data", LabelNames: []string{"type", "name"}},
		{Type: "variable", LabelNames: []string{"name"}},
		{Type: "output", LabelNames: []string{"name"}},
		{Type: "locals"},
		{Type: "module", LabelNames: []string{"name"}},
	},
}

var resourceSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "ami"},
		{Name: "instance_type"},
		{Name: "availability_zone"},
		{Name: "subnet_id"},
		{Name: "vpc_security_group_ids"},
		{Name: "security_groups"},
		{Name: "key_name"},
		{Name: "ebs_optimized"},
		{Name: "monitoring"},
		{Name: "iam_instance_profile"},
		{Name: "tags"},
	},
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "root_block_device"},
	},
}

var rootBlockDeviceSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "volume_size"},
		{Name: "volume_type"},
		{Name: "delete_on_termination"},
		{Name: "encrypted"},
		{Name: "iops"},
		{Name: "throughput"},
	},
}

func (p *Parser) parseHCLResource(block *hcl.Block, name string) (*models.EC2Instance, error) {
	content, diags := block.Body.Content(resourceSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode resource: %s", diags.Error())
	}

	instance := &models.EC2Instance{
		InstanceID:     name,
		Tags:           make(map[string]string),
		SecurityGroups: make([]string, 0),
	}

	ctx := &hcl.EvalContext{}
	p.applyHCLAttributes(instance, content.Attributes, ctx)

	for _, blk := range content.Blocks {
		if blk.Type == "root_block_device" {
			rbd, err := p.parseRootBlockDevice(blk)
			if err != nil {
				return nil, err
			}
			instance.RootBlockDevice = rbd
		}
	}

	return instance, nil
}

func (p *Parser) applyHCLAttributes(instance *models.EC2Instance, attrs hcl.Attributes, ctx *hcl.EvalContext) {
	for attrName, attr := range attrs {
		val, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			continue
		}
		p.setInstanceAttribute(instance, attrName, val)
	}
}

func (p *Parser) setInstanceAttribute(instance *models.EC2Instance, name string, val cty.Value) {
	switch name {
	case "ami":
		instance.AMI = valueToString(val)
	case "instance_type":
		instance.InstanceType = valueToString(val)
	case "availability_zone":
		instance.AvailabilityZone = valueToString(val)
	case "subnet_id":
		instance.SubnetID = valueToString(val)
	case "key_name":
		instance.KeyName = valueToString(val)
	case "ebs_optimized":
		instance.EBSOptimized = valueToBool(val)
	case "monitoring":
		instance.Monitoring = valueToBool(val)
	case "iam_instance_profile":
		instance.IAMInstanceProfile = valueToString(val)
	case "vpc_security_group_ids", "security_groups":
		instance.SecurityGroups = valueToStringSlice(val)
	case "tags":
		instance.Tags = valueToStringMap(val)
	}
}

func (p *Parser) parseRootBlockDevice(block *hcl.Block) (models.BlockDevice, error) {
	content, diags := block.Body.Content(rootBlockDeviceSchema)
	if diags.HasErrors() {
		return models.BlockDevice{}, fmt.Errorf("failed to decode root_block_device: %s", diags.Error())
	}

	bd := models.BlockDevice{}
	ctx := &hcl.EvalContext{}

	for attrName, attr := range content.Attributes {
		val, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			continue
		}

		switch attrName {
		case "volume_size":
			bd.VolumeSize = valueToInt(val)
		case "volume_type":
			bd.VolumeType = valueToString(val)
		case "delete_on_termination":
			bd.DeleteOnTermination = valueToBool(val)
		case "encrypted":
			bd.Encrypted = valueToBool(val)
		case "iops":
			bd.IOPS = valueToInt(val)
		case "throughput":
			bd.Throughput = valueToInt(val)
		}
	}

	return bd, nil
}

func valueToString(val cty.Value) string {
	if val.IsNull() || !val.IsKnown() {
		return ""
	}
	if val.Type() != cty.String {
		return ""
	}
	return val.AsString()
}

func valueToBool(val cty.Value) bool {
	if val.IsNull() || !val.IsKnown() {
		return false
	}
	if val.Type() != cty.Bool {
		return false
	}
	return val.True()
}

func valueToInt(val cty.Value) int {
	if val.IsNull() || !val.IsKnown() {
		return 0
	}
	if val.Type() != cty.Number {
		return 0
	}
	n, _ := val.AsBigFloat().Int64()
	return int(n)
}

func valueToStringSlice(val cty.Value) []string {
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	if !val.Type().IsListType() && !val.Type().IsTupleType() && !val.Type().IsSetType() {
		return nil
	}

	result := make([]string, 0)
	for it := val.ElementIterator(); it.Next(); {
		_, v := it.Element()
		if v.Type() == cty.String {
			result = append(result, v.AsString())
		}
	}
	return result
}

func valueToStringMap(val cty.Value) map[string]string {
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	if !val.Type().IsMapType() && !val.Type().IsObjectType() {
		return nil
	}

	result := make(map[string]string)
	for it := val.ElementIterator(); it.Next(); {
		k, v := it.Element()
		if k.Type() == cty.String && v.Type() == cty.String {
			result[k.AsString()] = v.AsString()
		}
	}
	return result
}

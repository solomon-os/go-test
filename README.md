# EC2 Drift Detector

A Go application that detects infrastructure drift between AWS EC2 instances and their Terraform configurations. It compares the actual EC2 instance configuration in AWS with the expected configuration defined in Terraform and reports any differences.

## Features

- **Drift Detection**: Compares AWS EC2 instances against Terraform state files
- **Multiple Attribute Support**: Checks instance type, AMI, security groups, tags, and more
- **Nested Field Comparison**: Supports nested attributes like `root_block_device.volume_size`
- **Concurrent Processing**: Handles multiple instances concurrently using Go's concurrency primitives
- **Multiple Output Formats**: Supports JSON, table, and human-readable text output
- **Flexible CLI**: Specify which attributes to check and which instances to scan

## Project Structure

```
.
├── cmd/
│   └── drift-detector/
│       └── main.go          # CLI entry point
├── internal/
│   ├── aws/
│   │   ├── ec2.go           # AWS EC2 client
│   │   └── ec2_test.go
│   ├── drift/
│   │   ├── detector.go      # Drift detection engine
│   │   └── detector_test.go
│   ├── models/
│   │   └── instance.go      # Data models
│   ├── reporter/
│   │   ├── reporter.go      # Output formatting
│   │   └── reporter_test.go
│   └── terraform/
│       ├── parser.go        # Terraform state parser
│       └── parser_test.go
├── testdata/
│   ├── terraform.tfstate    # Sample Terraform state
│   ├── aws_ec2_response.json # Sample AWS response
│   └── main.tf              # Sample Terraform config
├── go.mod
├── go.sum
└── README.md
```

## Requirements

- Go 1.21 or later
- AWS credentials configured (for live AWS queries)
- Terraform state file (`.tfstate` or `.json`)

## Installation

```bash
# Clone the repository
git clone https://github.com/solomon-os/go-test.git
cd go-test

# Install dependencies
go mod tidy

# Build the application
go build -o drift-detector ./cmd/drift-detector
```

## Usage

### Basic Usage

```bash
# Check all instances in a Terraform state file
./drift-detector --tf-state terraform.tfstate --region us-east-1

# Check specific instances
./drift-detector --tf-state terraform.tfstate -i i-123456,i-789012

# Output in JSON format
./drift-detector --tf-state terraform.tfstate -o json

# Output in table format
./drift-detector --tf-state terraform.tfstate -o table
```

### Check Specific Attributes

```bash
# Only check instance type and AMI
./drift-detector --tf-state terraform.tfstate -a instance_type,ami

# Check nested attributes
./drift-detector --tf-state terraform.tfstate -a root_block_device.volume_size,root_block_device.encrypted
```

### Single Instance Detection

```bash
./drift-detector detect i-0abc123def456789a --tf-state terraform.tfstate
```

### List Available Attributes

```bash
./drift-detector list-attributes
```

### Command Line Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--tf-state` | `-t` | Path to Terraform state file | (required) |
| `--region` | `-r` | AWS region | us-east-1 |
| `--instances` | `-i` | Instance IDs to check (comma-separated) | all in state |
| `--attributes` | `-a` | Attributes to check (comma-separated) | all default |
| `--output` | `-o` | Output format: text, table, json | text |
| `--timeout` | | Timeout for AWS API calls | 30s |

## Supported Attributes

The following attributes can be checked for drift:

| Attribute | Description |
|-----------|-------------|
| `instance_type` | EC2 instance type (e.g., t2.micro) |
| `ami` | Amazon Machine Image ID |
| `availability_zone` | Availability zone |
| `subnet_id` | Subnet ID |
| `security_groups` | List of security group IDs |
| `tags` | Instance tags (key-value pairs) |
| `key_name` | SSH key pair name |
| `ebs_optimized` | EBS optimization status |
| `monitoring` | Detailed monitoring status |
| `iam_instance_profile` | IAM instance profile ARN |
| `root_block_device.volume_size` | Root volume size in GB |
| `root_block_device.volume_type` | Root volume type (gp2, gp3, io1, etc.) |
| `root_block_device.encrypted` | Root volume encryption status |

## Sample Output

### Text Format
```
EC2 Drift Detection Report
==========================

Instance: i-0abc123def456789a
  Status: DRIFT DETECTED
  Drifted Attributes:
    - instance_type:
        AWS:       t2.small
        Terraform: t2.micro
    - tags:
        AWS:       {Name=web-server-01, Environment=production, Team=platform, CostCenter=12345}
        Terraform: {Name=web-server-01, Environment=production, Team=platform}

Instance: i-0def456789abc123b
  Status: No drift detected

Summary
-------
Total instances checked: 2
Instances with drift:    1
Instances without drift: 1
```

### Table Format
```
INSTANCE ID              DRIFT DETECTED  DRIFTED ATTRIBUTES
-----------              --------------  ------------------
i-0abc123def456789a      Yes             instance_type, tags
i-0def456789abc123b      No              -

Summary: 1/2 instances with drift
```

### JSON Format
```json
{
  "total_instances": 2,
  "drifted_instances": 1,
  "results": [
    {
      "instance_id": "i-0abc123def456789a",
      "has_drift": true,
      "drifted_attributes": [
        {
          "path": "instance_type",
          "aws_value": "t2.small",
          "terraform_value": "t2.micro"
        }
      ]
    }
  ]
}
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run tests with verbose output
go test -v ./...
```

## Design Decisions

### Architecture

1. **Modular Design**: The application is split into distinct packages (`aws`, `terraform`, `drift`, `reporter`) for separation of concerns and testability.

2. **Interface-Based AWS Client**: The AWS EC2 client is defined as an interface, allowing easy mocking in tests without requiring actual AWS credentials.

3. **Concurrent Processing**: Multiple instances are processed concurrently using goroutines and channels, improving performance for large-scale drift detection.

4. **Flexible Attribute System**: Attributes are specified as dot-notation paths (e.g., `root_block_device.volume_size`), supporting both top-level and nested fields.

### Trade-offs

1. **State File vs HCL Parsing**: The current implementation focuses on Terraform state files (JSON format) rather than raw HCL. This approach:
   - Provides accurate representation of deployed resources
   - Is simpler to implement and more reliable
   - May miss resources not yet applied

2. **Order-Independent Comparison**: Security groups and tags are compared without considering order, which is the correct behavior for these AWS resources but may not be desired in all cases.

3. **Shallow Block Device Comparison**: Only root block device is compared; additional EBS volumes are not currently supported.

## Future Improvements

1. **HCL Parsing**: Add support for parsing `.tf` files directly using the HashiCorp HCL library

2. **Additional Resources**: Extend support to other AWS resources (RDS, S3, Lambda, etc.)

3. **Historical Tracking**: Store drift detection results over time for trend analysis

4. **Remediation Suggestions**: Generate Terraform code to fix detected drift

5. **CI/CD Integration**: Add GitHub Actions workflow for automated drift detection

6. **Webhook Notifications**: Send alerts to Slack, PagerDuty, etc. when drift is detected

7. **AWS Organizations Support**: Scan multiple AWS accounts concurrently

8. **Custom Comparators**: Allow users to define custom comparison logic for specific attributes

## License

MIT License

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

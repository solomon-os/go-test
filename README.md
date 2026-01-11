# EC2 Drift Detector

A Go application that detects infrastructure drift between AWS EC2 instances and their Terraform configurations. It compares the actual EC2 instance configuration in AWS with the expected configuration defined in Terraform and reports any differences.

## Features

- **Drift Detection**: Compares AWS EC2 instances against Terraform state/HCL files
- **Multiple Attribute Support**: Checks instance type, AMI, security groups, tags, and more
- **Nested Field Comparison**: Supports nested attributes like `root_block_device.volume_size`
- **Concurrent Processing**: Handles multiple instances concurrently using Go's concurrency primitives
- **Multiple Output Formats**: Supports JSON, table, and human-readable text output
- **HCL & State File Support**: Parses both `.tfstate` and `.tf` files
- **Environment Variable Support**: Loads AWS credentials from `.env` file

## Project Structure

```
.
├── cmd/
│   └── drift-detector/
│       └── main.go              # CLI entry point
├── internal/
│   ├── aws/
│   │   ├── ec2.go               # AWS EC2 client
│   │   └── ec2_test.go
│   ├── cli/
│   │   ├── cli.go               # CLI commands
│   │   └── cli_test.go
│   ├── drift/
│   │   ├── detector.go          # Drift detection engine
│   │   └── detector_test.go
│   ├── logger/
│   │   └── logger.go            # Structured logging
│   ├── models/
│   │   └── instance.go          # Data models
│   ├── reporter/
│   │   ├── reporter.go          # Output formatting
│   │   └── reporter_test.go
│   └── terraform/
│       ├── parser.go            # Terraform state parser
│       ├── parser_test.go
│       ├── hcl.go               # HCL (.tf) parser
│       └── hcl_test.go
├── testdata/
│   ├── terraform.tfstate        # Sample Terraform state
│   ├── aws_ec2_response.json    # Sample AWS response
│   └── main.tf                  # Sample Terraform config
├── .env.example                 # Example environment variables
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

## Requirements

- Go 1.21 or later
- AWS credentials with `ec2:DescribeInstances` permission
- Terraform state file (`.tfstate`) or HCL file (`.tf`)

## Installation

```bash
git clone https://github.com/solomon-os/go-test.git
cd go-test

make build
```

## AWS Setup

### Required IAM Permission

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "ec2:DescribeInstances",
            "Resource": "*"
        }
    ]
}
```

### Setup Steps

1. Create IAM user in AWS Console
2. Attach policy with `ec2:DescribeInstances` permission
3. Generate access keys

### Configure Credentials

Create a `.env` file in the project root:

```bash
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=your-secret-key
```

Or use AWS CLI:

```bash
aws configure
```

## Usage

### Basic Usage

```bash
# Check all instances in a Terraform state file
./main --tf-state terraform.tfstate --region us-east-1

# Check using HCL file
./main --tf-state main.tf --region us-east-1

# Check specific instances
./main --tf-state terraform.tfstate -i i-123456,i-789012

# Output in JSON format
./main --tf-state terraform.tfstate -o json

# Output in table format
./main --tf-state terraform.tfstate -o table
```

### Check Specific Attributes

```bash
# Only check instance type and AMI
./main --tf-state terraform.tfstate -a instance_type,ami

# Check nested attributes
./main --tf-state terraform.tfstate -a root_block_device.volume_size,root_block_device.encrypted
```

### Single Instance Detection

```bash
./main detect i-0abc123def456789a --tf-state terraform.tfstate
```

### List Available Attributes

```bash
./main list-attributes
```

### Command Line Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--tf-state` | `-t` | Path to Terraform state or HCL file | (required) |
| `--region` | `-r` | AWS region | us-east-1 |
| `--instances` | `-i` | Instance IDs to check (comma-separated) | all in state |
| `--attributes` | `-a` | Attributes to check (comma-separated) | all default |
| `--output` | `-o` | Output format: text, table, json | text |
| `--timeout` | | Timeout for AWS API calls | 30s |

## Supported Attributes

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

Instance: i-0def456789abc123b
  Status: No drift detected

Summary
-------
Total instances checked: 2
Instances with drift:    1
Instances without drift: 1
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
make test     # Run all tests
make cover    # Run tests with coverage report
make lint     # Run linter
make clean    # Clean build artifacts
```

### Test Coverage

| Package | Coverage |
|---------|----------|
| internal/aws | 87.5% |
| internal/cli | 93.9% |
| internal/drift | 84.2% |
| internal/reporter | 100% |
| internal/terraform | 85.8% |
| **Total** | **86.2%** |

## Design Decisions

### Architecture

1. **Modular Design**: The application is split into distinct packages (`aws`, `terraform`, `drift`, `reporter`, `logger`) for separation of concerns and testability.

2. **Interface-Based Design**: AWS client, parser, detector, and reporter are defined as interfaces, allowing easy mocking in tests.

3. **Concurrent Processing**: Multiple instances are processed concurrently using goroutines and channels.

4. **Structured Logging**: Uses Go's `log/slog` for structured, leveled logging.

5. **Environment Variable Support**: Uses `godotenv` to load `.env` files automatically.

### Trade-offs

1. **Order-Independent Comparison**: Security groups and tags are compared without considering order, which is correct for AWS resources.

2. **Shallow Block Device Comparison**: Only root block device is compared; additional EBS volumes are not currently supported.

## License

MIT License

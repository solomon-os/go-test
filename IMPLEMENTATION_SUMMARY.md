# What I Built

## 1. Better Error Handling

Made errors more useful - now they tell you what went wrong, where, and if you should retry.

- `internal/errors/errors.go` - the base stuff
- `internal/aws/errors.go` - AWS errors like rate limits and access denied
- `internal/terraform/errors.go` - parsing errors
- `internal/drift/errors.go` - drift detection errors

## 2. Limited Goroutines

Before, the code spawned unlimited goroutines which could crash things. Now it uses a worker pool that limits how many run at once.

- `internal/worker/pool.go` - the worker pool
- `internal/drift/detector.go` - now uses the pool

## 3. Retry Logic

AWS calls fail sometimes. Now they automatically retry with exponential backoff instead of just dying.

- `internal/retry/retry.go` - generic retry logic
- `internal/aws/ec2.go` - wrapped the API calls

## 4. Documentation

Added godoc comments to all the new code so people know what things do.

## 5. Repository Pattern

Separated data fetching from business logic. Makes testing way easier.

- `internal/repository/interfaces.go` - the contracts
- `internal/repository/aws/ec2_repository.go` - gets data from AWS
- `internal/repository/terraform/repository.go` - gets data from tfstate

## 6. Extensible Registries

Want to add a new comparator or output format? Just register it. No need to touch existing code.

- `internal/drift/comparator/comparator.go` - compare values different ways
- `internal/reporter/formatter/formatter.go` - output as JSON, table, text, etc.

## 7. Factory Pattern

One place to create all the components. Also has a builder for injecting mocks in tests.

- `internal/factory/factory.go` - creates everything
- `internal/factory/container.go` - for testing with mocks

## Tests

All the new stuff has tests:

- `internal/errors/errors_test.go`
- `internal/retry/retry_test.go`
- `internal/worker/pool_test.go`
- `internal/drift/comparator/comparator_test.go`
- `internal/reporter/formatter/formatter_test.go`
- `internal/repository/interfaces_test.go`
- `internal/repository/aws/ec2_repository_test.go`
- `internal/repository/terraform/repository_test.go`
- `internal/factory/factory_test.go`

Run them with `go test ./...`

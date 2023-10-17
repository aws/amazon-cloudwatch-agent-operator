# Validator
This validator is a version of the ADOT test framework validator fitted to the needs of the pulse E2E tests.
It validates the metrics and traces that come out of the application after pulse has been enabled.

## Run
### Run as a command

Run the following command in the root directory of the repository to run the pulse metric and trace validations

```shell
./gradlew :testing:validator:run --args='-c pulse-validation.yml --endpoint <app-endpoint> --region us-east-1 --account-id 417921506511 --metric-namespace AWS/APM --rollup'
```

Help

```shell
./gradlew :testing:validator:run --args='-h'
```

## Add a validation suite

1. add a config file under `resources/validations`
2. add an expected data under `resources/expected-data-template`
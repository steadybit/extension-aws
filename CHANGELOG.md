# Changelog

## v2.1.0

- Support Readiness & Liveness probes (requires helm chart version >= 2.0.0)
- Refactored to use `action_kit_sdk` and thus use the extended rollback safety while having connection issues
- Added Lambda discovery & actions (requires new permissions)

## v2.0.0

- Renamed attack `ec2-instance.state` to `com.github.steadybit.extension_aws.ec2_instance.state`
- Added EC2-Instance discovery
- Added Zone-Discovery and Availability Zone Blackhole attack
- Added AWS FIS-Experiment discovery and AWS FIS-Experiment action

## v1.8.0

- Print build information on extension startup.

## v1.7.1

 - Add missing `kind` field to both actions.

## v1.7.0

 - Support creation of a TLS server through the environment variables `STEADYBIT_EXTENSION_TLS_SERVER_CERT` and `STEADYBIT_EXTENSION_TLS_SERVER_KEY`. Both environment variables must refer to files containing the certificate and key in PEM format.
 - Support mutual TLS through the environment variable `STEADYBIT_EXTENSION_TLS_CLIENT_CAS`. The environment must refer to a comma-separated list of files containing allowed clients' CA certificates in PEM format. 

## v1.6.0

 - Support for AWS role assumption. This permits one extension instance from gathering data from multiple AWS accounts. To configure this, you must set the `STEADYBIT_EXTENSION_ASSUME_ROLES` environment variable to a comma-separated list of role ARNs. Example: `STEADYBIT_EXTENSION_ASSUME_ROLES='arn:aws:iam::1111111111:role/steadybit-extension-aws,arn:aws:iam::22222222:role/steadybit-extension-aws'`.

## v1.5.0

 - Support for the `STEADYBIT_LOG_FORMAT` env variable. When set to `json`, extensions will log JSON lines to stderr.

## v1.4.0

 - Restrict discovery execution to AWS agents to avoid common issues.
 - The log level can now be configured through the `STEADYBIT_LOG_LEVEL` environment variable. 

## v1.3.0

 - Expose AWS RDS instance status in target table

## v1.2.0

 - Report AWS RDS instance status

## v1.1.0

 - EC2 instance state attacks, i.e., EC2 instance stop, reboot, hibernate and terminate.

## v1.0.0

 - Initial release
# Changelog

## Unreleased

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
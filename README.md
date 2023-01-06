<img src="./logo.jpeg" height="130" align="right" alt="Amazon Web Services (AWS) logo depicting five orange colored boxes and the text Amazon Web Services">

# Steadybit extension-aws

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject HTTP faults into various AWS services.

## Capabilities

 - Amazon Elastic Cloud Compute (EC2)
     - Attacks
         - State change of EC2 instances, e.g., stop, hibernate, terminate and reboot. 
 - Amazon Relational Database Service (RDS)
   - Discoveries
     - RDS instances
   - Attacks
     - Reboot of RDS instances

## Configuration

The process requires valid access credentials to interact with various AWS APIs.

### When Deployed in AWS EKS 

#### IAM Policy

Create an IAM policy called `steadybit-extension-aws` with the following content.
You can optionally restrict for which resources the extension may become active
by tweaking the `Resource` clause.

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "rds:RebootDBCluster",
        "rds:ListTagsForResource",
        "rds:RebootDBInstance",
        "rds:DescribeDBInstances",
        "rds:DescribeDBClusters"
      ],
      "Resource": "*"
    },

    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances",
        "ec2:DescribeTags",
        "ec2:StopInstances",
        "ec2:RebootInstances",
        "ec2:TerminateInstances"
      ],
      "Resource": "*"
    }
  ]
}
```

#### IAM Role

An IAM role is necessary that the AWS extension Kubernetes pod can assume in order to interact
with the AWS APIs.

```yaml
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Principal": {
                "Federated": "arn:aws:iam::{{AWS account number}:oidc-provider/{{OpenID Connect provider URL without scheme}}"
            },
            "Condition": {
                "StringEquals": {
                    "{{OpenID Connect provider URL without scheme}}:aud": [
                        "sts.amazonaws.com"
                    ],
                    "{{OpenID Connect provider URL without scheme}}:sub": [
                      "system:serviceaccount:{{Kubernetes namespace}}:{{Kubernetes service account name}}"
                    ]
                }
            }
        }
    ]
}
```

### Assume Role into Multiple AWS Accounts

By default, the extension uses the provided credentials to discover all resources within the belonging AWS account. To interact with multiple AWS accounts using a single extension, you can instruct the extension only to use the provided credentials to assume roles (using AWS STS) into given role-ARNs (and thus to possibly other AWS accounts).

To achieve this, you must set the STEADYBIT_EXTENSION_ASSUME_ROLES environment variable to a comma-separated list of role-ARNs. Example:

```sh
STEADYBIT_EXTENSION_ASSUME_ROLES='arn:aws:iam::1111111111:role/steadybit-extension-aws,arn:aws:iam::22222222:role/steadybit-extension-aws'
```

#### Necessary AWS Configuration

IAM policies need to be correctly configured for cross-account role assumption. In a nutshell, these are the necessary steps:

 1. The credentials provided to the extension are allowed to assume the provided role-ARNs.
    ```json
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": "sts:AssumeRole",
                "Resource": "arn:aws:iam::<TARGET_ACCOUNT>:role/<ROLE_IN_TARGET_ACCOUNT>"
            }
        ]
    }
    ```
 2. The roles themselves have all the [required permissions](#iam-policy).
 3. The roles have trust relationships that allow them to be assumed by the given credentials.
    ```json
    {
        "Version": "2012-10-17",
        "Statement": [
            {
               "Effect": "Allow",
               "Principal": {
                   "AWS": "arn:aws:iam::<SOURCE_ACCOUNT>:<SOURCE_ROLE>"
               },
               "Action": "sts:AssumeRole",
               "Condition": {}
            }
        ]
    }
    ```

## Deployment

We recommend that you deploy the extension with our [official Helm chart](https://github.com/steadybit/helm-charts/tree/main/charts/steadybit-extension-aws).

## Agent Configuration

The Steadybit AWS agent needs to be configured to interact with the AWS extension by adding the following environment variables:

```shell
# Make sure to adapt the URLs and indices in the environment variables names as necessary for your setup

STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_URL=http://steadybit-extension-aws.steadybit-extension.svc.cluster.local:8085
STEADYBIT_AGENT_DISCOVERIES_EXTENSIONS_0_URL=http://steadybit-extension-aws.steadybit-extension.svc.cluster.local:8085
```

When leveraging our official Helm charts, you can set the configuration through additional environment variables on the agent:

```
--set agent.env[0].name=STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_URL \
--set agent.env[0].value="http://steadybit-extension-aws.steadybit-extension.svc.cluster.local:8085" \
--set agent.env[1].name=STEADYBIT_AGENT_DISCOVERIES_EXTENSIONS_0_URL \
--set agent.env[1].value="http://steadybit-extension-aws.steadybit-extension.svc.cluster.local:8085"
```

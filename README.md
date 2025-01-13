<img src="./logo.jpeg" height="130" align="right" alt="Amazon Web Services (AWS) logo depicting five orange colored boxes and the text Amazon Web Services">

# Steadybit extension-aws

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject faults into various AWS
services.

Learn about the capabilities of this extension in
our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_aws).

## Configuration

| Environment Variable                                            | Helm value                                      | Meaning                                                                                                                                                       | Required | Default                                                                                                                                       |
|-----------------------------------------------------------------|-------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| `STEADYBIT_EXTENSION_WORKER_THREADS`                            |                                                 | How many parallel workers should call aws apis (only used if `STEADYBIT_EXTENSION_ASSUME_ROLES` is used)                                                      | no       | 1                                                                                                                                             |
| `STEADYBIT_EXTENSION_ASSUME_ROLES`                              | `aws.assumeRoles`                               | See detailed description below                                                                                                                                | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_REGIONS`                                   | `aws.regions`                                   | See detailed description below                                                                                                                                | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EC2`                    | `aws.discovery.disabled.ec2`                    | Disable EC2-Discovery and all related definitions                                                                                                             | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_EC2`                    |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 30                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ECS`                    | `aws.discovery.disabled.ecs`                    | Disable ECS-Discovery and all related definitions                                                                                                             | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_ECS_TASK`               |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 30                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_ECS_SERVICE`            |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 30                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ELASTICACHE`            | `aws.discovery.disabled.elasticache`            | Disable Elasticache-Discovery and all related definitions                                                                                                     | no       | true                                                                                                                                          |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_ELASTICACHE`            |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 30                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ELB`                    | `aws.discovery.disabled.elb`                    | Disable ELB-Discovery and all related definitions                                                                                                             | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_ELB_ALB`                |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 30                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_FIS`                    | `aws.discovery.disabled.fis`                    | Disable FIS-Discovery and all related definitions                                                                                                             | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_FIS`                    |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 300                                                                                                                                           |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_MSK`                    | `aws.discovery.disabled.msk`                    | Disable MSK-Discovery and all related definitions                                                                                                             | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_MSK`                    |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 30                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_LAMBDA`                 | `aws.discovery.disabled.lambda`                 | Disable Lambda-Discovery and all related definitions                                                                                                          | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_LAMBDA`                 |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 60                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_RDS`                    | `aws.discovery.disabled.rds`                    | Disable RDS-Discovery and all related definitions                                                                                                             | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_RDS`                    |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 30                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_SUBNET`                 | `aws.discovery.disabled.subnet`                 | Disable Subnet-Discovery and all related definitions                                                                                                          | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_SUBNET`                 |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 30                                                                                                                                            |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_VPC`                    | `aws.discovery.disabled.vpc`                    | Disable VPC-Discovery and all related definitions                                                                                                             | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ZONE`                   | `aws.discovery.disabled.zone`                   | Disable Zone-Discovery and all related definitions                                                                                                            | no       | false                                                                                                                                         |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL_ZONE`                   |                                                 | Discovery-Interval in seconds                                                                                                                                 | no       | 300                                                                                                                                           |
| `STEADYBIT_EXTENSION_ENRICH_EC2_DATA_FOR_TARGET_TYPES`          |                                                 | These target types will be enriched with EC2 data. They must have the attribute specified by 'STEADYBIT_EXTENSION_ENRICH_EC2_DATA_MATCHER_ATTRIBUTE' for this | no       | com.steadybit.extension_jvm.jvm-instance,com.steadybit.extension_container.container,com.steadybit.extension_kubernetes.kubernetes-deployment |
| `STEADYBIT_EXTENSION_ENRICH_EC2_DATA_MATCHER_ATTRIBUTE`         |                                                 | Targets for EC2 Data enrichment will be matched by this attribute.                                                                                            | no       | host.hostname                                                                                                                                 |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_EC2`         | `aws.discovery.attributes.excludes.ec2`         | List of EC2 Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                    | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_ECS`         | `aws.discovery.attributes.excludes.ecs`         | List of ECS Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                    | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_ELASTICACHE` | `aws.discovery.attributes.excludes.elasticache` | List of Elasticache Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                            | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_ELB`         | `aws.discovery.attributes.excludes.elb`         | List of ELB Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                    | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_FIS`         | `aws.discovery.attributes.excludes.fis`         | List of FIS Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                    | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_MSK`         | `aws.discovery.attributes.excludes.msk`         | List of MSK Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                    | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_LAMBDA`      | `aws.discovery.attributes.excludes.lambda`      | List of Lambda Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                 | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_RDS`         | `aws.discovery.attributes.excludes.rds`         | List of RDS Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                    | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_SUBNET`      | `aws.discovery.attributes.excludes.subnet`      | List of Subnet Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                 | no       |                                                                                                                                               |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_ZONE`        | `aws.discovery.attributes.excludes.zone`        | List of Availibilty Zone Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                       | no       |                                                                                                                                               |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

When installed as linux package this configuration is in`/etc/steadybit/extension-aws`.

### Authentication

The process requires valid access credentials to interact with various AWS APIs.

#### Required permissions (policies)

The extension needs an IAM Role with the given permissions. You can optionally restrict for which resources the extension may
become active
by tweaking the `Resource` clause.

<details>
    <summary>Availability Zone-Discovery & Availability Zone Blackhole</summary>

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ec2:DescribeTags",
        "ec2:DescribeAvailabilityZones",
        "ec2:DescribeSubnets",
        "ec2:DescribeNetworkAcls",
        "ec2:DescribeVpcs",
        "ec2:CreateNetworkAcl",
        "ec2:CreateNetworkAclEntry",
        "ec2:ReplaceNetworkAclAssociation",
        "ec2:DeleteNetworkAcl",
        "ec2:CreateTags"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
```

</details>
<details>
    <summary>EC2-Discovery & Actions</summary>

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances",
        "ec2:DescribeTags",
        "ec2:StopInstances",
        "ec2:RebootInstances",
        "ec2:TerminateInstances",
				"ec2:StartInstances",
      ],
      "Resource": "*"
    }
  ]
}
```

</details>
<details>
    <summary>ECS-Discovery & Actions</summary><a id='ecs-permissions'></a>


Depends on what you want to do on ECS, you might need an additional setup.

If only the action of Stopping the task in ECS is sufficient, then you need to add these permissions.

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecs:ListTasks",
        "ecs:DescribeTasks",
        "ecs:ListClusters",
        "ecs:StopTask",
        "ecs:UpdateService",
        "ecs:ListServices",
        "ecs:DescribeServices"
      ],
      "Resource": "*"
    }
  ]
}
```
If you want to perform the stress CPU/memory/io or fill disk attacks on ECS Tasks you need to include the following permission as well, and then you can proceed with the installation of the **SSM Agent which is needed to run these actions**❗️.

```

{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ssm:DescribeInstanceInformation",
                "ssm:GetCommandInvocation",
                "ssm:CancelCommand"
            ],
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": "ssm:SendCommand",
            "Resource": "arn:aws:ssm:*:*:document/AWSFIS-*"
        },
        {
            "Effect": "Allow",
            "Action": "ssm:SendCommand",
            "Resource": "*",
            "Condition": {
                "StringLike": {
                    "ssm:resourceTag/FAULT_INJECTION_SIDECAR": [
                        "true"
                    ]
                }
            }
        }
    ]
}
```
[Installation guide for the SSM Agent](./README-ecs-ssm-setup.md).
</details>
<details>
    <summary>Elasticache-Discovery & Actions</summary>

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
				"elasticache:DescribeReplicationGroups",
				"elasticache:TestFailover",
				"elasticache:ListTagsForResource"
      ],
      "Resource": "*"
    }
  ]
}
```

</details>
<details>
    <summary>ELB-Discovery & Actions</summary>

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "elasticloadbalancing:DescribeLoadBalancers",
        "elasticloadbalancing:DescribeListeners",
        "elasticloadbalancing:DescribeTags",
        "elasticloadbalancing:DescribeRules",
        "elasticloadbalancing:SetRulePriorities",
        "elasticloadbalancing:CreateRule",
        "elasticloadbalancing:DeleteRule",
        "elasticloadbalancing:AddTags",
        "elasticloadbalancing:RemoveTags"
      ],
      "Resource": "*"
    }
  ]
}
```

</details>
<details>
    <summary>FIS-Discovery & Actions</summary>

FIS will create a [ServiceLinkedRole](https://docs.aws.amazon.com/fis/latest/userguide/using-service-linked-roles.html)
AWSServiceRoleForFIS when you start an
experiment. If you started the experiment from the ui and if the role is already existing, you can omit the iam:
CreateServiceLinkedRole permission. If you want to
start the very first fis experiment via the steadybit agent, you will need to add the permission.

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "fis:ListExperimentTemplates",
        "fis:GetExperiment",
        "fis:GetExperimentTemplate",
        "fis:StartExperiment",
        "fis:StopExperiment",
        "fis:TagResource"
      ],
      "Effect": "Allow",
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": "iam:CreateServiceLinkedRole",
      "Resource": "arn:aws:iam::<YOUR-ACCOUNT>:role/aws-service-role/fis.amazonaws.com/AWSServiceRoleForFIS"
    }
  ]
}
```

</details>
<details>
    <summary>MSK-Discovery & Actions</summary>

```yaml
{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Sid": "VisualEditor0",
			"Effect": "Allow",
			"Action": [
				"kafka:ListClustersV2",
				"kafka:TagResource",
				"kafka:ListNodes",
				"kafka:UntagResource",
				"kafka:RebootBroker"
			],
			"Resource": "*"
		}
	]
}
```

</details>
<details>
    <summary>Lambda Functions-Discovery & Attacks</summary>

> ❗️These attacks need the [failure-lambda library to be included in your lambdas](https://github.com/steadybit/failure-lambda?tab=readme-ov-file#how-to-install-with-parameter-in-ssm-parameter-store).

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ssm:AddTagsToResource",
        "ssm:PutParameter",
        "ssm:DeleteParameter",
        "lambda:ListFunctions",
        "tag:GetResources"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
```

</details>
<details>
    <summary>RDS-Discovery & Actions</summary>

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "rds:RebootDBCluster",
        "rds:ListTagsForResource",
        "rds:StopDBInstance",
        "rds:RebootDBInstance",
        "rds:DescribeDBInstances",
        "rds:FailoverDBClusters",
        "rds:DescribeDBClusters"
      ],
      "Resource": "*"
    }
  ]
}
```

</details>

#### Authentication setup

The extension is using
the [default credentials provider chain](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials).

You can pass credentials using the following sequence:

1. Environment variables.
    1. Static Credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN)
    1. Web Identity Token (AWS_WEB_IDENTITY_TOKEN_FILE)
1. Shared configuration files.
    1. SDK defaults to credentials file under .aws folder that is placed in the home folder on your computer.
    1. SDK defaults to config file under .aws folder that is placed in the home folder on your computer.
1. If your application uses an ECS task definition or RunTask API operation, IAM role for tasks.
1. If your application is running on an Amazon EC2 instance, IAM role for Amazon EC2.

You can find more information about best matching ways to provide credentials in the following installation guides.

<details>
    <summary>Authenticate on EC2 Instance</summary>

If you installed the agent on an EC2 instance, the easiest way is to use the option 4 from
the [default credentials provider chain](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials).

Steps:

- Assign your previous created IAM role to the ec2 instance. There is a slight difference between IAM Roles and Instance
  Profiles, if you see a message like No
  roles attached to instance profile, make sure to
  check [this page](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)
- The IAM role needs a trust relationship so that EC2 is able to assume the role.
    ```yaml
    {
        "Version": "2012-10-17",
        "Statement": [
          {
            "Effect": "Allow",
            "Principal": {
              "Service": [
                "ec2.amazonaws.com"
              ]
            },
            "Action": "sts:AssumeRole"
          }
        ]
    }
    ```

</details>

<details>
    <summary>Authenticate when running as ECS Task</summary>

The `taskRoleArn` of your task definition needs to have the required permissions mentioned [here](#ecs-permissions). Make sure, that the
role can be assumed by ECS and provide a
trust relationship to the role.

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "ecs-tasks.amazonaws.com"
        ]
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

</details>
<details>
    <summary>Authenticate when running in EKS</summary>

If you installed the agent into an EKS cluster, the recommended way to provide credentials is to use option 1.ii from
the [default credentials provider chain](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials).
This approach will use a Web
Identity Token.

With this option you need to associate an IAM role with a Kubernetes service account.

Steps:

- [Create an OIDC Provider for your cluster](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html)
- Create an IAM Role with the required permissions.
- Allow the Role to be assumed by the OIDC Provider. Add the following trust relationship to the IAM Role
    ```yaml
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Federated": "arn:aws:iam::<ACCOUNT>:oidc-provider/oidc.eks.<REGION>.amazonaws.com/id/<ID>"
                },
                "Action": "sts:AssumeRoleWithWebIdentity",
                "Condition": {
                    "StringEquals": {
                        "oidc.eks.<REGION>.amazonaws.com/id/<ID>:aud": "sts.amazonaws.com",
                        "oidc.eks.<REGION>.amazonaws.com/id/<ID>:sub": "system:serviceaccount:<SERVICE-ACCOUNT-NAMESPACE>:<SERVICE-ACCOUNT-NAME>"
                    }
                }
            }
        ]
    }
    ```
- Associate the IAM Role to your Kubernetes Service Account. If you are using our helm charts to create the Service
  Account, you can use the parameter
  `serviceAccount.eksRoleArn`.

</details>

<details>
    <summary>Authenticate when running outside of AWS</summary>

You can install the aws extension outside your AWS infrastructure to communicate with the AWS API. In this case you need
to set up an IAM User with API
credentials which is allowed to access the resources already described in the section above.

The following variables needs to be added to the environment configuration:

```yaml
AWS_REGION=<replace-with-region-to-attack>
AWS_ACCESS_KEY_ID=<replace-with-aws-access-key>
AWS_SECRET_ACCESS_KEY=<replace-with-aws-secret-access-key>
```

</details>

### Assume Role into Multiple AWS Accounts

By default, the extension uses the provided credentials to discover all resources within the belonging AWS account. To
interact with multiple AWS accounts using
a single extension, you can instruct the extension only to use the provided credentials to assume roles (using AWS STS)
into given role-ARNs (and thus to
possibly other AWS accounts).

To achieve this, you must set the STEADYBIT_EXTENSION_ASSUME_ROLES environment variable to a comma-separated list of
role-ARNs. Example:

```sh
STEADYBIT_EXTENSION_ASSUME_ROLES='arn:aws:iam::1111111111:role/steadybit-extension-aws,arn:aws:iam::22222222:role/steadybit-extension-aws'
```

If you are using our helm-chart, you can use the parameter `aws.assumeRoles`.

#### Necessary AWS Configuration

IAM policies need to be correctly configured for cross-account role assumption. In a nutshell, these are the necessary
steps:

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
2. The roles themselves have all the [required permissions](#required-permissions-policies).
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

### Multi Region Support

By default, the extension will discover targets only in the AWS Region that is provided by the current authentication (environment variable `AWS_REGION`).

If you want to discover targets in multiple regions, you can set the `STEADYBIT_EXTENSION_REGIONS` environment variable to a comma-separated list of regions. Example:

```sh
STEADYBIT_EXTENSION_REGIONS='us-east-1,us-west-2'
```


### Agent Lockout - Requirements

In order to prevent the agent or the extension of beeing locked out by their own attacks, we implemented some security
checks.

For example, the blackhole az attack won't start, if

- the extension is running in the attacked account
- the agent is running in the attacked account

The extension can verify the own account based on the authentication described above.

For the agent you need make sure, that the agent can determine the aws account where it is running on. The agent
tries to get the account id from:

1. A static configuration property `steadybit.agent.aws.accountId` or env `STEADYBIT_AGENT_AWS_ACCOUNT_ID`
2. The EC2-Metadata-Service if reachable
3. sts:GetCallerIdentity. You don't need any special permissions for that call, but the agent agents needs to provide
   credentials for the api. Same
   authentication setup mechanism as for the extensions apply for the agent setup. Make sure to
   check [Authentication Setup](#Authentication-Setup) for
   more details. For example, if running in kubernetes and using a ServiceAccount, you can
   use `serviceAccount.eksRoleArn` of the agent helm chart to link
   your serviceAccount to a given role.

## Installation

### Kubernetes

Detailed information about agent and extension installation in kubernetes can also be found in
our [documentation](https://docs.steadybit.com/install-and-configure/install-agent/install-on-kubernetes).

#### Recommended (via agent helm chart)

All extensions provide a helm chart that is also integrated in the
[helm-chart](https://github.com/steadybit/helm-charts/tree/main/charts/steadybit-agent) of the agent.

You must provide additional values to activate this extension.

```
--set extension-aws.enabled=true \
--set extension-aws.serviceAccount.eksRoleArn={{YOUR_SERVICE_ACCOUNT_ARN_IF_RUNNING_IN_EKS}} \
```

Additional configuration options can be found in
the [helm-chart](https://github.com/steadybit/extension-aws/blob/main/charts/steadybit-extension-aws/values.yaml) of the
extension.

#### Alternative (via own helm chart)

If you need more control, you can install the extension via its
dedicated [helm-chart](https://github.com/steadybit/extension-aws/blob/main/charts/steadybit-extension-aws).

```bash
helm repo add steadybit-extension-aws https://steadybit.github.io/extension-aws
helm repo update
helm upgrade steadybit-extension-aws \
    --install \
    --wait \
    --timeout 5m0s \
    --create-namespace \
    --namespace steadybit-agent \
    --set serviceAccount.eksRoleArn={{YOUR_SERVICE_ACCOUNT_ARN_IF_RUNNING_IN_EKS}} \
    steadybit-extension-aws/steadybit-extension-aws
```

### Linux Package

Please use
our [agent-linux.sh script](https://docs.steadybit.com/install-and-configure/install-agent/install-on-linux-hosts)
to install the extension on your Linux machine. The script will download the latest version of the extension and install
it using the package manager.

After installing, configure the extension by editing `/etc/steadybit/extension-aws` and then restart the service.

## Extension registration

Make sure that the extension is registered with the agent. In most cases this is done automatically. Please refer to
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-discovery) for more
information about extension registration and how to verify.


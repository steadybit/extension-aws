<img src="./logo.jpeg" height="130" align="right" alt="Amazon Web Services (AWS) logo depicting five orange colored boxes and the text Amazon Web Services">

# Steadybit extension-aws

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject faults into various AWS services.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.github.steadybit.extension_aws).

## Configuration

### Environment Variables

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

### Disable parts of the extension

If you don't want to use certain parts of the extension and therefore don't want to provide the required permissions, you can disable these parts by providing
the following environment variables. Actions using these targets will not be shown in the ui, as there are no targets reported.

| Environment Variable                            | Meaning                 | Required | Default |
|-------------------------------------------------|-------------------------|----------|---------|
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EC2`    | Disable EC2-Discovery   | no       | false   |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_RDS`    | Disable RDS-Discovery   | no       | false   |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ZONE`   | Disable Zone-Discovery  | no       | false   |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_FIS`    | Disable FIS-Discovery   | no       | false   |
| `STEADYBIT_EXTENSION_DISCOVERY_DISABLED_LAMBDA` | Disable Lamba-Discovery | no       | false   |

### Authentication

The process requires valid access credentials to interact with various AWS APIs.

#### Required permissions (policies)

You will need an IAM Role with the given permissions. You can optionally restrict for which resources the extension may become active
by tweaking the `Resource` clause.

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
        "rds:RebootDBInstance",
        "rds:DescribeDBInstances",
        "rds:DescribeDBClusters"
      ],
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
        "ec2:TerminateInstances"
      ],
      "Resource": "*"
    }
  ]
}
```

</details>
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
    <summary>FIS-Discovery & Actions</summary>

FIS will create a [ServiceLinkedRole](https://docs.aws.amazon.com/fis/latest/userguide/using-service-linked-roles.html) AWSServiceRoleForFIS when you start an
experiment. If you started the experiment from the ui and the role is already existing, you can omit the iam:CreateServiceLinkedRole permission. If you want to
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
    <summary>Lambda Functions-Discovery & Attacks</summary>

```yaml
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ssm:AddTagsToResource",
        "ssm:PutParameter",
        "ssm:DeleteParameter",
        "lambda:ListFunctions"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
```

</details>

#### Authentication setup

The extension is using the [default credentials provider chain](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials).

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

- Assign your previous created IAM role to the ec2 instance. There is a slight difference between IAM Roles and Instance Profiles, if you see a message like No
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

The `taskRoleArn` of your task definition needs to have the required permissions mentioned before. Make sure, that the role can be assumed by ECS and provide a
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
the [default credentials provider chain](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials). This approach will use a Web
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
- Associate the IAM Role to your Kubernetes Service Account. If you are using our helm charts to create the Service Account, you can use the parameter
  `serviceAccount.eksRoleArn`.

</details>

<details>
    <summary>Authenticate when running outside of AWS</summary>

You can install the aws extension outside your AWS infrastructure to communicate with the AWS API. In this case you need to set up an IAM User with API
credentials which is allowed to access the resources already described in the section above.

The following variables needs to be added to the environment configuration:

```yaml
AWS_REGION=<replace-with-region-to-attack>
AWS_ACCESS_KEY_ID=<replace-with-aws-access-key>
AWS_SECRET_ACCESS_KEY=<replace-with-aws-secret-access-key>
```

</details>

### Assume Role into Multiple AWS Accounts

By default, the extension uses the provided credentials to discover all resources within the belonging AWS account. To interact with multiple AWS accounts using
a single extension, you can instruct the extension only to use the provided credentials to assume roles (using AWS STS) into given role-ARNs (and thus to
possibly other AWS accounts).

To achieve this, you must set the STEADYBIT_EXTENSION_ASSUME_ROLES environment variable to a comma-separated list of role-ARNs. Example:

```sh
STEADYBIT_EXTENSION_ASSUME_ROLES='arn:aws:iam::1111111111:role/steadybit-extension-aws,arn:aws:iam::22222222:role/steadybit-extension-aws'
```

If you are using our helm-chart, you can use the parameter `aws.assumeRoles`.

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

## Installation

We recommend that you install the extension with our [official Helm chart](https://github.com/steadybit/extension-aws/tree/main/charts/steadybit-extension-aws).

### Helm

```bash
helm repo add steadybit https://steadybit.github.io/extension-aws
helm repo update
helm upgrade steadybit-extension-aws \
    --install \
    --wait \
    --timeout 5m0s \
    --create-namespace \
    --namespace steadybit-extension \
    --serviceAccount.eksRoleArn={{YOUR_SERVICE_ACCOUNT_ARN_IF_RUNNING_IN_EKS}} \
    steadybit/steadybit-extension-aws
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.


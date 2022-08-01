# Steadybit extension-aws

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject HTTP faults into various AWS services.

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

## Deployment

We recommend that you deploy the extension with our [official Helm chart](https://github.com/steadybit/helm-charts/tree/main/charts/steadybit-extension-aws).
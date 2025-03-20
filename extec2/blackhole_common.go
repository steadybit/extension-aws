package extec2

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"golang.org/x/exp/slices"
	"strings"
)

type BlackholeState struct {
	AgentAWSAccount     string
	ExtensionAwsAccount string
	TargetRegion        string
	DiscoveredByRole    *string
	NetworkAclIds       []string
	OldNetworkAclIds    map[string]string   // map[NewAssociationId] = oldNetworkAclId
	TargetSubnets       map[string][]string // map[vpcId] = [subnetIds]
	AttackExecutionId   uuid.UUID
}

type blackholeEC2Api interface {
	ec2.DescribeSubnetsAPIClient
	ec2.DescribeNetworkAclsAPIClient
	CreateNetworkAcl(ctx context.Context, params *ec2.CreateNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclOutput, error)
	CreateNetworkAclEntry(ctx context.Context, params *ec2.CreateNetworkAclEntryInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclEntryOutput, error)
	ReplaceNetworkAclAssociation(ctx context.Context, params *ec2.ReplaceNetworkAclAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceNetworkAclAssociationOutput, error)
	DeleteNetworkAcl(ctx context.Context, params *ec2.DeleteNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkAclOutput, error)
}

type blackholeImdsApi interface {
	GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
}

func prepareBlackhole(ctx context.Context, state *BlackholeState, request action_kit_api.PrepareActionRequestBody, extensionRootAccountNumber string, clientProvider func(account string, region string, role *string) (blackholeEC2Api, blackholeImdsApi, error), subnetProvider func(clientEc2 blackholeEC2Api, ctx context.Context, target *action_kit_api.Target) (map[string][]string, error)) (*action_kit_api.PrepareResult, error) {
	targetAccount := extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	targetRegion := extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	discoveredByRole := utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")

	// Get AWS Clients
	clientEc2, clientImds, err := clientProvider(targetAccount, targetRegion, discoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize AWS clients for AWS targetAccount %s", targetAccount), err)
	}
	//Get Extension Account
	protectedAccounts := getProtectedAWSAccounts(ctx, clientImds, extensionRootAccountNumber)
	if len(protectedAccounts) == 0 {
		return nil, extension_kit.ToError("Could not get AWS Account of the extension. Attack is disabled to prevent an extension lockout.", nil)
	}
	if slices.Contains(protectedAccounts, targetAccount) {
		return nil, extension_kit.ToError(fmt.Sprintf("The extension is running in a protected AWS account (%s). Attack is disabled to prevent an extension lockout.", protectedAccounts), nil)
	}

	agentAwsAccountId := ""
	if request.ExecutionContext != nil && request.ExecutionContext.AgentAwsAccountId != nil {
		agentAwsAccountId = *request.ExecutionContext.AgentAwsAccountId
	}

	if agentAwsAccountId == "" {
		return nil, extension_kit.ToError("Could not get AWS Account of the agent. Attack is disabled to prevent an agent lockout. Please check https://github.com/steadybit/extension-aws#agent-lockout---requirements", nil)
	}

	if targetAccount == agentAwsAccountId {
		return nil, extension_kit.ToError(fmt.Sprintf("The agent is running in the same AWS account (%s) as the target. Attack is disabled to prevent an agent lockout.", agentAwsAccountId), nil)
	}

	// Get Target Subnets
	targetSubnets, err := subnetProvider(clientEc2, ctx, request.Target)
	if err != nil {
		return nil, err
	}

	state.AgentAWSAccount = agentAwsAccountId
	state.ExtensionAwsAccount = targetAccount
	state.TargetRegion = targetRegion
	state.TargetSubnets = targetSubnets
	state.AttackExecutionId = request.ExecutionId
	state.DiscoveredByRole = discoveredByRole
	return nil, nil
}

func startBlackhole(ctx context.Context, state *BlackholeState, clientProvider func(account string, region string, role *string) (blackholeEC2Api, blackholeImdsApi, error)) (*action_kit_api.StartResult, error) {
	clientEc2, _, err := clientProvider(state.ExtensionAwsAccount, state.TargetRegion, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", state.ExtensionAwsAccount), err)
	}
	log.Info().Msgf("Starting AZ Blackhole attack against AWS account %s and region %s", state.ExtensionAwsAccount, state.TargetRegion)
	log.Debug().Msgf("Attack state: %+v", state)

	state.OldNetworkAclIds = make(map[string]string)

	for vpcId, subnetIds := range state.TargetSubnets {
		log.Info().Msgf("Creating temporary ACL to block traffic in VPC %s", vpcId)
		//Find existing to be modified network acl associations matching the subnetIds in the given VPC
		desiredAclAssociations, getNetworkAclAssociationsErr := getNetworkAclAssociations(ctx, clientEc2, vpcId, subnetIds)
		if getNetworkAclAssociationsErr != nil {
			log.Error().Err(getNetworkAclAssociationsErr).Msgf("Failed to get network ACL associations for VPC %s", vpcId)
			err = extension_kit.ToError(fmt.Sprintf("Failed to get network ACL associations for VPC %s", vpcId), getNetworkAclAssociationsErr)
			break
		}
		log.Info().Msgf("Found %d network ACL associations to modify", len(desiredAclAssociations))

		networkAclId, createNetworkAclErr := createNetworkAcl(ctx, state, clientEc2, vpcId, desiredAclAssociations)
		if createNetworkAclErr != nil {
			log.Error().Err(createNetworkAclErr).Msgf("Failed to create network ACL for VPC %s", vpcId)
			err = extension_kit.ToError(fmt.Sprintf("Failed to create network ACL for VPC %s", vpcId), createNetworkAclErr)
			break
		}

		//Replace the association IDs for the above subnets with the new network acl which will deny all traffic for those subnets in that AZ
		replaceNetworkAclAssociationsErr := replaceNetworkAclAssociations(ctx, state, clientEc2, desiredAclAssociations, networkAclId)
		if replaceNetworkAclAssociationsErr != nil {
			log.Error().Err(replaceNetworkAclAssociationsErr).Msgf("Failed to replace network ACL associations for VPC %s", vpcId)
			err = extension_kit.ToError(fmt.Sprintf("Failed to replace network ACL associations for VPC %s", vpcId), replaceNetworkAclAssociationsErr)
			break
		}
	}

	if err != nil {
		_ = rollbackBlackholeViaTags(ctx, state, clientEc2)
	}
	return nil, err
}

func stopBlackhole(ctx context.Context, state *BlackholeState, clientProvider func(account string, region string, role *string) (blackholeEC2Api, blackholeImdsApi, error)) (*action_kit_api.StopResult, error) {
	clientEc2, _, err := clientProvider(state.ExtensionAwsAccount, state.TargetRegion, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s and region %s", state.ExtensionAwsAccount, state.TargetRegion), err)
	}

	return nil, rollbackBlackholeViaTags(ctx, state, clientEc2)
}

func getProtectedAWSAccounts(ctx context.Context, clientImds blackholeImdsApi, extensionRootAccountNumber string) []string {
	ec2MetadataAccountId := getAccountNumberByEC2Metadata(ctx, clientImds)
	if ec2MetadataAccountId == "" && extensionRootAccountNumber != "" {
		log.Info().Msgf("Agent AWS Account %s provided by STS get-caller-identity", extensionRootAccountNumber)
		return []string{extensionRootAccountNumber}
	}
	if ec2MetadataAccountId != "" && extensionRootAccountNumber != "" && extensionRootAccountNumber != ec2MetadataAccountId {
		log.Info().Msgf("Agent AWS Account %s provided by EC2-Metadata-Service differs from the one provided by STS get-caller-identity %s", ec2MetadataAccountId, extensionRootAccountNumber)
		return []string{ec2MetadataAccountId, extensionRootAccountNumber}
	}
	if ec2MetadataAccountId != "" {
		log.Info().Msgf("Agent AWS Account %s provided by EC2", extensionRootAccountNumber)
		return []string{ec2MetadataAccountId}
	}
	return []string{}
}

func getAccountNumberByEC2Metadata(ctx context.Context, clientImds blackholeImdsApi) string {
	ec2Metadata, err := clientImds.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil || ec2Metadata == nil {
		log.Debug().Err(err).Msg("Unable to get AWS Account ID by EC2-Metadata-Service.")
		return ""
	}
	if ec2Metadata != nil && ec2Metadata.InstanceIdentityDocument.AccountID != "" {
		log.Info().Msgf("Agent AWS Account %s provided by EC2-Metadata-Service", ec2Metadata.InstanceIdentityDocument.AccountID)
	}
	return ec2Metadata.InstanceIdentityDocument.AccountID
}

func replaceNetworkAclAssociations(ctx context.Context, state *BlackholeState, clientEc2 blackholeEC2Api, desiredAclAssociations []types.NetworkAclAssociation, networkAclId string) error {
	for _, networkAclAssociation := range desiredAclAssociations {
		oldNetworkAclId := strings.Clone(*networkAclAssociation.NetworkAclId)
		networkAclAssociationInput := &ec2.ReplaceNetworkAclAssociationInput{
			AssociationId: networkAclAssociation.NetworkAclAssociationId,
			NetworkAclId:  aws.String(networkAclId),
		}
		log.Debug().Msgf("Replacing acl entry %+v", networkAclAssociationInput)
		replaceNetworkAclAssociationResponse, err := clientEc2.ReplaceNetworkAclAssociation(ctx, networkAclAssociationInput)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to replace network ACL association %s with network ACL %s", *networkAclAssociation.NetworkAclAssociationId, networkAclId)
			return err
		}
		state.OldNetworkAclIds[*replaceNetworkAclAssociationResponse.NewAssociationId] = oldNetworkAclId
		log.Debug().Msgf("Replaced acl entry %+v", replaceNetworkAclAssociationResponse)
	}
	return nil
}

func createNetworkAcl(ctx context.Context, state *BlackholeState, clientEc2 blackholeEC2Api, vpcId string, desiredAclAssociations []types.NetworkAclAssociation) (string, error) {
	//Create new network acl for each vpc in the zone
	tagList := []types.Tag{
		{
			Key:   aws.String("Name"),
			Value: aws.String("created by steadybit"),
		},
	}
	if state.AttackExecutionId.String() == "" {
		return "", fmt.Errorf("AttackExecutionId is empty")
	}

	tagList = append(tagList, types.Tag{
		Key:   aws.String("steadybit-attack-execution-id"),
		Value: aws.String(state.AttackExecutionId.String()),
	})

	for _, desiredAclAssociation := range desiredAclAssociations {
		tagList = append(tagList, types.Tag{
			Key:   aws.String("steadybit-replaced " + *desiredAclAssociation.SubnetId),
			Value: aws.String(*desiredAclAssociation.NetworkAclId),
		})
	}

	createNetworkAclResult, err := clientEc2.CreateNetworkAcl(ctx, &ec2.CreateNetworkAclInput{
		VpcId: aws.String(vpcId),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeNetworkAcl,
				Tags:         tagList,
			},
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create network ACL")
		return "", err
	}
	log.Debug().Msgf("Created network ACL %+v", *createNetworkAclResult.NetworkAcl)

	state.NetworkAclIds = append(state.NetworkAclIds, *createNetworkAclResult.NetworkAcl.NetworkAclId)

	//Create deny all egress rule
	createNetworkAclEntry(ctx, clientEc2, *createNetworkAclResult.NetworkAcl.NetworkAclId, 100, true)
	createNetworkAclEntry(ctx, clientEc2, *createNetworkAclResult.NetworkAcl.NetworkAclId, 101, false)
	return *createNetworkAclResult.NetworkAcl.NetworkAclId, nil
}

func createNetworkAclEntry(ctx context.Context, clientEc2 blackholeEC2Api, networkAclId string, ruleNumber int, egress bool) {
	createdNetworkAclEntry, err := clientEc2.CreateNetworkAclEntry(ctx, &ec2.CreateNetworkAclEntryInput{
		NetworkAclId: aws.String(networkAclId),
		RuleNumber:   aws.Int32(int32(ruleNumber)),
		CidrBlock:    aws.String("0.0.0.0/0"),
		Egress:       aws.Bool(egress),
		Protocol:     aws.String("-1"),
		PortRange: &types.PortRange{
			From: aws.Int32(0),
			To:   aws.Int32(65535),
		},
		RuleAction: types.RuleActionDeny,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create network ACL entry")
	} else {
		log.Debug().Msgf("Created network ACL entry for network ACL %+v", createdNetworkAclEntry)
	}
}

func getNetworkAclAssociations(ctx context.Context, clientEc2 blackholeEC2Api, vpcId string, targetSubnetIds []string) ([]types.NetworkAclAssociation, error) {
	desiredAclAssociations := make([]types.NetworkAclAssociation, 0, len(targetSubnetIds))
	networkAclsAssociatedWithSubnets := make([]types.NetworkAclAssociation, 0, len(targetSubnetIds))
	paginator := ec2.NewDescribeNetworkAclsPaginator(clientEc2, &ec2.DescribeNetworkAclsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("association.subnet-id"),
				Values: targetSubnetIds,
			},
		},
	})
	for paginator.HasMorePages() {
		describeNetworkAclsResult, err := paginator.NextPage(ctx)

		if err != nil {
			log.Error().Err(err).Msg("Failed to get network ACLs")
			return nil, err
		}
		for _, networkAcl := range describeNetworkAclsResult.NetworkAcls {
			networkAclsAssociatedWithSubnets = append(networkAclsAssociatedWithSubnets, networkAcl.Associations...)
		}
	}

	for _, subnetId := range targetSubnetIds {
		for _, networkAcl := range networkAclsAssociatedWithSubnets {
			if *networkAcl.SubnetId == subnetId {
				log.Info().Msgf("Found network ACL %s associated with subnet %s", *networkAcl.NetworkAclId, subnetId)
				desiredAclAssociations = append(desiredAclAssociations, networkAcl)
			}
		}
	}
	log.Debug().Msgf("Found  %+v acl associations for subnets  %+v in VPC %s.", desiredAclAssociations, targetSubnetIds, vpcId)
	return desiredAclAssociations, nil
}

func rollbackBlackholeViaTags(ctx context.Context, state *BlackholeState, clientEc2 blackholeEC2Api) error {
	// get all network ACLs created by Steadybit
	networkAcls, err := getAllNACLsCreatedBySteadybit(clientEc2, ctx, state.AttackExecutionId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network ACLs created by Steadybit")
		return extension_kit.ToError("Failed to get network ACLs created by Steadybit", err)
	}

	var errors []string
	for _, networkAcl := range *networkAcls {
		for _, tag := range networkAcl.Tags {
			// find tags beginning with "steadybit-replaced "
			if strings.HasPrefix(*tag.Key, "steadybit-replaced ") {
				subnetId := strings.TrimPrefix(*tag.Key, "steadybit-replaced ")
				for _, networkAclAssociation := range networkAcl.Associations {
					if *networkAclAssociation.SubnetId == subnetId {
						networkAclAssociationInput := &ec2.ReplaceNetworkAclAssociationInput{
							AssociationId: aws.String(*networkAclAssociation.NetworkAclAssociationId),
							NetworkAclId:  aws.String(*tag.Value),
						}
						log.Debug().Msgf("Rolling back to old acl %+v", networkAclAssociationInput)
						replaceNetworkAclAssociationResponse, replaceErr := clientEc2.ReplaceNetworkAclAssociation(context.Background(), networkAclAssociationInput)
						if replaceErr != nil {
							log.Error().Err(replaceErr).Msgf("Failed to rollback to old acl entry %+v", networkAclAssociationInput)
							errors = append(errors, replaceErr.Error())
						} else {
							log.Debug().Msgf("Rolled back to old acl %+v", replaceNetworkAclAssociationResponse)
						}
					}
				}
			}
		}
	}

	//Don't delete any network ACLs if there are errors in rollback because their tags contains information about the old network ACLs
	if errors != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to rollback network ACL association: %s", strings.Join(errors, ", ")), nil)
	} else {
		for _, networkAcl := range *networkAcls {
			//Delete NACL
			deleteNetworkAclInput := &ec2.DeleteNetworkAclInput{
				NetworkAclId: aws.String(*networkAcl.NetworkAclId),
			}
			log.Debug().Msgf("Deleting network acl %+v", deleteNetworkAclInput)
			deleteNetworkAclResponse, deleteErr := clientEc2.DeleteNetworkAcl(context.Background(), deleteNetworkAclInput)
			if deleteErr != nil {
				log.Error().Err(deleteErr).Msgf("Failed to delete network acl %+v", deleteNetworkAclInput)
				errors = append(errors, deleteErr.Error())
			} else {
				log.Debug().Msgf("Deleted network acl entry  %+v", deleteNetworkAclResponse)
			}
		}
	}

	if errors != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to delete network acls: %s", strings.Join(errors, ", ")), nil)
	}
	return nil
}

func getAllNACLsCreatedBySteadybit(clientEc2 blackholeEC2Api, ctx context.Context, executionId uuid.UUID) (*[]types.NetworkAcl, error) {
	result := make([]types.NetworkAcl, 0)
	paginator := ec2.NewDescribeNetworkAclsPaginator(clientEc2, &ec2.DescribeNetworkAclsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{"created by steadybit"},
			}, {
				Name:   aws.String("tag:steadybit-attack-execution-id"),
				Values: []string{executionId.String()},
			},
		},
	})

	for paginator.HasMorePages() {
		describeNetworkAclsResult, err := paginator.NextPage(ctx)

		if err != nil {
			log.Error().Err(err).Msg("Failed to get network ACLs")
			return nil, err
		}
		result = append(result, describeNetworkAclsResult.NetworkAcls...)
	}
	return &result, nil
}

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extaz

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
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"strings"
)

type azBlackholeAction struct {
	clientProvider             func(account string) (azBlackholeEC2Api, azBlackholeImdsApi, error)
	extensionRootAccountNumber string
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[BlackholeState] = (*azBlackholeAction)(nil)
var _ action_kit_sdk.ActionWithStop[BlackholeState] = (*azBlackholeAction)(nil)

type BlackholeState struct {
	AgentAWSAccount     string
	ExtensionAwsAccount string
	TargetZone          string
	NetworkAclIds       []string
	OldNetworkAclIds    map[string]string   // map[NewAssociationId] = oldNetworkAclId
	TargetSubnets       map[string][]string // map[vpcId] = [subnetIds]
	AttackExecutionId   uuid.UUID
}

type azBlackholeEC2Api interface {
	ec2.DescribeSubnetsAPIClient
	ec2.DescribeNetworkAclsAPIClient
	ec2.DescribeTagsAPIClient
	CreateNetworkAcl(ctx context.Context, params *ec2.CreateNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclOutput, error)
	CreateNetworkAclEntry(ctx context.Context, params *ec2.CreateNetworkAclEntryInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclEntryOutput, error)
	ReplaceNetworkAclAssociation(ctx context.Context, params *ec2.ReplaceNetworkAclAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceNetworkAclAssociationOutput, error)
	DeleteNetworkAcl(ctx context.Context, params *ec2.DeleteNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkAclOutput, error)
}

type azBlackholeImdsApi interface {
	GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
}

func NewAzBlackholeAction() action_kit_sdk.Action[BlackholeState] {
	return &azBlackholeAction{
		clientProvider:             defaultClientProvider,
		extensionRootAccountNumber: utils.Accounts.GetRootAccount().AccountNumber,
	}
}

func (e *azBlackholeAction) NewEmptyState() BlackholeState {
	return BlackholeState{}
}

func (e *azBlackholeAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          azBlackholeActionId,
		Label:       "Blackhole Availability Zone",
		Description: "Simulates an outage of an entire availability zone.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(azIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: "zone",
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by zone",
					Description: extutil.Ptr("Find zone by name"),
					Query:       "aws.zone=\"\"",
				},
				{
					Label:       "by zone-id",
					Description: extutil.Ptr("Find zone by zone id"),
					Query:       "aws.zone.id=\"\"",
				},
			})}),
		Category:    extutil.Ptr("network"),
		TimeControl: action_kit_api.External,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr(""),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("60s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (e *azBlackholeAction) Prepare(ctx context.Context, state *BlackholeState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	// Get Target Attributes
	targetAccount := request.Target.Attributes["aws.account"]
	if targetAccount == nil || len(targetAccount) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.targetAccount' target attribute.", nil))
	}

	targetZone := request.Target.Attributes["aws.zone"]
	if targetZone == nil || len(targetZone) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.zone' target attribute.", nil))
	}

	// Get AWS Clients
	clientEc2, clientImds, err := e.clientProvider(targetAccount[0])
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize AWS clients for AWS targetAccount %s", targetAccount[0]), err))
	}
	//Get Extension Account
	extensionAwsAccount := e.getExtensionAWSAccount(ctx, clientImds)
	if extensionAwsAccount == "" {
		return nil, extutil.Ptr(extension_kit.ToError("Could not get AWS Account of the extension. Attack is disabled to prevent an extension lockout.", nil))
	}
	if targetAccount[0] == extensionAwsAccount {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("The extension is running in the same AWS account (%s) as the target. Attack is disabled to prevent an extension lockout.", extensionAwsAccount), nil))
	}

	agentAwsAccountId := ""
	if request.ExecutionContext != nil && request.ExecutionContext.AgentAwsAccountId != nil {
		agentAwsAccountId = *request.ExecutionContext.AgentAwsAccountId
	}

	if agentAwsAccountId == "" {
		return nil, extutil.Ptr(extension_kit.ToError("Could not get AWS Account of the agent. Attack is disabled to prevent an agent lockout.", nil))
	}

	if targetAccount[0] == agentAwsAccountId {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("The agent is running in the same AWS account (%s) as the target. Attack is disabled to prevent an agent lockout.", extensionAwsAccount), nil))
	}

	// Get Target Subnets
	targetSubnets, err := getTargetSubnets(clientEc2, ctx, targetZone[0])
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to get subnets for zone %s", targetZone[0]), err))
	}

	state.AgentAWSAccount = agentAwsAccountId
	state.ExtensionAwsAccount = targetAccount[0]
	state.TargetZone = targetZone[0]
	state.TargetSubnets = targetSubnets
	state.AttackExecutionId = request.ExecutionId
	return nil, nil
}

func (e *azBlackholeAction) Start(ctx context.Context, state *BlackholeState) (*action_kit_api.StartResult, error) {
	clientEc2, _, err := e.clientProvider(state.ExtensionAwsAccount)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", state.ExtensionAwsAccount), err)
	}
	log.Info().Msgf("Starting AZ Blackhole attack against AWS account %s", state.ExtensionAwsAccount)
	log.Debug().Msgf("Attack state: %+v", state)

	state.OldNetworkAclIds = make(map[string]string)

	for vpcId, subnetIds := range state.TargetSubnets {
		log.Info().Msgf("Creating temporary ACL to block traffic in VPC %s", vpcId)
		//Find existing to be modified network acl associations matching the subnetIds in the given VPC
		desiredAclAssociations, err := getNetworkAclAssociations(ctx, clientEc2, vpcId, subnetIds)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to get network ACL associations for VPC %s", vpcId)
			err = extension_kit.ToError(fmt.Sprintf("Failed to get network ACL associations for VPC %s", vpcId), err)
			break
		}
		log.Info().Msgf("Found %d network ACL associations to modify", len(desiredAclAssociations))

		networkAclId, err := createNetworkAcl(ctx, state, clientEc2, vpcId, desiredAclAssociations)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to create network ACL for VPC %s", vpcId)
			err = extension_kit.ToError(fmt.Sprintf("Failed to create network ACL for VPC %s", vpcId), err)
			break
		}

		//Replace the association IDs for the above subnets with the new network acl which will deny all traffic for those subnets in that AZ
		err = replaceNetworkAclAssociations(ctx, state, clientEc2, desiredAclAssociations, networkAclId)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to replace network ACL associations for VPC %s", vpcId)
			err = extension_kit.ToError(fmt.Sprintf("Failed to replace network ACL associations for VPC %s", vpcId), err)
			break
		}
	}

	if err != nil {
		_ = rollbackBlackholeViaTags(ctx, state, clientEc2)
	}
	return nil, err
}

func getTargetSubnets(clientEc2 azBlackholeEC2Api, ctx context.Context, targetZone string) (map[string][]string, error) {
	subnetResults := make(map[string][]string)

	paginator := ec2.NewDescribeSubnetsPaginator(clientEc2,
		&ec2.DescribeSubnetsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("availabilityZone"),
					Values: []string{targetZone},
				},
			},
		})

	for paginator.HasMorePages() {
		subnets, err := paginator.NextPage(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get subnets")
			return nil, err
		}
		for _, subnet := range subnets.Subnets {
			subnetResults[*subnet.VpcId] = append(subnetResults[*subnet.VpcId], *subnet.SubnetId)
		}
		log.Debug().Msgf("Found %d subnets in AZ %s for creating temporary ACL to block traffic: %+v", len(subnets.Subnets), targetZone, subnets.Subnets)
	}
	return subnetResults, nil
}

func (e *azBlackholeAction) getExtensionAWSAccount(ctx context.Context, clientImds azBlackholeImdsApi) string {
	ec2MetadataAccountId := getAccountNumberByEC2Metadata(ctx, clientImds)
	resultAccountNumber := ec2MetadataAccountId

	if ec2MetadataAccountId == "" && e.extensionRootAccountNumber != "" {
		resultAccountNumber = e.extensionRootAccountNumber
		log.Info().Msgf("Agent AWS Account %s provided by STS get-caller-identity", e.extensionRootAccountNumber)
	}

	if ec2MetadataAccountId != "" && e.extensionRootAccountNumber != "" && e.extensionRootAccountNumber != ec2MetadataAccountId {
		log.Error().Msgf("Agent AWS Account %s provided by EC2-Metadata-Service differs from the one provided by STS get-caller-identity %s", ec2MetadataAccountId, e.extensionRootAccountNumber)
		return ""
	}
	return resultAccountNumber
}

func getAccountNumberByEC2Metadata(ctx context.Context, clientImds azBlackholeImdsApi) string {
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

func replaceNetworkAclAssociations(ctx context.Context, state *BlackholeState, clientEc2 azBlackholeEC2Api, desiredAclAssociations []types.NetworkAclAssociation, networkAclId string) error {
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

func createNetworkAcl(ctx context.Context, state *BlackholeState, clientEc2 azBlackholeEC2Api, vpcId string, desiredAclAssociations []types.NetworkAclAssociation) (string, error) {
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

func createNetworkAclEntry(ctx context.Context, clientEc2 azBlackholeEC2Api, networkAclId string, ruleNumber int, egress bool) {
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
	}
	log.Debug().Msgf("Created network ACL entry for network ACL %+v", createdNetworkAclEntry)
}

func getNetworkAclAssociations(ctx context.Context, clientEc2 azBlackholeEC2Api, vpcId string, targetSubnetIds []string) ([]types.NetworkAclAssociation, error) {
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
			for _, association := range networkAcl.Associations {
				networkAclsAssociatedWithSubnets = append(networkAclsAssociatedWithSubnets, association)
			}
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

func (e *azBlackholeAction) Stop(ctx context.Context, state *BlackholeState) (*action_kit_api.StopResult, error) {
	clientEc2, _, err := e.clientProvider(state.ExtensionAwsAccount)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", state.ExtensionAwsAccount), err)
	}

	return nil, rollbackBlackholeViaTags(ctx, state, clientEc2)
}

func rollbackBlackholeViaTags(ctx context.Context, state *BlackholeState, clientEc2 azBlackholeEC2Api) error {
	// get all network ACLs created by Steadybit
	networkAclsAssociatedWithSubnets, err := getAllNACLsCreatedBySteadybit(clientEc2, ctx, state.AttackExecutionId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network ACLs created by Steadybit")
		return extutil.Ptr(extension_kit.ToError("Failed to get network ACLs created by Steadybit", err))
	}

	networkAclIdsCreadedBySteadybit := make(map[string]struct{}) // a Set of network ACL IDs created by Steadybit to be deleted later
	oldNetworkAclIds := make(map[string]string)                  // key: new association ID, value: old network ACL ID --> to be used to rollback the network ACL association

	var errors []string

	for _, networkAclsAssociatedWithSubnet := range *networkAclsAssociatedWithSubnets {
		networkAclIdsCreadedBySteadybit[*networkAclsAssociatedWithSubnet.NetworkAclId] = struct{}{}
		tags, err := getTagsOfNacl(clientEc2, ctx, *networkAclsAssociatedWithSubnet.NetworkAclId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get tags of network ACL")
			errors = append(errors, err.Error())
		}

		for _, tag := range *tags {
			// find tags beginning with "steadybit-replaced "
			if strings.HasPrefix(*tag.Key, "steadybit-replaced ") {
				subnetId := strings.TrimPrefix(*tag.Key, "steadybit-replaced ")
				if *networkAclsAssociatedWithSubnet.SubnetId == subnetId {
					// get the old network ACL ID
					oldNetworkAclId := tag.Value
					oldNetworkAclIds[*networkAclsAssociatedWithSubnet.NetworkAclAssociationId] = *oldNetworkAclId
				}
			}
		}
	}

	for associationId, oldNetworkAclIdValue := range oldNetworkAclIds {
		networkAclAssociationInput := &ec2.ReplaceNetworkAclAssociationInput{
			AssociationId: aws.String(associationId),
			NetworkAclId:  aws.String(oldNetworkAclIdValue),
		}
		log.Debug().Msgf("Rolling back to old acl entry %+v", networkAclAssociationInput)
		replaceNetworkAclAssociationResponse, err := clientEc2.ReplaceNetworkAclAssociation(context.Background(), networkAclAssociationInput)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to rollback to old acl entry %+v", networkAclAssociationInput)
			errors = append(errors, err.Error())
		}
		log.Debug().Msgf("Rolled back to old acl entry  %+v", replaceNetworkAclAssociationResponse)
	}

	for networkAclId := range networkAclIdsCreadedBySteadybit {
		deleteNetworkAclInput := &ec2.DeleteNetworkAclInput{
			NetworkAclId: aws.String(networkAclId),
		}
		log.Debug().Msgf("Deleting network acl entry %+v", deleteNetworkAclInput)
		deleteNetworkAclResponse, err := clientEc2.DeleteNetworkAcl(context.Background(), deleteNetworkAclInput)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to delete network acl entry %+v", deleteNetworkAclInput)
			errors = append(errors, err.Error())
		}
		log.Debug().Msgf("Deleted network acl entry  %+v", deleteNetworkAclResponse)
	}
	if errors != nil {
		return extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to replace network ACL association: %s", strings.Join(errors, ", ")), nil))
	}
	return nil
}

func getTagsOfNacl(clientEc2 azBlackholeEC2Api, ctx context.Context, networkAclId string) (*[]types.TagDescription, error) {
	tagsOfNacl := make([]types.TagDescription, 0)

	paginator := ec2.NewDescribeTagsPaginator(clientEc2, &ec2.DescribeTagsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("resource-id"),
				Values: []string{networkAclId},
			},
			{
				Name:   aws.String("resource-type"),
				Values: []string{"network-acl"},
			},
		},
	})

	for paginator.HasMorePages() {
		describeTagsResult, err := paginator.NextPage(ctx)

		if err != nil {
			log.Error().Err(err).Msg("Failed to get network ACLs Tags")
			return nil, err
		}
		tagsOfNacl = append(tagsOfNacl, describeTagsResult.Tags...)
	}
	return &tagsOfNacl, nil
}

func getAllNACLsCreatedBySteadybit(clientEc2 azBlackholeEC2Api, ctx context.Context, executionId uuid.UUID) (*[]types.NetworkAclAssociation, error) {
	networkAclsAssociatedWithSubnets := make([]types.NetworkAclAssociation, 0)
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
		for _, networkAcl := range describeNetworkAclsResult.NetworkAcls {
			for _, association := range networkAcl.Associations {
				networkAclsAssociatedWithSubnets = append(networkAclsAssociatedWithSubnets, association)
			}
		}
	}
	return &networkAclsAssociatedWithSubnets, nil
}

func defaultClientProvider(account string) (azBlackholeEC2Api, azBlackholeImdsApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, nil, err
	}
	clientEc2 := ec2.NewFromConfig(awsAccount.AwsConfig)
	clientImds := imds.NewFromConfig(awsAccount.AwsConfig)
	if err != nil {
		return nil, nil, err
	}
	return clientEc2, clientImds, nil
}

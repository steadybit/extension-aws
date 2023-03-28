// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extaz

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"strings"
)

func RegisterAZAttackHandlers() {
	exthttp.RegisterHttpHandler("/az/attack/blackhole", exthttp.GetterAsHandler(getBlackholeAttackDescription))
	exthttp.RegisterHttpHandler("/az/attack/blackhole/prepare", prepareBlackhole)
	exthttp.RegisterHttpHandler("/az/attack/blackhole/start", startBlackhole)
	exthttp.RegisterHttpHandler("/az/attack/blackhole/stop", stopBlackhole)
}

func getBlackholeAttackDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          azBlackholeActionId,
		Label:       "Blackhole Availability Zone",
		Description: "Simulates an outage of an entire availability zone.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(azIcon),
		TargetType:  extutil.Ptr("zone"),
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
		Prepare: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/az/attack/blackhole/prepare",
		},
		Start: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/az/attack/blackhole/start",
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/az/attack/blackhole/stop",
		}),
	}
}

type BlackholeState struct {
	AgentAWSAccount     string
	ExtensionAwsAccount string
	TargetZone          string
	NetworkAclIds       []string
	OldNetworkAclIds    map[string]string
	TargetSubnets       map[string][]string
	AttackExecutionId   uuid.UUID
}

type AZBlackholeEC2Api interface {
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeNetworkAcls(ctx context.Context, params *ec2.DescribeNetworkAclsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkAclsOutput, error)
	CreateNetworkAcl(ctx context.Context, params *ec2.CreateNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclOutput, error)
	CreateNetworkAclEntry(ctx context.Context, params *ec2.CreateNetworkAclEntryInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclEntryOutput, error)
	ReplaceNetworkAclAssociation(ctx context.Context, params *ec2.ReplaceNetworkAclAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceNetworkAclAssociationOutput, error)
	DeleteNetworkAcl(ctx context.Context, params *ec2.DeleteNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkAclOutput, error)
	DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error)
}
type AZBlackholeImdsApi interface {
	GetInstanceIdentityDocument(
		ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options),
	) (
		*imds.GetInstanceIdentityDocumentOutput, error,
	)
}

func prepareBlackhole(w http.ResponseWriter, r *http.Request, body []byte) {
	agentAWSAccount := r.Header.Get("AgentAWSAccount")
	extensionRootAccountNumber := utils.Accounts.GetRootAccount().AccountNumber
	state, extKitErr := PrepareBlackhole(r.Context(), body, agentAWSAccount, extensionRootAccountNumber, func(account string) (AZBlackholeEC2Api, AZBlackholeImdsApi, error) {
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
	})
	if extKitErr != nil {
		exthttp.WriteError(w, *extKitErr)
		return
	}

	var convertedState action_kit_api.ActionState
	err := extconversion.Convert(*state, &convertedState)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state", err))
		return
	}

	exthttp.WriteBody(w, extutil.Ptr(action_kit_api.PrepareResult{
		State: convertedState,
	}))
}

func PrepareBlackhole(ctx context.Context, body []byte, agentAWSAccount string, extensionRootAccountNumber string, clientProvider func(account string) (AZBlackholeEC2Api, AZBlackholeImdsApi, error)) (*BlackholeState, *extension_kit.ExtensionError) {
	var request action_kit_api.PrepareActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

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
	clientEc2, clientImds, err := clientProvider(targetAccount[0])
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize AWS clients for AWS targetAccount %s", targetAccount[0]), err))
	}
	//Get Extension Account
	extensionAwsAccount := getExtensionAWSAccount(ctx, clientImds, extensionRootAccountNumber)

	if extensionAwsAccount == "" {
		return nil, extutil.Ptr(extension_kit.ToError("Could not get AWS Account of the extension. Attack is disabled to prevent an extension lockout.", nil))
	}
	if targetAccount[0] == extensionAwsAccount {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("The extension is running in the same AWS account (%s) as the target. Attack is disabled to prevent an extension lockout.", extensionAwsAccount), nil))
	}

	if agentAWSAccount == "" {
		return nil, extutil.Ptr(extension_kit.ToError("Could not get AWS Account of the agent. Attack is disabled to prevent an agent lockout.", nil))
	}
	if targetAccount[0] == agentAWSAccount {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("The agent is running in the same AWS account (%s) as the target. Attack is disabled to prevent an agent lockout.", extensionAwsAccount), nil))
	}

	// Get Target Subnets
	targetSubnets, err := getTargetSubnets(clientEc2, ctx, targetZone[0])
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to get subnets for zone %s", targetZone[0]), err))
	}

	return extutil.Ptr(BlackholeState{
		AgentAWSAccount:     agentAWSAccount,
		ExtensionAwsAccount: targetAccount[0],
		TargetZone:          targetZone[0],
		TargetSubnets:       targetSubnets,
		AttackExecutionId:   request.ExecutionId,
	}), nil
}

func getTargetSubnets(clientEc2 AZBlackholeEC2Api, ctx context.Context, targetZone string) (map[string][]string, error) {
	subnetResults := make(map[string][]string)
	var nextToken *string

	for {
		subnets, err := clientEc2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("availabilityZone"),
					Values: []string{targetZone},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to get subnets")
			return nil, err
		}
		for _, subnet := range subnets.Subnets {
			subnetResults[*subnet.VpcId] = append(subnetResults[*subnet.VpcId], *subnet.SubnetId)
		}
		log.Debug().Msgf("Found %d subnets in AZ %s for creating temporary ACL to block traffic: %+v", len(subnets.Subnets), targetZone, subnets.Subnets)
		if subnets.NextToken == nil {
			break
		}
		nextToken = subnets.NextToken
	}
	return subnetResults, nil
}

func getExtensionAWSAccount(ctx context.Context, clientImds AZBlackholeImdsApi, extensionRootAccountNumber string) string {

	ec2MetadataAccountId := getAccountNumberByEC2Metadata(ctx, clientImds)
	resultAccountNumber := ec2MetadataAccountId

	if ec2MetadataAccountId == "" && extensionRootAccountNumber != "" {
		resultAccountNumber = extensionRootAccountNumber
		log.Info().Msgf("Agent AWS Account %s provided by STS get-caller-identity", extensionRootAccountNumber)
	}

	if ec2MetadataAccountId != "" && extensionRootAccountNumber != "" && extensionRootAccountNumber != ec2MetadataAccountId {
		log.Error().Msgf("Agent AWS Account %s provided by EC2-Metadata-Service differs from the one provided by STS get-caller-identity %s", ec2MetadataAccountId, extensionRootAccountNumber)
		return ""
	}
	return resultAccountNumber
}

func getAccountNumberByEC2Metadata(ctx context.Context, clientImds AZBlackholeImdsApi) string {
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

func startBlackhole(w http.ResponseWriter, r *http.Request, body []byte) {
	clientProvider := func(account string) (AZBlackholeEC2Api, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return ec2.NewFromConfig(awsAccount.AwsConfig), nil
	}
	state, extKitErr := StartBlackhole(r.Context(), body, clientProvider)
	if extKitErr != nil {
		rollbackBlackholeViaTags(state.ExtensionAwsAccount, state.AttackExecutionId.String(), r.Context(), clientProvider)
		exthttp.WriteError(w, *extKitErr)
		return
	}
	var convertedState action_kit_api.ActionState
	err := extconversion.Convert(*state, &convertedState)
	if err != nil {
		rollbackBlackholeViaTags(state.ExtensionAwsAccount, state.AttackExecutionId.String(), r.Context(), clientProvider)
		exthttp.WriteError(w, extension_kit.ToError("Failed to convert attack state", err))
		return
	}

	exthttp.WriteBody(w, extutil.Ptr(action_kit_api.StartResult{
		State: extutil.Ptr(convertedState),
	}))
}

func StartBlackhole(ctx context.Context, body []byte, clientProvider func(account string) (AZBlackholeEC2Api, error)) (*BlackholeState, *extension_kit.ExtensionError) {
	var request action_kit_api.StartActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	var state BlackholeState
	err = extconversion.Convert(request.State, &state)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse attack state", err))
	}

	clientEc2, err := clientProvider(state.ExtensionAwsAccount)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", state.ExtensionAwsAccount), err))
	}
	log.Info().Msgf("Starting AZ Blackhole attack against AWS account %s", state.ExtensionAwsAccount)
	log.Debug().Msgf("Attack state: %+v", state)
	state.OldNetworkAclIds = make(map[string]string)
	for vpcId, subnetIds := range state.TargetSubnets {
		log.Info().Msgf("Creating temporary ACL to block traffic in VPC %s", vpcId)
		//Find existing to be modified network acl associations matching the subnetIds in the given VPC
		desiredAclAssociations := getNetworkAclAssociations(ctx, clientEc2, vpcId, subnetIds)
		log.Info().Msgf("Found %d network ACL associations to modify", len(desiredAclAssociations))
		//Create new network acl
		networkAclId, err := createNetworkAcl(ctx, &state, clientEc2, vpcId, desiredAclAssociations)
		if err != nil {
			return &state, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to create network ACL for VPC %s", vpcId), err))
		}
		//Replace the association IDs for the above subnets with the new network acl which will deny all traffic for those subnets in that AZ
		err = replaceNetworkAclAssociations(ctx, &state, clientEc2, desiredAclAssociations, networkAclId)
		if err != nil {
			return &state, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to replace network ACL associations for VPC %s", vpcId), err))
		}
	}

	return &state, nil
}

func replaceNetworkAclAssociations(ctx context.Context, state *BlackholeState, clientEc2 AZBlackholeEC2Api, desiredAclAssociations []types.NetworkAclAssociation, networkAclId string) error {
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

func createNetworkAcl(ctx context.Context, state *BlackholeState, clientEc2 AZBlackholeEC2Api, vpcId string, desiredAclAssociations []types.NetworkAclAssociation) (string, error) {
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

func createNetworkAclEntry(ctx context.Context, clientEc2 AZBlackholeEC2Api, networkAclId string, ruleNumber int, egress bool) {
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

func getNetworkAclAssociations(ctx context.Context, clientEc2 AZBlackholeEC2Api, vpcId string, targetSubnetIds []string) []types.NetworkAclAssociation {
	desiredAclAssociations := make([]types.NetworkAclAssociation, 0, len(targetSubnetIds))
	networkAclsAssociatedWithSubnets := make([]types.NetworkAclAssociation, 0, len(targetSubnetIds))
	var nextToken *string
	for {

		describeNetworkAclsResult, err := clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("association.subnet-id"),
					Values: targetSubnetIds,
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to get network ACLs")
			return nil
		}
		for _, networkAcl := range describeNetworkAclsResult.NetworkAcls {
			for _, association := range networkAcl.Associations {
				networkAclsAssociatedWithSubnets = append(networkAclsAssociatedWithSubnets, association)
			}
		}
		if describeNetworkAclsResult.NextToken == nil {
			break
		} else {
			nextToken = describeNetworkAclsResult.NextToken
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
	return desiredAclAssociations
}

func stopBlackhole(w http.ResponseWriter, req *http.Request, body []byte) {
	result, err := StopBlackhole(body, req.Context(), func(account string) (AZBlackholeEC2Api, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return ec2.NewFromConfig(awsAccount.AwsConfig), nil
	})
	if err != nil {
		exthttp.WriteError(w, *err)
	} else {
		exthttp.WriteBody(w, result)
	}
}

func StopBlackhole(body []byte, ctx context.Context, clientProvider func(account string) (AZBlackholeEC2Api, error)) (*action_kit_api.StopResult, *extension_kit.ExtensionError) {
	var request action_kit_api.ActionStatusRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	var state BlackholeState
	err = extconversion.Convert(request.State, &state)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse state", err))
	}

	return rollbackBlackholeViaTags(state.ExtensionAwsAccount, state.AttackExecutionId.String(), ctx, clientProvider)
}

func rollbackBlackholeViaTags(awsAccount string, executionId string, ctx context.Context, clientProvider func(account string) (AZBlackholeEC2Api, error)) (*action_kit_api.StopResult, *extension_kit.ExtensionError) {
	clientEc2, err := clientProvider(awsAccount)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", awsAccount), err))
	}

	// get all network ACLs created by Steadybit
	networkAclsAssociatedWithSubnets, err := getAllNACLsCreatedBySteadybit(clientEc2, ctx, executionId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network ACLs created by Steadybit")
		return nil, extutil.Ptr(extension_kit.ToError("Failed to get network ACLs created by Steadybit", err))
	}

	networkAclIdsCreadedBySteadybit := make([]string, 0) // network ACL IDs created by Steadybit to be deleted later
	oldNetworkAclIds := make(map[string]string)          // key: new association ID, value: old network ACL ID --> to be used to rollback the network ACL association

	var errors []string

	for _, networkAclsAssociatedWithSubnet := range networkAclsAssociatedWithSubnets {
		networkAclIdsCreadedBySteadybit = append(networkAclIdsCreadedBySteadybit, *networkAclsAssociatedWithSubnet.NetworkAclId)
		// find tags of the network ACL
		tags, err := getTagsOfNacl(clientEc2, ctx, *networkAclsAssociatedWithSubnet.NetworkAclId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get tags of network ACL")
			errors = append(errors, err.Error())
		}

		for _, tag := range tags {
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

	for _, networkAclId := range networkAclIdsCreadedBySteadybit {
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
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to replace network ACL association: %s", strings.Join(errors, ", ")), nil))
	}
	return extutil.Ptr(action_kit_api.StopResult{}), nil
}

func getTagsOfNacl(clientEc2 AZBlackholeEC2Api, ctx context.Context, networkAclId string) ([]types.TagDescription, error) {
	tagsOfNacl := make([]types.TagDescription, 0)
	var nextToken *string
	for {

		describeTagsResult, err := clientEc2.DescribeTags(ctx, &ec2.DescribeTagsInput{
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
			NextToken: nextToken,
			//MaxResults: aws.Int32(2),
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to get network ACLs Tags")
			return nil, err
		}
		tagsOfNacl = append(tagsOfNacl, describeTagsResult.Tags...)
		if describeTagsResult.NextToken != nil {
			nextToken = describeTagsResult.NextToken
		} else {
			break
		}
	}
	return tagsOfNacl, nil
}

func getAllNACLsCreatedBySteadybit(clientEc2 AZBlackholeEC2Api, ctx context.Context, executionId string) ([]types.NetworkAclAssociation, error) {
	networkAclsAssociatedWithSubnets := make([]types.NetworkAclAssociation, 0)
	var nextToken *string
	for {
		describeNetworkAclsResult, err := clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("Name"),
					Values: []string{"created by steadybit"},
				}, {
					Name:   aws.String("steadybit-attack-execution-id"),
					Values: []string{executionId},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to get network ACLs")
			return nil, err
		}
		for _, networkAcl := range describeNetworkAclsResult.NetworkAcls {
			for _, association := range networkAcl.Associations {
				networkAclsAssociatedWithSubnets = append(networkAclsAssociatedWithSubnets, association)
			}
		}
		if describeNetworkAclsResult.NextToken == nil {
			break
		} else {
			nextToken = describeNetworkAclsResult.NextToken
		}
	}
	return networkAclsAssociatedWithSubnets, nil
}

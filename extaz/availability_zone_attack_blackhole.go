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
	"github.com/aws/aws-sdk-go-v2/service/sts"
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

const DoADryRun = true // TODO remove dry run
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
		Parameters:  []action_kit_api.ActionParameter{},
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
	AwsAccount        string
	TargetZone        string
	NetworkAclIds     []string
	OldNetworkAclIds  map[string]string
	TargetSubnets     map[string][]string
	AttackExecutionId string
}

type AZBlackholeEC2Api interface {
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeNetworkAcls(ctx context.Context, params *ec2.DescribeNetworkAclsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkAclsOutput, error)
	CreateNetworkAcl(ctx context.Context, params *ec2.CreateNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclOutput, error)
	CreateNetworkAclEntry(ctx context.Context, params *ec2.CreateNetworkAclEntryInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclEntryOutput, error)
	ReplaceNetworkAclAssociation(ctx context.Context, params *ec2.ReplaceNetworkAclAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceNetworkAclAssociationOutput, error)
	DeleteNetworkAcl(ctx context.Context, params *ec2.DeleteNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkAclOutput, error)
}
type AZBlackholeImdsApi interface {
	GetInstanceIdentityDocument(
		ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options),
	) (
		*imds.GetInstanceIdentityDocumentOutput, error,
	)
}
type AZBlackholeStsApi interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

func prepareBlackhole(w http.ResponseWriter, r *http.Request, body []byte) {
	agentAWSAccount := r.Header.Get("AgentAWSAccount")
	state, extKitErr := PrepareBlackhole(body, r.Context(), agentAWSAccount, func(account string) (AZBlackholeEC2Api, AZBlackholeImdsApi, AZBlackholeStsApi, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, nil, nil, err
		}
		clientEc2 := ec2.NewFromConfig(awsAccount.AwsConfig)
		clientImds := imds.NewFromConfig(awsAccount.AwsConfig)
		clientSts := sts.NewFromConfig(awsAccount.AwsConfig)
		if err != nil {
			return nil, nil, nil, err
		}
		return clientEc2, clientImds, clientSts, nil
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

func PrepareBlackhole(body []byte, ctx context.Context, agentAWSAccount string, clientProvider func(account string) (AZBlackholeEC2Api, AZBlackholeImdsApi, AZBlackholeStsApi, error)) (*BlackholeState, *extension_kit.ExtensionError) {
	var request action_kit_api.PrepareActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	// Get Target Attributes
	targetAccount := request.Target.Attributes["aws.targetAccount"]
	if targetAccount == nil || len(targetAccount) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.targetAccount' target attribute.", nil))
	}

	targetZone := request.Target.Attributes["aws.zone"]
	if targetZone == nil || len(targetZone) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.zone' target attribute.", nil))
	}

	// Get AWS Clients
	clientEc2, clientImds, clientSts, err := clientProvider(targetAccount[0])
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize AWS clients for AWS targetAccount %s", targetAccount[0]), err))
	}
	//Get Extension Account
	extensionAwsAccount := getExtensionAWSAccount(ctx, clientImds, clientSts)
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
		AwsAccount:        targetAccount[0],
		TargetZone:        targetZone[0],
		TargetSubnets:     targetSubnets,
		AttackExecutionId: uuid.New().String(),
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
		log.Debug().Msgf("Found %d subnets in AZ %s for creating temporary ACL to block traffic: %s", len(subnets.Subnets), targetZone[0], subnets.Subnets)
		if subnets.NextToken == nil {
			break
		}
		nextToken = subnets.NextToken
	}
	return subnetResults, nil
}

func getExtensionAWSAccount(ctx context.Context, clientImds AZBlackholeImdsApi, clientSts AZBlackholeStsApi) string {
	output, err := clientImds.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		log.Debug().Err(err).Msg("Unable to get AWS Account ID by EC2-Metadata-Service.")
		return ""
	}
	if output.AccountID != "" {
		log.Info().Msgf("Agent AWS Account %s provided by EC2-Metadata-Service", output.AccountID)
		return output.AccountID
	}

	identity, err := clientSts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Debug().Err(err).Msg("Unable to get AWS Account ID by STS get-caller-identity.")
		return ""
	}
	log.Info().Msgf("Agent AWS Account %s provided by STS get-caller-identity", *identity.Account)
	return *identity.Account
}

func startBlackhole(w http.ResponseWriter, r *http.Request, body []byte) {
	state, extKitErr := StartBlackhole(r.Context(), body, func(account string) (AZBlackholeEC2Api, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return ec2.NewFromConfig(awsAccount.AwsConfig), nil
	})
	if extKitErr != nil {
		rollbackViaLabel(r.Context(), state)
		exthttp.WriteError(w, *extKitErr)
		return
	}
	var convertedState action_kit_api.ActionState
	err := extconversion.Convert(*state, &convertedState)
	if err != nil {
		rollbackViaLabel(r.Context(), state)
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

	clientEc2, err := clientProvider(state.AwsAccount)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", state.AwsAccount), err))
	}
	log.Info().Msgf("Starting AZ Blackhole attack against AWS account %s", state.AwsAccount)
	log.Debug().Msgf("Attack state: %+v", state)

	for vpcId, subnetIds := range state.TargetSubnets {
		log.Info().Msgf("Creating temporary ACL to block traffic in VPC %s", vpcId)
		//Find existing to be modified network acl associations matching the subnetIds in the given VPC
		desiredAclAssociations := getNetworkAclAssociations(ctx, clientEc2, vpcId, subnetIds)
		log.Info().Msgf("Found %d network ACL associations to modify", len(desiredAclAssociations))
		//Create new network acl
		networkAclId, err := createNetworkAcl(ctx, &state, clientEc2, vpcId, desiredAclAssociations)
		if err != nil {
			return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to create network ACL for VPC %s", vpcId), err))
		}
		//Replace the association IDs for the above subnets with the new network acl which will deny all traffic for those subnets in that AZ
		err = replaceNetworkAclAssociations(ctx, &state, clientEc2, desiredAclAssociations, networkAclId)
		if err != nil {
			return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to replace network ACL associations for VPC %s", vpcId), err))
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
			DryRun:        aws.Bool(DoADryRun),
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
	if state.AttackExecutionId != "" {
		return "", fmt.Errorf("AttackExecutionId is empty")
	}

	tagList = append(tagList, types.Tag{
		Key:   aws.String("steadybit-attack-execution-id"),
		Value: aws.String(state.AttackExecutionId),
	})

	for _, desiredAclAssociation := range desiredAclAssociations {
		tagList = append(tagList, types.Tag{
			Key: aws.String("steadybit-replaced" + *desiredAclAssociation.SubnetId),
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
		DryRun: aws.Bool(DoADryRun),
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
	describeNetworkAclsResult, err := clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("association.subnet-id"),
				Values: targetSubnetIds,
			},
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network ACLs")
		return nil
	}
	desiredAclAssociations := make([]types.NetworkAclAssociation, 0, len(describeNetworkAclsResult.NetworkAcls))
	networkAclsAssociatedWithSubnets := make([]types.NetworkAclAssociation, 0, len(describeNetworkAclsResult.NetworkAcls))
	for _, networkAcl := range describeNetworkAclsResult.NetworkAcls {
		for _, association := range networkAcl.Associations {
			networkAclsAssociatedWithSubnets = append(networkAclsAssociatedWithSubnets, association)
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

func stopBlackhole(w http.ResponseWriter, _ *http.Request, body []byte) {
	result, err := StopBlackhole(body, func(account string) (AZBlackholeEC2Api, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return ec2.NewFromConfig(awsAccount.AwsConfig), nil
	}))
	if err != nil {
		exthttp.WriteError(w, *err)
	} else {
		exthttp.WriteBody(w, result)
	}
}
func StopBlackhole(body []byte, clientProvider func(account string) (AZBlackholeEC2Api, error)) (*action_kit_api.StopResult, *extension_kit.ExtensionError) {
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

	if state.NetworkAclIds == nil || state.OldNetworkAclIds == nil {
		log.Error().Msg("NetworkAclIds or OldNetworkAclIds is nil")
	}

	clientEc2, err := clientProvider(state.AwsAccount)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", state.AwsAccount), err))
	}

	var errors []string
	for oldNetworkAclIdKey, oldNetworkAclIdValue := range state.OldNetworkAclIds {
		networkAclAssociationInput := &ec2.ReplaceNetworkAclAssociationInput{
			AssociationId: aws.String(oldNetworkAclIdKey),
			NetworkAclId:  aws.String(oldNetworkAclIdValue),
			DryRun:        aws.Bool(DoADryRun),
		}
		log.Debug().Msgf("Rolling back to old acl entry %+v", networkAclAssociationInput)
		replaceNetworkAclAssociationResponse, err := clientEc2.ReplaceNetworkAclAssociation(context.Background(), networkAclAssociationInput)
		if err != nil {
			errors = append(errors, err.Error())
		}
		log.Debug().Msgf("Rolled back to old acl entry  %+v", replaceNetworkAclAssociationResponse)
	}

	for _, networkAclId := range state.NetworkAclIds {
		deleteNetworkAclInput := &ec2.DeleteNetworkAclInput{
			NetworkAclId: aws.String(networkAclId),
			DryRun:       aws.Bool(DoADryRun),
		}
		log.Debug().Msgf("Deleting network acl entry %+v", deleteNetworkAclInput)
		deleteNetworkAclResponse, err := clientEc2.DeleteNetworkAcl(context.Background(), deleteNetworkAclInput)
		if err != nil {
			errors = append(errors, err.Error())
		}
		log.Debug().Msgf("Deleted network acl entry  %+v", deleteNetworkAclResponse)
	}
	if errors != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to replace network ACL association: %s", strings.Join(errors, ", ")), nil))
	}
	rollbackViaLabel(context.Background(), &state)
	return extutil.Ptr(action_kit_api.StopResult{}), nil
}

func rollbackViaLabel(context.Context, *BlackholeState) (*action_kit_api.StopResult, *extension_kit.ExtensionError) {
	// TO DO: Implement rollback via label
	return nil, nil
}

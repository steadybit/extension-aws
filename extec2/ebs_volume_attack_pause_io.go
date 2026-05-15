// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

// EbsPauseIoAttackState captures enough state to freeze + later unfreeze the filesystem on the
// EC2 instance the volume is attached to. SSM-driven; works only on Linux instances with the SSM
// agent registered as Online and a mounted filesystem on the EBS volume.
type EbsPauseIoAttackState struct {
	Account          string
	Region           string
	DiscoveredByRole *string
	VolumeId         string
	InstanceId       string
	AwsDeviceName    string // e.g. "/dev/sdf" — the AWS-side attachment device name
	StartCommandId   string
}

type ebsPauseIoApi interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
	SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	GetCommandInvocation(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
}

type ebsPauseIoAttack struct {
	clientProvider func(account string, region string, role *string) (ebsPauseIoApi, error)
}

var (
	_ action_kit_sdk.Action[EbsPauseIoAttackState]         = (*ebsPauseIoAttack)(nil)
	_ action_kit_sdk.ActionWithStop[EbsPauseIoAttackState] = (*ebsPauseIoAttack)(nil)
)

func NewEbsPauseIoAttack() action_kit_sdk.ActionWithStop[EbsPauseIoAttackState] {
	return &ebsPauseIoAttack{clientProvider: defaultEbsPauseIoClientProvider}
}

func (a *ebsPauseIoAttack) NewEmptyState() EbsPauseIoAttackState {
	return EbsPauseIoAttackState{}
}

func (a *ebsPauseIoAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    ebsPauseIoActionId,
		Label: "Trigger AWS EBS Volume Pause IO",
		Description: "Freezes every mounted filesystem on the target EBS volume via SSM + Linux fsfreeze. " +
			"All reads and writes against the filesystem(s) block at the VFS layer until the attack stops. " +
			"Requires the volume to be attached to a Linux EC2 instance with the SSM agent registered as Online, and with at least one mounted filesystem on the volume. " +
			"Raw-block-device workloads (e.g. databases bypassing the filesystem) are not affected — for those, AWS FIS aws:ebs:pause-volume-io is the only equivalent.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    new(ebsIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: ebsTargetType,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by volume id",
					Description: new("Find an EBS volume by its volume id"),
					Query:       "aws.ebs.volume.id=\"\"",
				},
			}),
		}),
		Technology:  new("AWS"),
		Category:    new("EBS"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("How long the filesystem stays frozen. Unfrozen on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("30s"),
				Order:        new(1),
				Required:     new(true),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *ebsPauseIoAttack) Prepare(ctx context.Context, state *EbsPauseIoAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.VolumeId = extutil.MustHaveValue(request.Target.Attributes, "aws.ebs.volume.id")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")

	instanceIds := request.Target.Attributes["aws.ebs.volume.attachment.instance-id"]
	if len(instanceIds) == 0 || instanceIds[0] == "" {
		return nil, extension_kit.ToError(fmt.Sprintf("EBS volume %s is not attached to an instance — pause-IO needs the workload's instance.", state.VolumeId), nil)
	}
	state.InstanceId = instanceIds[0]
	if devs := request.Target.Attributes["aws.ebs.volume.attachment.device"]; len(devs) > 0 {
		state.AwsDeviceName = devs[0]
	}

	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize AWS clients for account %s", state.Account), err)
	}

	// Reject Windows. fsfreeze is Linux-only.
	descOut, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{state.InstanceId}})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe EC2 instance %s", state.InstanceId), err)
	}
	if len(descOut.Reservations) == 0 || len(descOut.Reservations[0].Instances) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("EC2 instance %s not found", state.InstanceId), nil)
	}
	inst := descOut.Reservations[0].Instances[0]
	if strings.EqualFold(aws.ToString(inst.PlatformDetails), "Windows") || strings.EqualFold(string(inst.Platform), "windows") {
		return nil, extension_kit.ToError(fmt.Sprintf("EC2 instance %s runs Windows — the EBS pause-IO attack is Linux-only (uses fsfreeze).", state.InstanceId), nil)
	}

	// Require SSM agent Online — failing here gives a clearer error than a SendCommand failure later.
	ssmOut, err := client.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{Key: aws.String("InstanceIds"), Values: []string{state.InstanceId}},
		},
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to query SSM for instance %s", state.InstanceId), err)
	}
	if len(ssmOut.InstanceInformationList) == 0 || ssmOut.InstanceInformationList[0].PingStatus != ssmtypes.PingStatusOnline {
		return nil, extension_kit.ToError(fmt.Sprintf("SSM agent on instance %s is not Online (required to run fsfreeze). Confirm the agent is installed and registered.", state.InstanceId), nil)
	}

	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Will fsfreeze mounted filesystem(s) on volume %s (attached to %s as %s).", state.VolumeId, state.InstanceId, state.AwsDeviceName),
		}}),
	}, nil
}

func (a *ebsPauseIoAttack) Start(ctx context.Context, state *EbsPauseIoAttackState) (*action_kit_api.StartResult, error) {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize SSM client for account %s", state.Account), err)
	}
	out, err := client.SendCommand(ctx, buildSendCommandInput(state, ebsFreezeScript))
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to send freeze command to instance %s", state.InstanceId), err)
	}
	if out.Command == nil || out.Command.CommandId == nil {
		return nil, extension_kit.ToError("SSM SendCommand returned no CommandId", nil)
	}
	state.StartCommandId = *out.Command.CommandId

	if err := waitForCommand(ctx, client, state.InstanceId, state.StartCommandId); err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Freeze command on instance %s did not complete cleanly", state.InstanceId), err)
	}

	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Froze filesystem(s) on EBS volume %s via instance %s (SSM command %s).", state.VolumeId, state.InstanceId, state.StartCommandId),
		}}),
	}, nil
}

func (a *ebsPauseIoAttack) Stop(ctx context.Context, state *EbsPauseIoAttackState) (*action_kit_api.StopResult, error) {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize SSM client for account %s", state.Account), err)
	}
	out, err := client.SendCommand(ctx, buildSendCommandInput(state, ebsUnfreezeScript))
	if err != nil {
		log.Error().Err(err).Msgf("Failed to send unfreeze command to instance %s for volume %s", state.InstanceId, state.VolumeId)
		return nil, extension_kit.ToError(fmt.Sprintf(
			"Failed to send unfreeze command to instance %s for volume %s. Manual recovery: "+
				`aws ssm send-command --instance-ids %s --document-name AWS-RunShellScript --parameters 'commands=["sudo fsfreeze --unfreeze /<mountpoint>"]'`,
			state.InstanceId, state.VolumeId, state.InstanceId), err)
	}
	if out.Command == nil || out.Command.CommandId == nil {
		return nil, extension_kit.ToError("SSM SendCommand returned no CommandId for unfreeze", nil)
	}
	if err := waitForCommand(ctx, client, state.InstanceId, *out.Command.CommandId); err != nil {
		log.Warn().Err(err).Msgf("Unfreeze command on instance %s did not report Success — filesystem may already be unfrozen", state.InstanceId)
	}

	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Unfroze filesystem(s) on EBS volume %s via instance %s.", state.VolumeId, state.InstanceId),
		}}),
	}, nil
}

func buildSendCommandInput(state *EbsPauseIoAttackState, script string) *ssm.SendCommandInput {
	awsDevSuffix := ""
	if state.AwsDeviceName != "" {
		// Best-effort: from "/dev/sdf" derive "f". The script falls back gracefully when this fails.
		awsDevSuffix = state.AwsDeviceName[strings.LastIndexByte(state.AwsDeviceName, '/')+1:]
		awsDevSuffix = strings.TrimPrefix(awsDevSuffix, "sd")
		awsDevSuffix = strings.TrimPrefix(awsDevSuffix, "xvd")
	}
	return &ssm.SendCommandInput{
		InstanceIds:  []string{state.InstanceId},
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String(fmt.Sprintf("steadybit ebs-volume.pause-io %s", state.VolumeId)),
		Parameters: map[string][]string{
			"commands": {
				fmt.Sprintf("export VOLUME_ID=%q", state.VolumeId),
				fmt.Sprintf("export AWS_DEV_SUFFIX=%q", awsDevSuffix),
				script,
			},
		},
	}
}

// waitForCommand polls GetCommandInvocation until the command reaches a terminal state or ~30s
// elapses. Returns an error if the command terminated unsuccessfully.
func waitForCommand(ctx context.Context, client ebsPauseIoApi, instanceId, commandId string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		out, err := client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
			CommandId:  &commandId,
			InstanceId: &instanceId,
		})
		if err != nil {
			// SSM returns InvocationDoesNotExist briefly after SendCommand; tolerate it.
			var notFound *ssmtypes.InvocationDoesNotExist
			if errors.As(err, &notFound) && time.Now().Before(deadline) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return err
		}
		switch out.Status {
		case ssmtypes.CommandInvocationStatusSuccess:
			return nil
		case ssmtypes.CommandInvocationStatusFailed, ssmtypes.CommandInvocationStatusCancelled, ssmtypes.CommandInvocationStatusTimedOut:
			return fmt.Errorf("ssm command %s ended in status %s: %s", commandId, out.Status, aws.ToString(out.StandardErrorContent))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for SSM command %s on %s (last status %s)", commandId, instanceId, out.Status)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// ebsFreezeScript freezes every mounted filesystem found on the EBS volume identified by $VOLUME_ID.
// It resolves the kernel block device via the NVMe controller serial number (Nitro instances) or
// falls back to the Xen device name. Records the frozen mountpoints to a state file so Stop can
// unfreeze the exact same set even if Start raced with a remount.
const ebsFreezeScript = `set -euo pipefail
VOLUME_ID_SHORT="${VOLUME_ID#vol-}"
DEV=""
for d in /dev/nvme*n1; do
  [ -b "$d" ] || continue
  SERIAL=$(sudo nvme id-ctrl "$d" 2>/dev/null | awk -F': ' '/sn[[:space:]]*:/ {gsub(/-/,"",$2); print $2; exit}')
  if [ -n "$SERIAL" ] && echo "$SERIAL" | grep -q "$VOLUME_ID_SHORT"; then DEV="$d"; break; fi
done
if [ -z "$DEV" ] && [ -n "${AWS_DEV_SUFFIX:-}" ] && [ -b "/dev/xvd${AWS_DEV_SUFFIX}" ]; then DEV="/dev/xvd${AWS_DEV_SUFFIX}"; fi
if [ -z "$DEV" ]; then echo "could not resolve block device for $VOLUME_ID" >&2; exit 2; fi
MOUNTS=$(lsblk -nrpo MOUNTPOINTS "$DEV" 2>/dev/null | tr ',' '\n' | grep -v '^$' | sort -u || true)
if [ -z "$MOUNTS" ]; then echo "device $DEV has no mounted filesystem to freeze" >&2; exit 3; fi
while IFS= read -r MP; do sudo fsfreeze --freeze "$MP"; echo "froze $MP"; done <<EOF
$MOUNTS
EOF
echo "$MOUNTS" | sudo tee /var/run/steadybit-frozen-mounts-"$VOLUME_ID" >/dev/null
`

// ebsUnfreezeScript reads the state file written by ebsFreezeScript and unfreezes each recorded
// mountpoint. Idempotent — exits 0 if there is nothing to unfreeze.
const ebsUnfreezeScript = `set -euo pipefail
STATE_FILE="/var/run/steadybit-frozen-mounts-$VOLUME_ID"
if [ ! -r "$STATE_FILE" ]; then echo "no frozen-mount state for $VOLUME_ID; nothing to unfreeze" >&2; exit 0; fi
while IFS= read -r MP; do
  [ -n "$MP" ] || continue
  sudo fsfreeze --unfreeze "$MP" || echo "warn: unfreeze $MP failed (already unfrozen?)" >&2
done < "$STATE_FILE"
sudo rm -f "$STATE_FILE"
`

func defaultEbsPauseIoClientProvider(account string, region string, role *string) (ebsPauseIoApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return &combinedEc2SsmClient{
		ec2: ec2.NewFromConfig(awsAccess.AwsConfig),
		ssm: ssm.NewFromConfig(awsAccess.AwsConfig),
	}, nil
}

// combinedEc2SsmClient bundles EC2 + SSM clients behind a single interface so the attack's
// clientProvider has one type to swap in tests.
type combinedEc2SsmClient struct {
	ec2 *ec2.Client
	ssm *ssm.Client
}

func (c *combinedEc2SsmClient) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return c.ec2.DescribeInstances(ctx, params, optFns...)
}
func (c *combinedEc2SsmClient) DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	return c.ssm.DescribeInstanceInformation(ctx, params, optFns...)
}
func (c *combinedEc2SsmClient) SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	return c.ssm.SendCommand(ctx, params, optFns...)
}
func (c *combinedEc2SsmClient) GetCommandInvocation(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	return c.ssm.GetCommandInvocation(ctx, params, optFns...)
}

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package main

import (
	"context"
	_ "github.com/KimMachineGun/automemlimit" // By default, it sets `GOMEMLIMIT` to 90% of cgroup's memory limit.
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/extec2"
	"github.com/steadybit/extension-aws/v2/extecs"
	"github.com/steadybit/extension-aws/v2/extelasticache"
	"github.com/steadybit/extension-aws/v2/extelb"
	"github.com/steadybit/extension-aws/v2/extfis"
	"github.com/steadybit/extension-aws/v2/extlambda"
	"github.com/steadybit/extension-aws/v2/extmsk"
	"github.com/steadybit/extension-aws/v2/extrds"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthealth"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extruntime"
	"github.com/steadybit/extension-kit/extsignals"
	_ "go.uber.org/automaxprocs" // Importing automaxprocs automatically adjusts GOMAXPROCS.
	"os"
	"os/signal"
	"syscall"
)

func main() {
	extlogging.InitZeroLog()
	extbuild.PrintBuildInformation()
	extruntime.LogRuntimeInformation(zerolog.DebugLevel)
	exthealth.StartProbes(8086)
	exthealth.SetReady(false)

	ctx := context.Background()
	awsConfigForRootAccount, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to load AWS configuration")
	}

	config.ParseConfiguration(awsConfigForRootAccount.Region)

	utils.InitializeAwsAccess(config.Config, awsConfigForRootAccount)
	extec2.InitializeEc2Util()

	ctx, cancel := SignalCanceledContext()

	registerHandlers(ctx)

	extsignals.AddSignalHandler(extsignals.SignalHandler{
		Handler: func(signal os.Signal) {
			cancel()
		},
		Order: extsignals.OrderStopCustom,
		Name:  "custom-extension-aws",
	})
	extsignals.ActivateSignalHandlers()

	action_kit_sdk.RegisterCoverageEndpoints()
	exthealth.SetReady(true)

	exthttp.Listen(exthttp.ListenOpts{
		Port: 8085,
	})
}

func SignalCanceledContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, func() {
		signal.Stop(c)
		cancel()
	}
}

func registerHandlers(ctx context.Context) {
	cfg := config.Config
	discovery_kit_sdk.Register(utils.NewCommonAttributeDescriber())

	if !cfg.DiscoveryDisabledRds {
		discovery_kit_sdk.Register(extrds.NewRdsInstanceDiscovery(ctx))
		action_kit_sdk.RegisterAction(extrds.NewRdsInstanceRebootAttack())
		action_kit_sdk.RegisterAction(extrds.NewRdsInstanceStopAttack())

		discovery_kit_sdk.Register(extrds.NewRdsClusterDiscovery(ctx))
		action_kit_sdk.RegisterAction(extrds.NewRdsClusterFailoverAttack())
	}

	if !cfg.DiscoveryDisabledZone {
		discovery_kit_sdk.Register(extec2.NewAzDiscovery(ctx))
		action_kit_sdk.RegisterAction(extec2.NewAzBlackholeAction())
	}

	if !cfg.DiscoveryDisabledSubnet {
		discovery_kit_sdk.Register(extec2.NewSubnetDiscovery(ctx))
		action_kit_sdk.RegisterAction(extec2.NewSubnetBlackholeAction())
	}

	if !cfg.DiscoveryDisabledEc2 {
		discovery_kit_sdk.Register(extec2.NewEc2InstanceDiscovery(ctx))
		action_kit_sdk.RegisterAction(extec2.NewEc2InstanceStateAction())
	}

	if !cfg.DiscoveryDisabledFis {
		discovery_kit_sdk.Register(extfis.NewFisTemplateDiscovery(ctx))
		action_kit_sdk.RegisterAction(extfis.NewFisExperimentAction())
	}

	if !cfg.DiscoveryDisabledMsk {
		discovery_kit_sdk.Register(extmsk.NewMskClusterDiscovery(ctx))
		action_kit_sdk.RegisterAction(extmsk.NewMskRebootBrokerAttack())
	}

	if !cfg.DiscoveryDisabledLambda {
		discovery_kit_sdk.Register(extlambda.NewLambdaDiscovery(ctx))
		action_kit_sdk.RegisterAction(extlambda.NewInjectStatusCodeAction())
		action_kit_sdk.RegisterAction(extlambda.NewInjectExceptionAction())
		action_kit_sdk.RegisterAction(extlambda.NewInjectLatencyAction())
		action_kit_sdk.RegisterAction(extlambda.NewFillDiskspaceAction())
		action_kit_sdk.RegisterAction(extlambda.NewDenylistAction())
	}

	if !cfg.DiscoveryDisabledEcs {
		serviceDiscoveryPoller := extecs.NewServiceDescriptionPoller()
		serviceDiscoveryPoller.Start(ctx)

		discovery_kit_sdk.Register(extecs.NewEcsTaskDiscovery(ctx))
		discovery_kit_sdk.Register(extecs.NewEcsServiceDiscovery(ctx))
		action_kit_sdk.RegisterAction(extecs.NewEcsTaskStopAction())
		action_kit_sdk.RegisterAction(extecs.NewEcsServiceScaleAction())
		action_kit_sdk.RegisterAction(extecs.NewEcsTaskStopProcessAction())
		action_kit_sdk.RegisterAction(extecs.NewEcsTaskStressCpuAction())
		action_kit_sdk.RegisterAction(extecs.NewEcsTaskStressMemoryAction())
		action_kit_sdk.RegisterAction(extecs.NewEcsTaskStressIoAction())
		action_kit_sdk.RegisterAction(extecs.NewEcsTaskFillDiskAction())
		action_kit_sdk.RegisterAction(extecs.NewEcsServiceEventLogAction(serviceDiscoveryPoller))
		action_kit_sdk.RegisterAction(extecs.NewEcsServiceTaskCountCheckAction(serviceDiscoveryPoller))
	}

	if !cfg.DiscoveryDisabledElasticache {
		discovery_kit_sdk.Register(extelasticache.NewElasticacheReplicationGroupDiscovery(ctx))
		action_kit_sdk.RegisterAction(extelasticache.NewElasticacheNodeGroupFailoverAttack())
	}

	if !cfg.DiscoveryDisabledElb {
		discovery_kit_sdk.Register(extelb.NewAlbDiscovery(ctx))
		action_kit_sdk.RegisterAction(extelb.NewAlbStaticResponseAction())
	}

	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))
}

type ExtensionListResponse struct {
	action_kit_api.ActionList
	discovery_kit_api.DiscoveryList
}

func getExtensionList() ExtensionListResponse {
	return ExtensionListResponse{
		ActionList:    action_kit_sdk.GetActionList(),
		DiscoveryList: discovery_kit_sdk.GetDiscoveryList(),
	}
}

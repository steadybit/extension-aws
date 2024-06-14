// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package main

import (
	"context"
	_ "github.com/KimMachineGun/automemlimit" // By default, it sets `GOMEMLIMIT` to 90% of cgroup's memory limit.
	"github.com/rs/zerolog"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/extaz"
	"github.com/steadybit/extension-aws/extec2"
	"github.com/steadybit/extension-aws/extecs"
	"github.com/steadybit/extension-aws/extfis"
	"github.com/steadybit/extension-aws/extlambda"
	"github.com/steadybit/extension-aws/extrds"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthealth"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extruntime"
	_ "go.uber.org/automaxprocs" // Importing automaxprocs automatically adjusts GOMAXPROCS.
	_ "net/http/pprof"           //allow pprof
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

	config.ParseConfiguration()
	utils.InitializeAwsAccountAccess(config.Config)
	utils.InitializeAwsZones()

	ctx, cancel := SignalCanceledContext()
	defer cancel()

	registerHandlers(ctx)

	action_kit_sdk.InstallSignalHandler()
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
		discovery_kit_sdk.Register(extaz.NewAzDiscovery(ctx))
		action_kit_sdk.RegisterAction(extaz.NewAzBlackholeAction())
	}

	if !cfg.DiscoveryDisabledEc2 {
		discovery_kit_sdk.Register(extec2.NewEc2InstanceDiscovery(ctx))
		action_kit_sdk.RegisterAction(extec2.NewEc2InstanceStateAction())
	}

	if !cfg.DiscoveryDisabledFis {
		discovery_kit_sdk.Register(extfis.NewFisTemplateDiscovery(ctx))
		action_kit_sdk.RegisterAction(extfis.NewFisExperimentAction())
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
		discovery_kit_sdk.Register(extecs.NewEcsTaskDiscovery(ctx))
		action_kit_sdk.RegisterAction(extecs.NewEcsTaskStopAction())
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

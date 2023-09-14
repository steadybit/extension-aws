package utils

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"os"
	"time"
)

func StartDiscoveryTask(
	stopCh chan os.Signal,
	discovery string,
	interval int,
	supplier func(account *AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error),
	updateResults func(updatedTargets []discovery_kit_api.Target, err *extension_kit.ExtensionError)) {
	//init empty results
	updateResults([]discovery_kit_api.Target{}, nil)
	//start first discovery immediately
	discover(supplier, discovery, updateResults)
	//start loop
	go func() {
		log.Info().Msgf("Starting %s discovery", discovery)
		for {
			select {
			case <-stopCh:
				log.Info().Msgf("Stopping %s discovery", discovery)
				return
			case <-time.After(time.Duration(interval) * time.Second):
				discover(supplier, discovery, updateResults)
			}
		}
	}()
}

func discover(supplier func(account *AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error), discovery string, updateResults func(updatedTargets []discovery_kit_api.Target, err *extension_kit.ExtensionError)) {
	start := time.Now()
	updatedTargets, err := ForEveryAccount(Accounts, supplier, context.Background(), discovery)
	if err != nil {
		updateResults([]discovery_kit_api.Target{}, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to collect %s information", discovery), err)))
	} else {
		updateResults(*updatedTargets, nil)
		elapsed := time.Since(start)
		log.Debug().Msgf("Updated %d %s targets in %s", len(*updatedTargets), discovery, elapsed)
	}
}

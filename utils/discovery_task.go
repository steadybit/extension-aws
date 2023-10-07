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
	discoveryName string,
	interval time.Duration,
	supplier func(account *AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error),
	updateResults func(updatedTargets []discovery_kit_api.Target, err *extension_kit.ExtensionError)) {
	//init empty results
	updateResults([]discovery_kit_api.Target{}, nil)
	//start first discovery immediately
	discover(supplier, discoveryName, updateResults)
	//start loop
	go func() {
		log.Info().Msgf("Starting %s discovery", discoveryName)
		for {
			select {
			case <-stopCh:
				log.Info().Msgf("Stopping %s discovery", discoveryName)
				return
			case <-time.After(interval):
				discover(supplier, discoveryName, updateResults)
			}
		}
	}()
}

func discover(supplier func(account *AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error), discoveryName string, updateResults func(updatedTargets []discovery_kit_api.Target, err *extension_kit.ExtensionError)) {
	start := time.Now()
	updatedTargets, err := ForEveryAccount(Accounts, supplier, context.Background(), discoveryName)
	if err != nil {
		updateResults([]discovery_kit_api.Target{}, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to collect %s information", discoveryName), err)))
	} else {
		updateResults(*updatedTargets, nil)
		elapsed := time.Since(start)
		log.Debug().Msgf("Updated %d %s targets in %s", len(*updatedTargets), discoveryName, elapsed)
	}
}

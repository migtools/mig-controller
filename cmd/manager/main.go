/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/konveyor/mig-controller/pkg/apis"
	"github.com/konveyor/mig-controller/pkg/compat/conversion"
	"github.com/konveyor/mig-controller/pkg/controller"
	"github.com/konveyor/mig-controller/pkg/imagescheme"
	"github.com/konveyor/mig-controller/pkg/webhook"
	"github.com/konveyor/mig-controller/pkg/zapmod"
	appsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	logf.SetLogger(zapmod.ZapLogger(false))
	log := logf.Log.WithName("entrypoint")

	// Start prometheus metrics HTTP handler
	log.Info("setting up prometheus endpoint :2112/metrics")
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)

	// Get a config to talk to the apiserver
	log.Info("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up manager")
	// watchMgr is namespace scoped. it is used by the controllers to watch the resources owned by
	// the controllers present in the migration namespace
	watchMgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0", Namespace: "openshift-migration"})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	// unscopedMgr is not namespace scoped. it is used by controllers to list resources
	// outside of the migration namespace
	unscopedMgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	if err != nil {
		log.Error(err, "unable to set up unscoped manager")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	log.Info("setting up scheme")
	if err := apis.AddToScheme(watchMgr.GetScheme()); err != nil {
		log.Error(err, "unable to add K8s APIs to scheme")
		os.Exit(1)
	}
	if err := velerov1.AddToScheme(watchMgr.GetScheme()); err != nil {
		log.Error(err, "unable to add Velero APIs to scheme")
		os.Exit(1)
	}
	if err := imagescheme.AddToScheme(watchMgr.GetScheme()); err != nil {
		log.Error(err, "unable to add OpenShift image APIs to scheme")
		os.Exit(1)
	}
	if err := appsv1.AddToScheme(watchMgr.GetScheme()); err != nil {
		log.Error(err, "unable to add OpenShift apps APIs to scheme")
		os.Exit(1)
	}
	if err := routev1.AddToScheme(watchMgr.GetScheme()); err != nil {
		log.Error(err, "unable to add OpenShift route APIs to scheme")
		os.Exit(1)
	}
	if err := conversion.RegisterConversions(watchMgr.GetScheme()); err != nil {
		log.Error(err, "unable to register nessesary conversions")
		os.Exit(1)
	}

	// Setup all Controllers
	log.Info("Setting up controller")
	if err := controller.AddToManager(watchMgr, unscopedMgr); err != nil {
		log.Error(err, "unable to register controllers to the manager")
		os.Exit(1)
	}

	log.Info("setting up webhooks")
	if err := webhook.AddToManager(watchMgr); err != nil {
		log.Error(err, "unable to register webhooks to the manager")
		os.Exit(1)
	}

	// Start the Cmd
	log.Info("Starting the Cmd.")
	ctx, stopFunc := context.WithCancel(context.TODO())
	stopChan := make(chan error)
	go func() {
		stopChan <- unscopedMgr.Start(ctx)
	}()
	go func() {
		stopChan <- watchMgr.Start(ctx)
	}()

	for {
		err := <-stopChan
		if err != nil {
			log.Error(err, "unable to run the manager")
			stopFunc()
			os.Exit(1)
		}
	}
}

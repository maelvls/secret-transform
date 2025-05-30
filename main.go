package main

import (
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)



func init() {
	log.SetLogger(zap.New())
}

func main() {
	logger := log.Log.WithName("secret-transform")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	if err := setupReconciler(mgr); err != nil {
		logger.Error(err, "problem setting up controller")
		os.Exit(1)
	}

	logger.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

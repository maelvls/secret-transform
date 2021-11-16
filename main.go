/*
Copyright 2018 The Kubernetes Authors.
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
	"bytes"
	"context"
	"encoding/pem"
	"os"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	secretAnnotKey = "cert-manager-secret-transform"
	tlsDERDataKey  = "tls.der"
)

func init() {
	log.SetLogger(zap.New())
}

func main() {
	log := log.Log.WithName("secret-transform")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	rec := mgr.GetEventRecorderFor("secret-transformer")
	c, err := controller.New("secret-transformer", mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, r reconcile.Request) (reconcile.Result, error) {
			secret := corev1.Secret{}
			err := mgr.GetClient().Get(ctx, r.NamespacedName, &secret)
			switch {
			case k8serrors.IsNotFound(err):
				return reconcile.Result{}, nil
			case err != nil:
				return reconcile.Result{}, err
			}

			tlsKey, exists := secret.Data["tls.key"]
			if !exists {
				rec.Eventf(&secret, corev1.EventTypeWarning, "MissingTLSKey", "Secret %s does not contain a 'tls.key' data key", secret.Name)
				return reconcile.Result{}, nil
			}

			block, _ := pem.Decode(tlsKey)
			tlsDERNew := block.Bytes
			if tlsDEROld, exists := secret.Data[tlsDERDataKey]; exists && bytes.Compare(tlsDEROld, tlsDERNew) == 0 {
				return reconcile.Result{}, nil
			}
			log.Info("Updating secret with key", secret.Name, tlsDERDataKey)
			secret.Data[tlsDERDataKey] = tlsDERNew
			err = mgr.GetClient().Update(ctx, &secret)
			if err != nil {
				return reconcile.Result{}, err
			}

			rec.Eventf(&secret, corev1.EventTypeNormal, "Transformed", "Added key %s to the Secret data", tlsDERDataKey)
			return reconcile.Result{}, nil
		}),
	})
	if err != nil {
		log.Error(err, "unable to set up individual controller")
		os.Exit(1)
	}

	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		if o.GetAnnotations() == nil {
			return nil
		}

		var value string
		var ok bool
		if value, ok = o.GetAnnotations()[secretAnnotKey]; !ok {
			return nil
		}

		switch value {
		case tlsDERDataKey:
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: o.GetNamespace(), Name: o.GetName()}}}
		default:
			rec.Eventf(o, corev1.EventTypeWarning, "InvalidSecretTransform", "Value %s is invalid for annotation %s", value, secretAnnotKey)
			return nil
		}
	})); err != nil {
		log.Error(err, "unable to watch Secrets")
		os.Exit(1)
	}

	log.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

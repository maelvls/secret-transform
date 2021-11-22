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
	"strings"

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
	secretAnnotKeyIn  = "secret-transform-in"
	secretAnnotKeyOut = "secret-transform-out"
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

			keyOut, hasOut := secret.GetAnnotations()[secretAnnotKeyOut]
			inputsRaw, hasIn := secret.GetAnnotations()[secretAnnotKeyIn]

			if !hasIn && !hasOut {
				// Both "secret-transform-in" and "secret-transform-out" are not
				// set, so we don't need to do anything.
				return reconcile.Result{}, nil
			}

			if hasOut != hasIn {
				rec.Eventf(&secret, corev1.EventTypeWarning, "MissingAnnotation", "Both annotations %s and %s must be set", secretAnnotKeyIn, secretAnnotKeyOut)
				return reconcile.Result{}, nil
			}

			if keyOut != "ca.crt" && keyOut != "tls.crt" && keyOut != "tls.key" {
				rec.Eventf(&secret, corev1.EventTypeWarning, "InvalidOut", "Output cannot be 'tls.key', 'tls.crt', or 'ca.crt'")
				return reconcile.Result{}, nil
			}

			inputs := strings.Split(inputsRaw, ",")

			if len(inputsRaw) == 0 {
				rec.Eventf(&secret, corev1.EventTypeWarning, "InvalidIn", "The value for the annotation %s cannot be empty", secretAnnotKeyIn)
				return reconcile.Result{}, nil
			}

			var newOutValue []byte
			switch {
			case strings.HasSuffix(keyOut, "der"):
				if len(inputs) > 1 {
					rec.Eventf(&secret, corev1.EventTypeWarning, "MultipleDER", "When using the DER format, you can only pass a single input with the %s annotation", secretAnnotKeyIn)
					return reconcile.Result{}, nil
				}

				keyIn := inputs[0]
				bytes, ok := secret.Data[keyIn]
				if !ok {
					rec.Eventf(&secret, corev1.EventTypeWarning, "MissingIn", "The input secret key %s was not found in the secret", keyIn)
					return reconcile.Result{}, nil
				}

				pemBlock, err := pem.Decode(bytes)
				if err != nil {
					rec.Eventf(&secret, corev1.EventTypeWarning, "InvalidIn", "Input secret key '%s' cannot be decoded", keyIn)
					return reconcile.Result{}, nil
				}

				newOutValue = pemBlock.Bytes
			case strings.HasSuffix(keyOut, "pem"):
				for _, input := range inputs {
					if input != "ca.crt" && input != "tls.crt" && input != "tls.key" {
						rec.Eventf(&secret, corev1.EventTypeWarning, "InvalidIn", "The input secret keys can only be one of 'tls.key', 'tls.crt', or 'ca.crt' or a comma-separated list of those")
						continue
					}

					pemBytes, ok := secret.Data[input]
					if !ok {
						rec.Eventf(&secret, corev1.EventTypeWarning, "MissingIn", "The input secret key %s was not found in the secret", input)
						return reconcile.Result{}, nil
					}

					newOutValue = append(newOutValue, pemBytes...)
				}
			}

			oldOutValue, exists := secret.Data[keyOut]
			if exists && bytes.Compare(oldOutValue, newOutValue) == 0 {
				return reconcile.Result{}, nil
			}

			log.Info("Updating secret with key", secret.Name, keyOut)
			secret.Data[keyOut] = newOutValue
			err = mgr.GetClient().Update(ctx, &secret)
			if err != nil {
				return reconcile.Result{}, err
			}

			rec.Eventf(&secret, corev1.EventTypeNormal, "Transformed", "Added or updated key %s to the Secret data", keyOut)
			return reconcile.Result{}, nil
		}),
	})
	if err != nil {
		log.Error(err, "unable to set up individual controller")
		os.Exit(1)
	}

	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: o.GetNamespace(), Name: o.GetName()}}}
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

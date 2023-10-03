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
	"fmt"
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/record"
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
	// To combine `tls.crt` and `tls.key` into a single PEM, use the following
	// annotation on a Secret:
	//
	//  cert-manager.io/secret-transform: "tls.pem"
	//
	// The contents of `tls.key` and `tls.crt` will be merged into a new key
	// `tls.pem`. This key name isn't configurable.
	secretAnnotKey = "cert-manager.io/secret-transform" // Values: "tls.pem"
	tlsPEMDataKey  = "tls.pem"

	// To copy an existing key to a new key, use one of the annotations below on
	// a Secret:
	//
	//  cert-manager.io/secret-copy-ca.crt: "ca"
	//  cert-manager.io/secret-copy-tls.crt: "cert"
	//  cert-manager.io/secret-copy-tls.key: "key"
	//
	// In the first example, the contents of the `ca.crt` key will be copied to
	// a new key `ca`, even when the Secret's `ca.crt` is updated. Each of the
	// annotation values are configurable.
	secretSyncCACRTAnnotKey  = "cert-manager.io/secret-copy-ca.crt"
	secretSyncTLSCrtAnnotKey = "cert-manager.io/secret-copy-tls.crt"
	secretSyncTLSKeyAnnotKey = "cert-manager.io/secret-copy-tls.key"
)

func init() {
	log.SetLogger(zap.New())
}

func main() {
	log := log.Log.WithName("entrypoint")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	rec := mgr.GetEventRecorderFor("secret-transform")
	c, err := controller.New("secret-transform", mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, r reconcile.Request) (reconcile.Result, error) {
			secret := corev1.Secret{}
			err := mgr.GetClient().Get(ctx, r.NamespacedName, &secret)
			switch {
			case k8serrors.IsNotFound(err):
				return reconcile.Result{}, nil
			case err != nil:
				return reconcile.Result{}, err
			}

			secretBefore := secret.DeepCopy()

			transformTo := secret.GetAnnotations()[secretAnnotKey]
			if transformTo != "" {
				mergeCombinedPEM(ctx, rec, secret)
			}

			copyCACrtKey := secret.GetAnnotations()[secretSyncCACRTAnnotKey]
			if copyCACrtKey != "" {
				copyKey(ctx, rec, secret, "ca.crt", copyCACrtKey)
			}

			copyTLSCrtKey := secret.GetAnnotations()[secretSyncTLSCrtAnnotKey]
			if copyTLSCrtKey != "" {
				copyKey(ctx, rec, secret, "tls.crt", copyTLSCrtKey)
			}

			copyTLSKeyKey := secret.GetAnnotations()[secretSyncTLSKeyAnnotKey]
			if copyTLSKeyKey != "" {
				copyKey(ctx, rec, secret, "tls.key", copyTLSKeyKey)
			}

			if reflect.DeepEqual(secret.Data, secretBefore.Data) {
				return reconcile.Result{}, nil
			}

			err = mgr.GetClient().Update(ctx, &secret)
			if err != nil {
				return reconcile.Result{}, err
			}

			if transformTo != "" {
				rec.Eventf(&secret, corev1.EventTypeNormal, "Transformed", "Added key %s to the Secret data", tlsPEMDataKey)
			}
			if copyCACrtKey != "" {
				rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied key %s to the key %s in the Secret data", copyCACrtKey, "ca.crt")
			}
			if copyTLSCrtKey != "" {
				rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied key %s to the key %s in the Secret data", copyTLSCrtKey, "tls.crt")
			}
			if copyTLSKeyKey != "" {
				rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied key %s to the key %s in the Secret data", copyTLSKeyKey, "tls.key")
			}

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
		if o.GetAnnotations()[secretAnnotKey] == "" &&
			o.GetAnnotations()[secretSyncCACRTAnnotKey] == "" &&
			o.GetAnnotations()[secretSyncTLSCrtAnnotKey] == "" &&
			o.GetAnnotations()[secretSyncTLSKeyAnnotKey] == "" {
			return nil
		}

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

func mergeCombinedPEM(ctx context.Context, rec record.EventRecorder, secret corev1.Secret) {
	transformTo := secret.GetAnnotations()[secretAnnotKey]

	if transformTo != tlsPEMDataKey {
		rec.Eventf(&secret, corev1.EventTypeWarning, "InvalidSecretTransform", "Value %s is invalid for annotation %s", transformTo, tlsPEMDataKey)
		return
	}

	tlsKey, exists := secret.Data["tls.key"]
	if !exists {
		rec.Eventf(&secret, corev1.EventTypeWarning, "MissingTLSKey", "Secret %s does not contain a 'tls.key' data key", secret.Name)
		return
	}

	tlsCrt, exists := secret.Data["tls.crt"]
	if !exists {
		rec.Eventf(&secret, corev1.EventTypeWarning, "MissingTLSCrt", "Secret %s does not contain a 'tls.crt' data key", secret.Name)
		return
	}

	tlsPEMNew := []byte(fmt.Sprintf("%s%s", tlsKey, tlsCrt))

	if tlsPEMOld, exists := secret.Data[tlsPEMDataKey]; exists && bytes.Compare(tlsPEMOld, tlsPEMNew) == 0 {
		return
	}

	secret.Data[tlsPEMDataKey] = tlsPEMNew
}

// Only return an error when the error "retriable" i.e., retriying may fix the
// issue (e.g. a network error or optimistic locking).
func copyKey(ctx context.Context, rec record.EventRecorder, secret corev1.Secret, keyFrom string, keyTo string) (bool, reconcile.Result, error) {
	caCrtOriginal, exists := secret.Data[keyFrom]
	if !exists {
		rec.Eventf(&secret, corev1.EventTypeWarning, "MissingCACrtKey", "Secret %s does not contain a '%s' data key", secret.Name, keyFrom)
		return true, reconcile.Result{}, nil
	}

	caCrtCopy := secret.Data[keyTo]
	if bytes.Compare(caCrtOriginal, caCrtCopy) == 0 {
		return true, reconcile.Result{}, nil
	}

	secret.Data[keyTo] = secret.Data[keyFrom]

	return false, reconcile.Result{}, nil
}

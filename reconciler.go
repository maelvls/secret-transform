package main

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
	//  cert-manager.io/secret-copy-keystore.jks: "keystore"
	//  cert-manager.io/secret-copy-truststore.jks: "truststore"
	//  cert-manager.io/secret-copy-keystore.p12: "keystore"
	//  cert-manager.io/secret-copy-truststore.p12: "truststore"
	//
	// In the first example, the contents of the `ca.crt` key will be copied to
	// a new key `ca`, even when the Secret's `ca.crt` is updated. Each of the
	// annotation values are configurable.
	secretSyncCACRTAnnotKey         = "cert-manager.io/secret-copy-ca.crt"
	secretSyncTLSCrtAnnotKey        = "cert-manager.io/secret-copy-tls.crt"
	secretSyncTLSKeyAnnotKey        = "cert-manager.io/secret-copy-tls.key"
	secretSyncKeystoreJKSAnnotKey   = "cert-manager.io/secret-copy-keystore.jks"
	secretSyncTruststoreJKSAnnotKey = "cert-manager.io/secret-copy-truststore.jks"
	secretSyncKeystoreP12AnnotKey   = "cert-manager.io/secret-copy-keystore.p12"
	secretSyncTruststoreP12AnnotKey = "cert-manager.io/secret-copy-truststore.p12"
)

// Handles the "cert-manager.io/secret-transform" annotation. Mutates the
// Secret's data.
func mergeCombinedPEM(rec record.EventRecorder, secret *corev1.Secret) {
	transformTo := getAnnotValue(secret.GetAnnotations(), secretAnnotKey)
	if transformTo != tlsPEMDataKey {
		rec.Eventf(secret, corev1.EventTypeWarning, "InvalidSecretTransform", "Value %s is invalid for annotation %s", transformTo, secretAnnotKey)
		return
	}

	tlsKey, exists := secret.Data["tls.key"]
	if !exists {
		rec.Eventf(secret, corev1.EventTypeWarning, "MissingTLSKey", "Secret %s does not contain a 'tls.key' data key", secret.Name)
		return
	}

	tlsCrt, exists := secret.Data["tls.crt"]
	if !exists {
		rec.Eventf(secret, corev1.EventTypeWarning, "MissingTLSCrt", "Secret %s does not contain a 'tls.crt' data key", secret.Name)
		return
	}

	tlsPEMNew := []byte(fmt.Sprintf("%s%s", tlsKey, tlsCrt))

	if tlsPEMOld, exists := secret.Data[tlsPEMDataKey]; exists && bytes.Compare(tlsPEMOld, tlsPEMNew) == 0 {
		return
	}

	secret.Data[tlsPEMDataKey] = tlsPEMNew
}

// The Secret is mutated. Returns true if the Secret was mutated, false if
// nothing was changed. Returns an error if the key does not exist.
func copyKey(secret corev1.Secret, keyFrom string, keyTo string) error {
	caCrtOriginal, exists := secret.Data[keyFrom]
	if !exists {
		return fmt.Errorf("the key %q does not exist", keyFrom)
	}

	caCrtCopy := secret.Data[keyTo]
	if bytes.Compare(caCrtOriginal, caCrtCopy) == 0 {
		return nil
	}

	secret.Data[keyTo] = secret.Data[keyFrom]
	return nil
}

func getAnnotValue(annots map[string]string, annotationKey string) (value string) {
	if annots == nil {
		return ""
	}

	value, found := annots[annotationKey]
	if found && value != "" {
		return value
	}

	return ""
}

func Reconciler(client client.Client, rec record.EventRecorder) reconcile.Func {
	log := log.Log.WithName("secret-transform")
	return func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		log := log.WithValues("secret_name", req.NamespacedName.Name, "namespace", req.NamespacedName.Namespace)
		secret := corev1.Secret{}
		err := client.Get(ctx, req.NamespacedName, &secret)
		switch {
		case k8serrors.IsNotFound(err):
			return reconcile.Result{}, nil
		case err != nil:
			return reconcile.Result{}, err
		}

		secretBefore := secret.DeepCopy()

		transformTo := getAnnotValue(secret.GetAnnotations(), secretAnnotKey)
		if transformTo != "" {
			mergeCombinedPEM(rec, &secret)
		}

		copyCACrtKey := getAnnotValue(secret.GetAnnotations(), secretSyncCACRTAnnotKey)
		if copyCACrtKey != "" {
			err := copyKey(secret, "ca.crt", copyCACrtKey)
			if err != nil {
				log.WithValues(err, "while copying")
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", err.Error())
				return reconcile.Result{}, nil
			}
		}

		copyTLSCrtKey := getAnnotValue(secret.GetAnnotations(), secretSyncTLSCrtAnnotKey)
		if copyTLSCrtKey != "" {
			err := copyKey(secret, "tls.crt", copyTLSCrtKey)
			if err != nil {
				log.WithValues(err, "while copying")
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", err.Error())
				return reconcile.Result{}, nil
			}
		}

		copyTLSKeyKey := getAnnotValue(secret.GetAnnotations(), secretSyncTLSKeyAnnotKey)
		if copyTLSKeyKey != "" {
			err := copyKey(secret, "tls.key", copyTLSKeyKey)
			if err != nil {
				log.WithValues(err, "while copying")
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", err.Error())
				return reconcile.Result{}, nil
			}
		}

		copyKeystoreJKSKey := getAnnotValue(secret.GetAnnotations(), secretSyncKeystoreJKSAnnotKey)
		if copyKeystoreJKSKey != "" {
			err := copyKey(secret, "keystore.jks", copyKeystoreJKSKey)
			if err != nil {
				log.WithValues(err, "while copying")
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", err.Error())
				return reconcile.Result{}, nil
			}
		}

		copyTruststoreJKSKey := getAnnotValue(secret.GetAnnotations(), secretSyncTruststoreJKSAnnotKey)
		if copyTruststoreJKSKey != "" {
			err := copyKey(secret, "truststore.jks", copyTruststoreJKSKey)
			if err != nil {
				log.WithValues(err, "while copying")
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", err.Error())
				return reconcile.Result{}, nil
			}
		}

		copyKeystoreP12Key := getAnnotValue(secret.GetAnnotations(), secretSyncKeystoreP12AnnotKey)
		if copyKeystoreP12Key != "" {
			err := copyKey(secret, "keystore.p12", copyKeystoreP12Key)
			if err != nil {
				log.WithValues(err, "while copying")
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", err.Error())
				return reconcile.Result{}, nil
			}
		}

		copyTruststoreP12Key := getAnnotValue(secret.GetAnnotations(), secretSyncTruststoreP12AnnotKey)
		if copyTruststoreP12Key != "" {
			err := copyKey(secret, "truststore.p12", copyTruststoreP12Key)
			if err != nil {
				log.WithValues(err, "while copying")
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", err.Error())
				return reconcile.Result{}, nil
			}
		}

		if reflect.DeepEqual(secret.Data, secretBefore.Data) {
			return reconcile.Result{}, nil
		}

		err = client.Update(ctx, &secret)
		if err != nil {
			return reconcile.Result{}, err
		}

		if transformTo != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "Transformed", "Added key %s", tlsPEMDataKey)
		}
		if copyCACrtKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of %q into key %q", "ca.crt", copyCACrtKey)
		}
		if copyTLSCrtKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of %q into key %q", "tls.crt", copyTLSCrtKey)
		}
		if copyTLSKeyKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of %q into key %q", "tls.key", copyTLSKeyKey)
		}
		if copyKeystoreJKSKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of %q into key %q", "keystore.jks", copyKeystoreJKSKey)
		}
		if copyTruststoreJKSKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of %q into key %q", "truststore.jks", copyTruststoreJKSKey)
		}
		if copyKeystoreP12Key != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of %q into key %q", "keystore.p12", copyKeystoreP12Key)
		}
		if copyTruststoreP12Key != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of %q into key %q", "truststore.p12", copyTruststoreP12Key)
		}

		return reconcile.Result{}, nil
	}
}

// ShouldReconcileSecret returns true if the secret has any of the annotations that we're interested in
func ShouldReconcileSecret(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	return getAnnotValue(annotations, secretAnnotKey) != "" ||
		getAnnotValue(annotations, secretSyncCACRTAnnotKey) != "" ||
		getAnnotValue(annotations, secretSyncTLSCrtAnnotKey) != "" ||
		getAnnotValue(annotations, secretSyncTLSKeyAnnotKey) != "" ||
		getAnnotValue(annotations, secretSyncKeystoreJKSAnnotKey) != "" ||
		getAnnotValue(annotations, secretSyncTruststoreJKSAnnotKey) != "" ||
		getAnnotValue(annotations, secretSyncKeystoreP12AnnotKey) != "" ||
		getAnnotValue(annotations, secretSyncTruststoreP12AnnotKey) != ""
}

// setupReconciler sets up the controller with the Manager. This is extracted as
// a separate function to make it testable.
func setupReconciler(mgr manager.Manager) error {
	rec := mgr.GetEventRecorderFor("secret-transform")
	reconciler := Reconciler(mgr.GetClient(), rec)

	c, err := controller.New("secret-transform", mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return fmt.Errorf("unable to set up individual controller: %w", err)
	}

	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		if !ShouldReconcileSecret(o.GetAnnotations()) {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: o.GetNamespace(), Name: o.GetName()}}}
	})); err != nil {
		return fmt.Errorf("unable to watch Secrets: %w", err)
	}

	return nil
}

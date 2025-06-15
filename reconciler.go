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
	//  secret-transform/secret-transform: "tls.pem"
	//
	// The contents of `tls.key` and `tls.crt` will be merged into a new key
	// `tls.pem`. This key name isn't configurable.
	secretAnnotKey    = "secret-transform/secret-transform" // Values: "tls.pem"
	oldSecretAnnotKey = "cert-manager.io/secret-transform"  // Values: "tls.pem"

	tlsPEMDataKey = "tls.pem"

	// To copy an existing key to a new key, use one of the annotations below on
	// a Secret:
	//
	//  secret-transform/secret-copy-ca.crt: "ca"
	//  secret-transform/secret-copy-tls.crt: "cert"
	//  secret-transform/secret-copy-tls.key: "key"
	//  secret-transform/secret-copy-keystore.jks: "keystore"
	//  secret-transform/secret-copy-truststore.jks: "truststore"
	//  secret-transform/secret-copy-keystore.p12: "keystore"
	//  secret-transform/secret-copy-truststore.p12: "truststore"
	//
	// In the first example, the contents of the `ca.crt` key will be copied to
	// a new key `ca`, even when the Secret's `ca.crt` is updated. Each of the
	// annotation values are configurable.
	secretSyncCACRTAnnotKey         = "secret-transform/secret-copy-ca.crt"
	secretSyncTLSCrtAnnotKey        = "secret-transform/secret-copy-tls.crt"
	secretSyncTLSKeyAnnotKey        = "secret-transform/secret-copy-tls.key"
	secretSyncKeystoreJKSAnnotKey   = "secret-transform/secret-copy-keystore.jks"
	secretSyncTruststoreJKSAnnotKey = "secret-transform/secret-copy-truststore.jks"
	secretSyncKeystoreP12AnnotKey   = "secret-transform/secret-copy-keystore.p12"
	secretSyncTruststoreP12AnnotKey = "secret-transform/secret-copy-truststore.p12"

	// Initially, the project started with annotations starting with
	// cert-manager.io/*, which caused issues. These annotations are kept for
	// backwards compatibility.
	// https://github.com/maelvls/secret-transform/issues/11
	oldSecretSyncCACRTAnnotKey         = "cert-manager.io/secret-copy-ca.crt"
	oldSecretSyncTLSCrtAnnotKey        = "cert-manager.io/secret-copy-tls.crt"
	oldSecretSyncTLSKeyAnnotKey        = "cert-manager.io/secret-copy-tls.key"
	oldSecretSyncKeystoreJKSAnnotKey   = "cert-manager.io/secret-copy-keystore.jks"
	oldSecretSyncTruststoreJKSAnnotKey = "cert-manager.io/secret-copy-truststore.jks"
	oldSecretSyncKeystoreP12AnnotKey   = "cert-manager.io/secret-copy-keystore.p12"
	oldSecretSyncTruststoreP12AnnotKey = "cert-manager.io/secret-copy-truststore.p12"
)

// Handles the "secret-transform/secret-transform" annotation and its legacy
// counterpart "cert-manager.io/secret-transform". Mutates the Secret's data.
func mergeCombinedPEM(rec record.EventRecorder, secret *corev1.Secret) {
	annot, transformTo := getOneOf(secret.GetAnnotations(), secretAnnotKey, oldSecretAnnotKey)
	if transformTo != tlsPEMDataKey {
		rec.Eventf(secret, corev1.EventTypeWarning, "InvalidSecretTransform", "Value '%s' is invalid for annotation '%s'. The only valid value is '%s'", transformTo, annot, tlsPEMDataKey)
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

	if tlsPEMOld, exists := secret.Data[tlsPEMDataKey]; exists && bytes.Equal(tlsPEMOld, tlsPEMNew) {
		return
	}

	secret.Data[tlsPEMDataKey] = tlsPEMNew
}

// The Secret is mutated when the destination differs from the source.
// Returns an error if the source key does not exist.
func copyKey(secret corev1.Secret, keyFrom string, keyTo string) error {
	caCrtOriginal, exists := secret.Data[keyFrom]
	if !exists {
		return fmt.Errorf("the key %q does not exist", keyFrom)
	}

	caCrtCopy := secret.Data[keyTo]
	if bytes.Equal(caCrtOriginal, caCrtCopy) {
		return nil
	}

	secret.Data[keyTo] = secret.Data[keyFrom]
	return nil
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

		annotFound, transformTo := getOneOf(secret.GetAnnotations(), secretAnnotKey, oldSecretAnnotKey)
		if annotFound != "" {
			mergeCombinedPEM(rec, &secret)
		}

		annot, copyCACrtKey := getOneOf(secret.GetAnnotations(), secretSyncCACRTAnnotKey, oldSecretSyncCACRTAnnotKey)
		if copyCACrtKey != "" {
			err := copyKey(secret, "ca.crt", copyCACrtKey)
			if err != nil {
				log.WithValues(err, "while copying", "annot", annot)
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", err.Error())
				return reconcile.Result{}, nil
			}
		}

		annot, copyTLSCrtKey := getOneOf(secret.GetAnnotations(), secretSyncTLSCrtAnnotKey, oldSecretSyncTLSCrtAnnotKey)
		if copyTLSCrtKey != "" {
			err := copyKey(secret, "tls.crt", copyTLSCrtKey)
			if err != nil {
				log.WithValues(err, "while copying", "annot", annot)
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", fmt.Sprintf("annot '%s': %v", annot, err))
				return reconcile.Result{}, nil
			}
		}

		annot, copyTLSKeyKey := getOneOf(secret.GetAnnotations(), secretSyncTLSKeyAnnotKey, oldSecretSyncTLSKeyAnnotKey)
		if copyTLSKeyKey != "" {
			err := copyKey(secret, "tls.key", copyTLSKeyKey)
			if err != nil {
				log.WithValues(err, "while copying", "annot", annot)
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", fmt.Sprintf("annot '%s': %v", annot, err))
				return reconcile.Result{}, nil
			}
		}

		annot, copyKeystoreJKSKey := getOneOf(secret.GetAnnotations(), secretSyncKeystoreJKSAnnotKey, oldSecretSyncKeystoreJKSAnnotKey)
		if copyKeystoreJKSKey != "" {
			err := copyKey(secret, "keystore.jks", copyKeystoreJKSKey)
			if err != nil {
				log.WithValues(err, "while copying", "annot", annot)
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", fmt.Sprintf("annot '%s': %v", annot, err))
				return reconcile.Result{}, nil
			}
		}

		annot, copyTruststoreJKSKey := getOneOf(secret.GetAnnotations(), secretSyncTruststoreJKSAnnotKey, oldSecretSyncTruststoreJKSAnnotKey)
		if copyTruststoreJKSKey != "" {
			err := copyKey(secret, "truststore.jks", copyTruststoreJKSKey)
			if err != nil {
				log.WithValues(err, "while copying", "annot", annot)
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", fmt.Sprintf("annot '%s': %v", annot, err))
				return reconcile.Result{}, nil
			}
		}

		annot, copyKeystoreP12Key := getOneOf(secret.GetAnnotations(), secretSyncKeystoreP12AnnotKey, oldSecretSyncKeystoreP12AnnotKey)
		if copyKeystoreP12Key != "" {
			err := copyKey(secret, "keystore.p12", copyKeystoreP12Key)
			if err != nil {
				log.WithValues(err, "while copying", "annot", annot)
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", fmt.Sprintf("annot '%s': %v", annot, err))
				return reconcile.Result{}, nil
			}
		}

		annot, copyTruststoreP12Key := getOneOf(secret.GetAnnotations(), secretSyncTruststoreP12AnnotKey, oldSecretSyncTruststoreP12AnnotKey)
		if copyTruststoreP12Key != "" {
			err := copyKey(secret, "truststore.p12", copyTruststoreP12Key)
			if err != nil {
				log.WithValues(err, "while copying", "annot", annot)
				rec.Eventf(&secret, corev1.EventTypeWarning, "FailedCopying", fmt.Sprintf("annot '%s': %v", annot, err))
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
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of '%s' into key '%s'", "ca.crt", copyCACrtKey)
		}
		if copyTLSCrtKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of '%s' into key '%s'", "tls.crt", copyTLSCrtKey)
		}
		if copyTLSKeyKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of '%s' into key '%s'", "tls.key", copyTLSKeyKey)
		}
		if copyKeystoreJKSKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of '%s' into key '%s'", "keystore.jks", copyKeystoreJKSKey)
		}
		if copyTruststoreJKSKey != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of '%s' into key '%s'", "truststore.jks", copyTruststoreJKSKey)
		}
		if copyKeystoreP12Key != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of '%s' into key '%s'", "keystore.p12", copyKeystoreP12Key)
		}
		if copyTruststoreP12Key != "" {
			rec.Eventf(&secret, corev1.EventTypeNormal, "CopiedKey", "Copied the contents of '%s' into key '%s'", "truststore.p12", copyTruststoreP12Key)
		}

		return reconcile.Result{}, nil
	}
}

// ShouldReconcileSecret returns true if the secret has any of the annotations
// that we're interested in.
func ShouldReconcileSecret(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	if annot, _ := getOneOf(annotations, secretAnnotKey, oldSecretAnnotKey); annot != "" {
		return true
	}
	if annot, _ := getOneOf(annotations, secretSyncCACRTAnnotKey, oldSecretSyncCACRTAnnotKey); annot != "" {
		return true
	}
	if annot, _ := getOneOf(annotations, secretSyncTLSCrtAnnotKey, oldSecretSyncTLSCrtAnnotKey); annot != "" {
		return true
	}
	if annot, _ := getOneOf(annotations, secretSyncTLSKeyAnnotKey, oldSecretSyncTLSKeyAnnotKey); annot != "" {
		return true
	}
	if annot, _ := getOneOf(annotations, secretSyncKeystoreJKSAnnotKey, oldSecretSyncKeystoreJKSAnnotKey); annot != "" {
		return true
	}
	if annot, _ := getOneOf(annotations, secretSyncTruststoreJKSAnnotKey, oldSecretSyncTruststoreJKSAnnotKey); annot != "" {
		return true
	}
	if annot, _ := getOneOf(annotations, secretSyncKeystoreP12AnnotKey, oldSecretSyncKeystoreP12AnnotKey); annot != "" {
		return true
	}
	if annot, _ := getOneOf(annotations, secretSyncTruststoreP12AnnotKey, oldSecretSyncTruststoreP12AnnotKey); annot != "" {
		return true
	}

	return false
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

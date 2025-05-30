package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconciler(t *testing.T) {
	t.Run("the secret-transform annot combines tls.key and tls.crt", run_TestReconciler(case_TestReconciler{
		given: secret(
			map[string]string{"cert-manager.io/secret-transform": "tls.pem"},
			map[string][]byte{"tls.key": []byte("fakeKey"), "tls.crt": []byte("fakeCrt")},
		),
		expectKeys: []string{"tls.pem"},
		expectValues: map[string]string{
			"tls.pem": "fakeKeyfakeCrt",
		},
	}))

	t.Run("the secret-copy-ca.crt annot copies ca.crt", run_TestReconciler(case_TestReconciler{
		given: secret(
			map[string]string{"cert-manager.io/secret-copy-ca.crt": "ca"},
			map[string][]byte{"ca.crt": []byte("fakeCACrt")},
		),
		expectKeys: []string{"ca"},
		expectValues: map[string]string{
			"ca": "fakeCACrt",
		},
	}))

	t.Run("the secret-copy-tls.crt annot copies tls.crt", run_TestReconciler(case_TestReconciler{
		given: secret(
			map[string]string{"cert-manager.io/secret-copy-tls.crt": "cert"},
			map[string][]byte{"tls.crt": []byte("fakeTLSCrt")},
		),
		expectKeys:   []string{"cert"},
		expectValues: map[string]string{"cert": "fakeTLSCrt"},
	}))

	t.Run("the secret-copy-keystore.jks annot copies keystore.jks", run_TestReconciler(case_TestReconciler{
		given: secret(
			map[string]string{"cert-manager.io/secret-copy-keystore.jks": "keystore"},
			map[string][]byte{"keystore.jks": []byte("fakeKeystoreJKS")},
		),
		expectKeys:   []string{"keystore"},
		expectValues: map[string]string{"keystore": "fakeKeystoreJKS"},
	}))

	t.Run("the secret-copy-tls.key annot copies tls.key", run_TestReconciler(case_TestReconciler{
		given: secret(
			map[string]string{"cert-manager.io/secret-copy-tls.key": "key"},
			map[string][]byte{"tls.key": []byte("fakeTLSKey")},
		),
		expectKeys: []string{"key"},
		expectValues: map[string]string{
			"key": "fakeTLSKey",
		},
	}))
	t.Run("the secret-copy-truststore.jks annot copies truststore.jks", run_TestReconciler(case_TestReconciler{
		given: secret(
			map[string]string{"cert-manager.io/secret-copy-truststore.jks": "truststore"},
			map[string][]byte{"truststore.jks": []byte("fakeTruststoreJKS")},
		),
		expectKeys: []string{"truststore"},
		expectValues: map[string]string{
			"truststore": "fakeTruststoreJKS",
		},
	}))
	t.Run("the secret-copy-keystore.p12 annot copies keystore.p12", run_TestReconciler(case_TestReconciler{
		given: secret(
			map[string]string{"cert-manager.io/secret-copy-keystore.p12": "keystore"},
			map[string][]byte{"keystore.p12": []byte("fakeKeystoreP12")},
		),
		expectKeys:   []string{"keystore"},
		expectValues: map[string]string{"keystore": "fakeKeystoreP12"},
	}))
}

func TestShouldReconcileSecret(t *testing.T) {
	t.Run("nil annotations should not reconcile", func(t *testing.T) {
		assert.False(t, ShouldReconcileSecret(nil))
	})

	t.Run("empty annotations should not reconcile", func(t *testing.T) {
		assert.False(t, ShouldReconcileSecret(map[string]string{}))
	})

	t.Run("transform annotation should reconcile", func(t *testing.T) {
		assert.True(t, ShouldReconcileSecret(map[string]string{
			"cert-manager.io/secret-transform": "tls.pem",
		}))
	})

	t.Run("ca.crt copy annotation should reconcile", func(t *testing.T) {
		assert.True(t, ShouldReconcileSecret(map[string]string{
			"cert-manager.io/secret-copy-ca.crt": "ca",
		}))
	})

	t.Run("tls.crt copy annotation should reconcile", func(t *testing.T) {
		assert.True(t, ShouldReconcileSecret(map[string]string{
			"cert-manager.io/secret-copy-tls.crt": "cert",
		}))
	})

	t.Run("tls.key copy annotation should reconcile", func(t *testing.T) {
		assert.True(t, ShouldReconcileSecret(map[string]string{
			"cert-manager.io/secret-copy-tls.key": "key",
		}))
	})

	t.Run("keystore.jks copy annotation should reconcile", func(t *testing.T) {
		assert.True(t, ShouldReconcileSecret(map[string]string{
			"cert-manager.io/secret-copy-keystore.jks": "keystore",
		}))
	})

	t.Run("truststore.jks copy annotation should reconcile", func(t *testing.T) {
		assert.True(t, ShouldReconcileSecret(map[string]string{
			"cert-manager.io/secret-copy-truststore.jks": "truststore",
		}))
	})

	t.Run("keystore.p12 copy annotation should reconcile", func(t *testing.T) {
		assert.True(t, ShouldReconcileSecret(map[string]string{
			"cert-manager.io/secret-copy-keystore.p12": "keystore",
		}))
	})

	t.Run("truststore.p12 copy annotation should reconcile", func(t *testing.T) {
		assert.True(t, ShouldReconcileSecret(map[string]string{
			"cert-manager.io/secret-copy-truststore.p12": "truststore",
		}))
	})

	t.Run("unrelated annotation should not reconcile", func(t *testing.T) {
		assert.False(t, ShouldReconcileSecret(map[string]string{
			"unrelated": "value",
		}))
	})
}

func TestMergeCombinedPEM(t *testing.T) {
	t.Run("secret-transform annot: happy case", func(t *testing.T) {
		given := secret(map[string]string{
			"cert-manager.io/secret-transform": "tls.pem",
		}, map[string][]byte{
			// Both keys are present.
			"tls.key": []byte("fakeKey"),
			"tls.crt": []byte("fakeCrt"),
		})

		rec := record.NewFakeRecorder(10)

		mergeCombinedPEM(rec, given)

		assert.Equal(t, []byte("fakeKeyfakeCrt"), given.Data[tlsPEMDataKey])
		assertNoEvents(t, rec)
	})

	t.Run("secret-transform annot: invalid annotation value", func(t *testing.T) {
		given := secret(map[string]string{
			"cert-manager.io/secret-transform": "invalid-value",
		}, map[string][]byte{
			// Both keys are present, but the annotation value is invalid.
			"tls.key": []byte("fakeKey"),
			"tls.crt": []byte("fakeCrt"),
		})

		rec := record.NewFakeRecorder(10)
		got := given.DeepCopy()
		mergeCombinedPEM(rec, got)
		assert.Equal(t, given, got)
		assertEvents(t, rec, "Warning InvalidSecretTransform Value invalid-value is invalid for annotation tls.pem")
	})

	t.Run("secret-transform annot: show an event when tls.key is missing", func(t *testing.T) {
		given := secret(map[string]string{
			"cert-manager.io/secret-transform": "tls.pem",
		}, map[string][]byte{
			// Missing tls.key
			"tls.crt": []byte("fakeCrt"),
		})
		rec := record.NewFakeRecorder(10)
		got := given.DeepCopy()
		mergeCombinedPEM(rec, got)
		assert.Equal(t, given, got)
		assertEvents(t, rec, "Warning MissingTLSKey Secret test-secret does not contain a 'tls.key' data key")
	})

	t.Run("secret-transform annot: show an event when tls.crt is missing", func(t *testing.T) {
		given := secret(map[string]string{
			"cert-manager.io/secret-transform": "tls.pem",
		}, map[string][]byte{
			// Missing tls.crt.
			"tls.key": []byte("fakeKey"),
		})

		rec := record.NewFakeRecorder(10)
		got := given.DeepCopy()
		mergeCombinedPEM(rec, got)

		assert.Equal(t, given, got)
		assertEvents(t, rec, "Warning MissingTLSCrt Secret test-secret does not contain a 'tls.crt' data key")
	})
}

func TestCopyKey(t *testing.T) {
	t.Run("happy case", func(t *testing.T) {

		secret := corev1.Secret{
			Data: map[string][]byte{
				"sourceKey": []byte("someData"),
			},
		}

		err := copyKey(secret, "sourceKey", "destKey")
		require.NoError(t, err)
		assert.Equal(t, secret.Data["destKey"], []byte("someData"))
		assert.Equal(t, secret.Data["sourceKey"], secret.Data["destKey"])
	})
	t.Run("show an error when missing source key", func(t *testing.T) {
		secret := corev1.Secret{Data: map[string][]byte{}}
		err := copyKey(secret, "nonexistent", "destKey")
		require.EqualError(t, err, "the key \"nonexistent\" does not exist")
	})
}

type case_TestReconciler struct {
	given        *corev1.Secret
	expectKeys   []string
	expectValues map[string]string
}

func run_TestReconciler(test case_TestReconciler) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)

		client := fake.NewClientBuilder().
			WithRuntimeObjects([]runtime.Object{test.given}...).
			WithScheme(scheme).
			Build()

		recorder := record.NewFakeRecorder(10)
		reconciler := Reconciler(client, recorder)

		req := reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		}}
		result, err := reconciler.Reconcile(t.Context(), req)
		assert.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, result)

		updatedSecret := &corev1.Secret{}
		err = client.Get(t.Context(), req.NamespacedName, updatedSecret)
		assert.NoError(t, err)

		for _, key := range test.expectKeys {
			value, exists := updatedSecret.Data[key]
			assert.True(t, exists, "expected key %s to exist in secret data", key)
			expectedValue := test.expectValues[key]
			assert.Equal(t, expectedValue, string(value), "expected value for key %s", key)
		}
	}
}

// assertEvents checks if all expected events were recorded.
func assertEvents(t *testing.T, rec *record.FakeRecorder, expectedEvents ...string) {
	t.Helper()

	close(rec.Events)

	var actual []string
	for e := range rec.Events {
		actual = append(actual, e)
	}

	assert.Equal(t, expectedEvents, actual)
}

func assertNoEvents(t *testing.T, rec *record.FakeRecorder) {
	t.Helper()

	close(rec.Events)

	var actual []string
	for e := range rec.Events {
		actual = append(actual, e)
	}

	assert.Empty(t, actual, "expected no events, but got: %v", actual)
}

func secret(annotations map[string]string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-secret",
			Namespace:   "default",
			Annotations: annotations,
		},
		Data: data,
	}
}

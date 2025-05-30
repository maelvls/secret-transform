package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileSecret(t *testing.T) {
	tests := []struct {
		name           string
		secret         *corev1.Secret
		expectedKeys   []string
		expectedValues map[string]string
	}{
		{
			name: "the secret-transform annot combines tls.key and tls.crt",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						"cert-manager.io/secret-transform": "tls.pem",
					},
				},
				Data: map[string][]byte{
					"tls.key": []byte("fakeKey"),
					"tls.crt": []byte("fakeCrt"),
				},
			},
			expectedKeys: []string{"tls.pem"},
			expectedValues: map[string]string{
				"tls.pem": "fakeKeyfakeCrt",
			},
		},
		{
			name: "the secret-copy-ca.crt annot copies ca.crt",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						"cert-manager.io/secret-copy-ca.crt": "ca",
					},
				},
				Data: map[string][]byte{
					"ca.crt": []byte("fakeCACrt"),
				},
			},
			expectedKeys: []string{"ca"},
			expectedValues: map[string]string{
				"ca": "fakeCACrt",
			},
		},
		{
			name: "the secret-copy-tls.crt annot copies tls.crt",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						"cert-manager.io/secret-copy-tls.crt": "cert",
					},
				},
				Data: map[string][]byte{
					"tls.crt": []byte("fakeTLSCrt"),
				},
			},
			expectedKeys: []string{"cert"},
			expectedValues: map[string]string{
				"cert": "fakeTLSCrt",
			},
		},
		{
			name: "the secret-copy-tls.key annot copies tls.key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						"cert-manager.io/secret-copy-tls.key": "key",
					},
				},
				Data: map[string][]byte{
					"tls.key": []byte("fakeTLSKey"),
				},
			},
			expectedKeys: []string{"key"},
			expectedValues: map[string]string{
				"key": "fakeTLSKey",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			client := fake.NewClientBuilder().
				WithRuntimeObjects([]runtime.Object{tc.secret}...).
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

			for _, key := range tc.expectedKeys {
				value, exists := updatedSecret.Data[key]
				assert.True(t, exists, "expected key %s to exist in secret data", key)
				expectedValue := tc.expectedValues[key]
				assert.Equal(t, expectedValue, string(value), "expected value for key %s", key)
			}
		})
	}
}

func TestShouldReconcileSecret(t *testing.T) {
	testCases := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    false,
		},
		{
			name: "transform annotation",
			annotations: map[string]string{
				"cert-manager.io/secret-transform": "tls.pem",
			},
			expected: true,
		},
		{
			name: "ca.crt copy annotation",
			annotations: map[string]string{
				"cert-manager.io/secret-copy-ca.crt": "ca",
			},
			expected: true,
		},
		{
			name: "tls.crt copy annotation",
			annotations: map[string]string{
				"cert-manager.io/secret-copy-tls.crt": "cert",
			},
			expected: true,
		},
		{
			name: "unrelated annotation",
			annotations: map[string]string{
				"unrelated": "value",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ShouldReconcileSecret(tc.annotations)
			assert.Equal(t, tc.expected, result)
		})
	}
}

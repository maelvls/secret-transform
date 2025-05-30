package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

// TestMergeCombinedPEM verifies that tls.key and tls.crt get combined.
func TestMergeCombinedPEM(t *testing.T) {
	secret := corev1.Secret{
		Data: map[string][]byte{
			"tls.key": []byte("fakeKey"),
			"tls.crt": []byte("fakeCrt"),
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "my-secret",
			Annotations: map[string]string{
				secretAnnotKey: "tls.pem",
			},
		},
	}
	recorder := record.NewFakeRecorder(10)

	mergeCombinedPEM(recorder, secret)
	assert.Equal(t, []byte("fakeKeyfakeCrt"), secret.Data[tlsPEMDataKey])
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

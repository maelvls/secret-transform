# Secret Transformer

The cert-manager issuers store the X.509 keys and certificates in Secret
resources of the form:

```yaml
kind: Secret
type: kubernetes.io/tls
data:
  tls.crt: <certificate>
  tls.key: <key>
```

A common request reported in the cert-manager issue
[#843](https://github.com/jetstack/cert-manager/issues/843) is to create a PEM
bundle containing both the key and certificate for easier use with software
that require a unified PEM bundle, such as

- HAProxy, 
- Hitch,
- OpenDistro for Elasticsearch.

You can run the `secret-transform` controller (right now, it has to be run
out-of-cluster since I did not write any manifest) and if you annotate your
Secret with the following annotation:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  annotations:
    cert-manager.io/secret-transform: tls.pem
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg==
  tls.key: LS0tLS1CRUdJToCi0tLS0tRU5EIF...SBQUklWQVRFIEtFWS0tLS0tCg==
```

then a new data key will be created with the name `tls.pem` and the value
contains the key and certificate concatenated:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  annotations:
    cert-manager.io/secret-transform: tls.pem
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg==
  tls.key: LS0tLS1CRUdJToCi0tLS0tRU5EIF...SBQUklWQVRFIEtFWS0tLS0tCg==
  tls.pem: LS0tLS1CRUdJTiBSUXc0ZHk3NTNl...kQgQ0VSVElGSUNBVEUtLS0tLQo= # ✨
```

The updated Secret looks like this:

```sh
$ kubectl get secret example -ojsonpath='{.data.tls\.pem}' | base64 -d
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAzmuXe0BSZqjh7V94wfTifk/5hKS/V1RjyBa4RVdFBBHNGsUb
u+8UhhRgadS+R5ZrcErpt1YIchNuliqaZbXEW0BpWtRc3NmqDRzh
-----END RSA PRIVATE KEY-----
-----BEGIN CERTIFICATE-----
MIIFXTCCBEWgAwIBAgISBP8i8Bm2p/jl6yxMoLrrJlQkMA0GCSqGSIb3DQEBCwUA
tBpwpdCVsgQqdy69SIU4AYKejVC4nJK9mwAsJi41/W+M
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIFFjCCAv6gAwIBAgIRAJErCErPDBinU/bWLiWnX1owDQYJKoZIhvcNAQELBQAw
MldlTTKB3zhThV1+XWYp6rjd5JW1zbVWEkLNxE7GJThEUG3szgBVGP7pSWTUTsqX
nLRbwHOoq7hHwg==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIFYDCCBEigAwIBAgIQQAF3ITfU6UK47naqPGQKtzANBgkqhkiG9w0BAQsFADA/
Dfvp7OOGAN6dEOM4+qR9sdjoSYKEBpsr6GtPAQw4dy753ec5
-----END CERTIFICATE-----
```

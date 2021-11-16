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
[#843](https://github.com/jetstack/cert-manager/issues/843) is to create a DER
file containing the private key in binary format.

You can run the `secret-transform` controller (see manifests for deployment example) and if you annotate your
Secret with the following annotation:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  annotations:
   cert-manager-secret-transform: tls.der
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg==
  tls.key: LS0tLS1CRUdJToCi0tLS0tRU5EIF...SBQUklWQVRFIEtFWS0tLS0tCg==
```

then a new data key will be created with the name `tls.der` and the value
contains the key and certificate concatenated:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  annotations:
    cert-manager.io/secret-transform: tls.der
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg==
  tls.key: LS0tLS1CRUdJToCi0tLS0tRU5EIF...SBQUklWQVRFIEtFWS0tLS0tCg==
  tls.der: <binary key> # ✨
```

# Secret Transform

Copy and tranform the contents of your Kubernetes Secrets that contain TLS key
material. When a Secret is changed, secret-transform automatically re-copies or
re-transforms the Secret.

- [Installation \& Quick Start](#installation--quick-start)
- [Debugging](#debugging)
- [Renaming the key of a Secret](#renaming-the-key-of-a-secret)
  - [Use-case: Redis Enterprise for Kubernetes](#use-case-redis-enterprise-for-kubernetes)
  - [Use-case: FluxCD](#use-case-fluxcd)
- [Combined PEM bundle](#combined-pem-bundle)
  - [Use-case: MongoDB](#use-case-mongodb)
  - [Use-case: HAProxy Community Edition and HAProxy Enterprise Edition](#use-case-haproxy-community-edition-and-haproxy-enterprise-edition)
  - [Use-case: Hitch](#use-case-hitch)
  - [Use-case: Postgres JBDC driver (lower than 42.2.9)](#use-case-postgres-jbdc-driver-lower-than-4229)
  - [Use-case: Ejabbed](#use-case-ejabbed)
  - [Use-case: Elasticsearch (Elastic's and Open Distro's)](#use-case-elasticsearch-elastics-and-open-distros)
  - [Use-case: Dovecot](#use-case-dovecot)
- [Cut a New Release](#cut-a-new-release)

## Installation & Quick Start

A Helm chart is available as well as container images. To install
secret-transform, run:

```bash
helm upgrade --install secret-transform -n secret-transform --create-namespace \
  oci://ghcr.io/maelvls/charts/secret-transform
```

Then, annotate a Secret:

```bash
kubectl annotate secret cert-1 cert-manager.io/secret-copy-tls.crt=tlsCert
```

You will see that the value for the key `tls.crt` has been copied to the
`tlsCert` key.

## Debugging

If you want to know why one of the Secrets you have annotated hasn't been processed by secret-transform, you can run the following command:

```bash
kubectl events -n default --for secret/cert-1
```

If everything went well, you should see:

```text
LAST SEEN   TYPE     REASON      OBJECT          MESSAGE
0s          Normal   CopiedKey   Secret/cert-1   Copied the contents of "tls.crt" into key "cert"
```

If you would like to check whether both values are the same, you can run:

```bash
diff -u \
  <(kubectl get secret cert-1 -ojson | jq '.data."tls.crt"' -r | base64 -d | openssl x509 -text -noout) \
  <(kubectl get secret cert-1 -ojson | jq '.data."cert"' -r | base64 -d | openssl x509 -text -noout)
```

If the output is empty, then secret-transform is working well.

## Renaming the key of a Secret

cert-manager doesn't support customizing the name of the keys used in the
Secrets. The keys are fixed to `tls.crt`, `tls.key`, and `ca.crt`.

You can use the three annotations below to "rename" (or rather copy) the keys of
a Secret. Let's imagine you want the Secret to have the private key stored in
the key `keyFile`, the certificate in the key `certFile`, and the CA certificate
in the key `caFile`. You can annotate your Secret with the following
annotations:

```yaml
kind: Secret
metadata:
  annotations:
    cert-manager.io/secret-copy-ca.crt: caFile    # ✨ "ca.crt" to be renamed to "caFile"
    cert-manager.io/secret-copy-tls.crt: certFile # ✨ "tls.crt" to be renamed to "certFile"
    cert-manager.io/secret-copy-tls.key: keyFile  # ✨ "tls.key" to be renamed to "keyFile"
stringData:
  tls.crt: <the PEM-encoded contents of the certificate>
  tls.key: <the PEM-encoded contents of the private key>
  ca.crt: <the PEM-encoded contents of the CA certificate>
```

After adding the annotations, you will see the new keys appear in the Secret:

```diff
 kind: Secret
 metadata:
   annotations:
     cert-manager.io/secret-copy-ca.crt: caFile
     cert-manager.io/secret-copy-tls.crt: certFile
     cert-manager.io/secret-copy-tls.key: keyFile
 data:
    tls.crt: <the PEM-encoded contents of the certificate>
    tls.key: <the PEM-encoded contents of the private key>
    ca.crt: <the PEM-encoded contents of the CA certificate>
+   certFile: <copied from tls.crt>
+   keyFile: <copied from tls.key>
+   caFile: <copied from ca.crt>
```

## Renaming of optional keystore keys

cert-manager is able to optionally provide keystores in JKS or/and PKCS#12 format.
Similar to renaming the default Keys you can use it to rename your keystore keys.

**JKS:**

```yaml
kind: Secret
metadata:
  annotations:
    cert-manager.io/secret-copy-keystore.jks: keystore      # ✨ "keystore.jks" to be renamed to "keystore"
    cert-manager.io/secret-copy-truststore.jks: truststore  # ✨ "truststore.jks" to be renamed to "truststore"
stringData:
  tls.crt: <the PEM-encoded contents of the certificate>
  tls.key: <the PEM-encoded contents of the private key>
  ca.crt: <the PEM-encoded contents of the CA certificate>
  keystore.jks: <keystore that holds the certificate and the private key>
  truststore.jks: <truststore that holds the CA certificate>
```

After adding the annotations, you will see the new keys appear in the Secret:

```diff
 kind: Secret
 metadata:
   annotations:
    cert-manager.io/secret-copy-keystore.jks: keystore
    cert-manager.io/secret-copy-truststore.jks: truststore
 data:
    tls.crt: <the PEM-encoded contents of the certificate>
    tls.key: <the PEM-encoded contents of the private key>
    ca.crt: <the PEM-encoded contents of the CA certificate>
    keystore.jks: <keystore that holds the certificate and the private key>
    truststore.jks: <truststore that holds the CA certificate>
+   keystore: <copied from keystore.jks>
+   truststore: <copied from truststore.jks>
```

**PKCS#12:**

```yaml
kind: Secret
metadata:
  annotations:
    cert-manager.io/secret-copy-keystore.p12: keystore      # ✨ "keystore.p12" to be renamed to "keystore"
    cert-manager.io/secret-copy-truststore.p12: truststore  # ✨ "truststore.p12" to be renamed to "truststore"
stringData:
  tls.crt: <the PEM-encoded contents of the certificate>
  tls.key: <the PEM-encoded contents of the private key>
  ca.crt: <the PEM-encoded contents of the CA certificate>
  keystore.p12: <keystore that holds the certificate and the private key>
  truststore.p12: <truststore that holds the CA certificate>
```

After adding the annotations, you will see the new keys appear in the Secret:

```diff
 kind: Secret
 metadata:
   annotations:
    cert-manager.io/secret-copy-keystore.p12: keystore
    cert-manager.io/secret-copy-truststore.p12: truststore
 data:
    tls.crt: <the PEM-encoded contents of the certificate>
    tls.key: <the PEM-encoded contents of the private key>
    ca.crt: <the PEM-encoded contents of the CA certificate>
    keystore.p12: <keystore that holds the certificate and the private key>
    truststore.p12: <truststore that holds the CA certificate>
+   keystore: <copied from keystore.p12>
+   truststore: <copied from truststore.p12>
```

### Use-case: Redis Enterprise for Kubernetes

If you are using Redis Enterprise for Kubernetes, the page [Manage Redis
Enterprise cluster (REC)
certificates](https://docs.redis.com/latest/kubernetes/security/manage-rec-certificates/)
will ask you to create a Secret with the following keys:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
stringData:
  name: proxy # <proxy | api | cm | syncer | metrics_exporter>
  key: <the PEM-encoded contents of the private key>
  certificate: <the PEM-encoded contents of the certificate>
```

You can use secret-transform in combination with cert-manager to obtain this
Secret.

The Secret needs to be created beforehand so that `name: proxy` shows correctly.
When a Secret already exists, cert-manager doesn't create a new one: it simply
updates `tls.crt`, `tls.key`, and `ca.crt`.

The pre-created Secret I suggest is:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: redis-cert1
  annotations:
    cert-manager.io/secret-copy-tls.crt: certificate
    cert-manager.io/secret-copy-tls.key: key
data:
  name: proxy
```

After cert-manager has filled in `tls.crt` and `tls.key`, secret-manager will
copy these two fields into `certificate` and `key`. The resulting Secret will
look like this:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: redis-cert1
  annotations:
    cert-manager.io/secret-copy-tls.crt: certificate
    cert-manager.io/secret-copy-tls.key: key
data:
  tls.crt: LS0tLCR...UdJ0tC7g==
  tls.key: CRUdJTo...Ci0tLS0t==
  ca.crt: ...
  certificate: LS0tLCR...UdJ0tC7g==
  key: CRUdJTo...Ci0tLS0t==
  name: proxy
```

### Use-case: FluxCD

FluxCD expects the keys `caFile`, `certFile`, and `keyFile`. The
`secret-transform` controller can be used to create a copy of the standard keys
so that you can use them from FluxCD.

For example, if you annotate your Secret with the following annotation:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  annotations:
    cert-manager.io/secret-copy-ca.crt: caFile
    cert-manager.io/secret-copy-tls.crt: certFile
    cert-manager.io/secret-copy-tls.key: keyFile
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg==
  tls.key: LS0tLS1CRUdJToCi0tLS0tRU5EIF...SBQUklWQVRFIEtFWS0tLS0tCg==
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg==
```

The Secret will be transformed to:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  annotations:
    cert-manager.io/secret-copy-ca.crt: caFile
    cert-manager.io/secret-copy-tls.crt: certFile
    cert-manager.io/secret-copy-tls.key: keyFile
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg==
  tls.key: LS0tLS1CRUdJToCi0tLS0tRU5EIF...SBQUklWQVRFIEtFWS0tLS0tCg==
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg==
  certFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg== # ✨
  keyFile: LS0tLS1CRUdJToCi0tLS0tRU5EIF...SBQUklWQVRFIEtFWS0tLS0tCg== # ✨
  caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FU...CBDRVJUSUZJQ0FURS0tLS0tCg== # ✨
```

## Combined PEM bundle

> [!IMPORTANT]
> The combined PEM feature provided by this addon has been added to
> cert-manager 1.7 with the field `additionalOutputFormats: CombinedPEM`.
> Since the feature is still in alpha (as of Sept 2023), you will need to use the feature
> flag `--feature-gates=AdditionalCertificateOutputFormats=true`. You can read more in the cert-manager documentation page
> [Additional Certificate Output Formats](https://cert-manager.io/docs/usage/certificate/#additional-certificate-output-formats).

Another common request reported in the cert-manager issue
[#843](https://github.com/jetstack/cert-manager/issues/843) is to create a PEM
bundle containing both the key and certificate for easier use with software that
require a unified PEM bundle, such as

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

<a id="use-case-mongodb"/>

### Use-case: MongoDB

https://github.com/jetstack/cert-manager/issues/843

In order to configure mTLS, the `mongod` and `mongos` require a combined PEM file using the key [`certificateKeyFile`](https://docs.mongodb.com/manual/tutorial/configure-ssl/). The PEM file must contain the PKCS#8 PEM-encoded private key followed by the chain of PEM-encoded X.509 certificates. The configuration looks like this:

```yaml
net:
  tls:
    mode: requireTLS
    certificateKeyFile: /etc/ssl/mongodb.pem
```

> :heavy_check_mark: secret-tranform should be able to get around this.

<a id="use-case-haproxy-community-edition-and-haproxy-enterprise-edition"/>

### Use-case: HAProxy Community Edition and HAProxy Enterprise Edition

The [`crt`](https://cbonte.github.io/haproxy-dconv/2.5/configuration.html#5.1-crt) parameter requires a PEM bundle containing the PKCS#8 private key followed by the X.509 certificate chain. An example of configuration looks like this:

```haproxy
frontend www
   bind :443 ssl crt /etc/certs/ssl.pem
```

> :heavy_check_mark: secret-tranform should be able to get around this.

<a id="use-case-hitch"/>

### Use-case: Hitch

Hitch, a reverse-proxy that aims at terminating TLS connections, requires the use of a combined PEM bundle using the configuration key [`pem-file`](https://github.com/varnish/hitch/blob/master/docs/configuration.md). The bundle must be comprised of a PKCS#8-encode private key followed by the X.509 certificate leaf followed by intermediate certificates. An example of configuration looks like this:

```hitch
pem-file = "/etc/tls/combined.pem"
```

or

```hitch
pem-file = {
    cert = "/etc/tls/combined.pem"
}
```

> :heavy_check_mark: secret-tranform should be able to get around this.

<a id="use-case-postgres-jbdc-driver-lower-than-4229"/>

### Use-case: Postgres JBDC driver (lower than 42.2.9)

If you are stuck with a version of the Postgres JDBC driver older than 42.2.9 (released before Dec 2019), [`sslkey`](https://jdbc.postgresql.org/documentation/head/ssl-client.html) refers to a file containing the PKCS#8-formated DER-encoded private key.

```java
props.setProperty("sslkey","/etc/ssl/protgres/postgresql.key");
```

> ❌ secret-transform is not able to work around this issue yet.

<a id="use-case-ejabbed"/>

### Use-case: Ejabbed

Related issue in the cert-manager repository: [Add ca.crt to TLS secret generated by ACME issuers](https://github.com/jetstack/cert-manager/issues/1571).

[Ejabbed](https://github.com/processone/ejabberd), an open-source Erlang-based XMPP server, requires all file paths given with [`certfiles`](https://docs.ejabberd.im/admin/configuration/toplevel/#certfiles) to be "valid" (i.e., not empty). The pain point is that Ejabbed fails when the `ca.crt` file is empty on disk. This makes it difficult to use Ejabberd with cert-manager, for example with the following Ejabbed configuration:

```yaml
certfiles:
  - /etc/ssl/ejabbed/tls.crt
  - /etc/ssl/ejabbed/tls.key
  - /etc/ssl/ejabbed/ca.crt # May be empty with the ACME Issuer.
```

> ❌ secret-transform is not able to work around this issue yet.

<a id="use-case-elasticsearch-elastics-and-open-distros"/>

### Use-case: Elasticsearch (Elastic's and Open Distro's)

Related to the issue on the cloud-on-k8s project: [fleet and elastic agent doesn't work without a ca.crt](https://github.com/elastic/cloud-on-k8s/issues/4790).

Elasticsearch cannot start when the `ca.crt` file is empty on disk, which may happen for ACME issued certificates. A "possible" workaround for these empty `ca.crt` could be to set [`pemtrustedcas_filepath`](https://opensearch.org/docs/latest/security-plugin/configuration/tls/#x509-pem-certificates-and-pkcs-8-keys) to the existing system CA bundle. For example, on REHL, that could be `/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem` or `/etc/ssl/cert.pem` on Alpine Linux. But Elasticsearch expects this file to exist within its config path (i.e., `/usr/share/elasticsearch/config`).

> ❌ secret-transform is not able to work around this issue yet.

### Use-case: Dovecot

Source: https://github.com/jetstack/cert-manager/issues/843#issuecomment-691693003

Dovecot is an IMAP and POP3 server. It requires separate PEM files for the certificate and private key. One person is asking for "PEM format" but I don't quite understand why. See: https://doc.dovecot.org/configuration_manual/dovecot_ssl_configuration/

> ❌ secret-transform is not able to work around this issue yet.

## Cut a New Release

We use `goreleaser`. To cut a new release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The GitHub Action will push the new Helm chart and Docker images, and a draft
GitHub release will be created.

Then, edit the draft GitHub release by rewriting the commit messages into
user-focused messages.

Finally, click "Publish" to announce the release to everyone who is watching the
repository!

> **Note:** It is also possible to run `goreleaser` locally. First, install
> Goreleaser and Helm 3.12 (or above) since we need the annotation
> `org.opencontainers.image.source`. Then, run:
>
> ```bash
> # This is a dry-run just to see if the Helm chart and the images can be build.
> goreleaser --snapshot --clean
> ```
>
> I often don't have the time to wait for GitHub Actions to run goreleaser, so I
> often run it myself:
>
> ```bash
> # This is the real deal.
> export GITHUB_TOKEN=...
> goreleaser
> ```
>
> But it is preferable to let the GitHub Action do it.

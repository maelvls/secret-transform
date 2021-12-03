# Secret Transformer

> ⚠️ The combined PEM feature provided by this addon will be added to cert-manager 1.7. See [PR #4598](https://github.com/jetstack/cert-manager/pull/4598).

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

## Use cases


- [Use-case: MongoDB](#use-case-mongodb)
- [Use-case: HAProxy Community Edition and HAProxy Enterprise Edition](#use-case-haproxy-community-edition-and-haproxy-enterprise-edition)
- [Use-case: Hitch](#use-case-hitch)
- [Use-case: Postgres JBDC driver (lower than 42.2.9)](#use-case-postgres-jbdc-driver-lower-than-4229)
- [Use-case: Ejabbed](#use-case-ejabbed)
- [Use-case: Elasticsearch (Elastic's and Open Distro's)](#use-case-elasticsearch-elastics-and-open-distros)

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

- HAProxy,
- Hitch,
- OpenDistro for Elasticsearch. The [`crt`](https://cbonte.github.io/haproxy-dconv/2.5/configuration.html#5.1-crt) parameter requires a PEM bundle containing the PKCS#8 private key followed by the X.509 certificate chain. An example of configuration looks like this:

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

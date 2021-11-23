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

**Contents:**

- [Why?](#why)
  - [Use-case: MongoDB](#use-case-mongodb)
  - [Use-case: HAProxy Community Edition and HAProxy Enterprise Edition](#use-case-haproxy-community-edition-and-haproxy-enterprise-edition)
  - [Use-case: Hitch](#use-case-hitch)
  - [Use-case: Postgres JBDC driver (lower than 42.2.9)](#use-case-postgres-jbdc-driver-lower-than-4229)
  - [Use-case: Ejabbed](#use-case-ejabbed)
  - [Use-case: Elasticsearch (Elastic's and Open Distro's)](#use-case-elasticsearch-elastics-and-open-distros)
- [Why is it not working?](#why-is-it-not-working)
- [Using cert-manager](#using-cert-manager)

## Why?

A common request reported in the cert-manager issue
[#843](https://github.com/jetstack/cert-manager/issues/843) is to "transform"
the existing `tls.key`, `tls.crt` and `ca.crt` into something else. The three
supported use-cases are:

- Transform the `tls.key` into a PKCS#8-formated DER-encoded binary private key.
  To use this mode, annotate your Secret with the following annotations:

  ```yaml
  kind: Secret
  metadata:
    annotations:
      secret-transform-in: tls.key
      secret-transform-out: tls.der # Any name with the extension `.der`.
  data:
    tls.key: ...
    tls.crt: ...
    tls.der: <der-encoded tls.key> # ✨ Automatically added!
  ```

- Bundle together the `tls.key` and `tls.crt` into a PEM-encoded concatenation
  of the PKCS#8-formated PEM-encoded private key followed by the chain of
  PEM-encoded X.509 certificates.

  ```yaml
  kind: Secret
  metadata:
  annotations:
    secret-transform-in: tls.key,tls.crt
    secret-transform-out: tls.pem # Any name with the extension `.pem`.
  data:
    tls.key: ...
    tls.crt: ...
    tls.pem: <content of tls.key followed by tls.crt> # ✨ Automatically added!
  ```

- [Why?](#why)
  - [Use-case: MongoDB](#use-case-mongodb)
  - [Use-case: HAProxy Community Edition and HAProxy Enterprise Edition](#use-case-haproxy-community-edition-and-haproxy-enterprise-edition)
  - [Use-case: Hitch](#use-case-hitch)
  - [Use-case: Postgres JBDC driver (lower than 42.2.9)](#use-case-postgres-jbdc-driver-lower-than-4229)
  - [Use-case: Ejabbed](#use-case-ejabbed)
  - [Use-case: Elasticsearch (Elastic's and Open Distro's)](#use-case-elasticsearch-elastics-and-open-distros)
- [Why is it not working?](#why-is-it-not-working)
- [Using cert-manager](#using-cert-manager)

> Note that the secret-transform controller only outputs PEM and DER. If you
> would like to use the PKCS#12 encoding (`.p12`, `.pfx`) or the JKS encoding
> (`.jks`) to store both the private key and certificate, cert-manager already
> provides a mechanism since cert-manager 0.14. You can refer to the section
> [General Availability of JKS and PKCS#12
> keystores](https://cert-manager.io/docs/release-notes/release-notes-0.15/#general-availability-of-jks-and-pkcs-12-keystores)
> on the cert-manager website for more information.
>
> Here is a list of projects that can be used with the cert-manager 0.14
> JKS/PKCS#12 support:
>
> - Open Distro for Elasticsearch (OpenSearch),
> - Elastic's Elasticsearch,
> - PostgreSQL JDBC driver using the
>   [`sslkey`](https://jdbc.postgresql.org/documentation/head/ssl-client.html)
>   with the PKCS#12 bundle; this is available as of 42.2.9 (Dec 2019).

### Use-case: MongoDB

In order to configure mTLS, the `mongod` and `mongos` require a combined PEM file using the key [`certificateKeyFile`](https://docs.mongodb.com/manual/tutorial/configure-ssl/). The PEM file must contain the PKCS#8 PEM-encoded private key followed by the chain of PEM-encoded X.509 certificates. The configuration looks like this:

```yaml
net:
  tls:
    mode: requireTLS
    certificateKeyFile: /etc/ssl/mongodb.pem
```

> :heavy_check_mark: secret-tranform should be able to get around this.

### Use-case: HAProxy Community Edition and HAProxy Enterprise Edition

- HAProxy,
- Hitch,
- OpenDistro for Elasticsearch. The [`crt`](https://cbonte.github.io/haproxy-dconv/2.5/configuration.html#5.1-crt) parameter requires a PEM bundle containing the PKCS#8 private key followed by the X.509 certificate chain. An example of configuration looks like this:

```haproxy
frontend www
   bind :443 ssl crt /etc/certs/ssl.pem
```

> :heavy_check_mark: secret-tranform should be able to get around this.

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

### Use-case: Postgres JBDC driver (lower than 42.2.9)

If you are stuck with a version of the Postgres JDBC driver older than 42.2.9 (released before Dec 2019), [`sslkey`](https://jdbc.postgresql.org/documentation/head/ssl-client.html) refers to a file containing the PKCS#8-formated DER-encoded private key.

```java
props.setProperty("sslkey","/etc/ssl/protgres/postgresql.key");
```

> :heavy_check_mark: secret-tranform should be able to get around this.

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

### Use-case: Elasticsearch (Elastic's and Open Distro's)

Related to the issue on the cloud-on-k8s project: [fleet and elastic agent doesn't work without a ca.crt](https://github.com/elastic/cloud-on-k8s/issues/4790).

Elasticsearch cannot start when the `ca.crt` file is empty on disk, which may happen for ACME issued certificates.

> ❌ secret-transform is not able to work around this issue yet.

<!--
## Use-case: Java applications relying on the default SSLSocketFactory

When an application relies on the default `javax.net.ssl.SSLSocketFactory`, the
private key and certificate chain must be bundled into a Java Key Store (JKS).
Configuring the JKS file to be used usually looks like this:

```sh
JAVA_OPTS=$JAVA_OPTS -Djavax.net.ssl.keyStore=/etc/certs/ssl.jks -Djavax.net.ssl.keyStore=jks
```
-->

## Why is it not working?

When running `kubectl describe secret example-secret`, the events are not shown,
which means you won't be able to diagnose when secret-transform does not seem to
be working.

```sh
kubectl get events --field-selector involvedObject.name=example-cert
```

## Using cert-manager

> The Issuer field `secretTemplate` requires cert-manager v1.5.0 or later.

If you are using Certificate resources:

```yaml
kind: Certificate
spec:
  secretTemplate:
    annotations:
      secret-transform: tls.key,tls.crt:tls.pem
```

If you are using the ACME Issuer with the "ingress-shim" mechanism to get
Certificate resources automatically created for your Ingress resources,
secret-transform does not integrate well and you will have to create manually
the Certificate resource instead and use `secretTemplate`.

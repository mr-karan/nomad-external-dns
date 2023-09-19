<a href="https://zerodha.tech"><img src="https://zerodha.tech/static/images/github-badge.svg" align="right" /></a>

# nomad-external-dns

_Synchronize Nomad Services with external DNS providers._

Inspired by [kubernetes-sigs/external-dns](https://github.com/kubernetes-sigs/external-dns), `nomad-external-dns` makes Nomad Services discoverable via DNS servers.
Nomad 1.3+ [bundles support](https://www.hashicorp.com/blog/nomad-1-3-adds-native-service-discovery-and-edge-workload-support) for native service discovery and `nomad-external-dns` helps to advertise the services inside this registry to external DNS providers.

## Supported Providers

* [AWS Route 53](https://aws.amazon.com/route53/)
* [CloudFlare](https://www.cloudflare.com/dns)

## How it Works

`nomad-external-dns` uses the concept of "Annotated Tags" to set properties for the DNS records. Here's an example of a `service` stanza inside a Nomad jobspec:

```hcl
    service {
      provider = "nomad"
      name     = "redis-cache"
      tags = [
        "external-dns/hostname=redis.test.internal",
        "external-dns/ttl=30s",
      ]
      port = "db"
    }
```

- At every `app.update_interval` frequency, list of all services across namespaces in the Nomad cluster are fetched.
- For each service, `external-dns` prefix is used to determine properties like TTL, Hostname etc.
- DNS record for this service is created with the registered DNS Provider. `nomad-external-dns` creates or updates an existing record automatically.

## Deploy

NOTE: This is meant to run inside a Nomad cluster and should have proper ACL to query for services across multiple namespaces.

You can choose one of the various deployment options:

### Binary

Grab the latest release from [Releases](https://github.com/mr-karan/nomad-external-dns/releases).

To run:

```
$ ./nomad-external-dns.bin --config config.toml
```

### Nomad

Refer to the [jobspec](./docs/nomad.md#jobspec) for deploying in a Nomad cluster.

If you're deploying on AWS, consider referring to the IAM policy mentioned [here](./docs/aws.md#iam-policy)

## Configuration

Refer to [config.sample.toml](./config.sample.toml) for a list of configurable values.

### Environment Variables

All config variables can also be populated as env vairables by prefixing `NOMAD_EXTERNAL_DNS_` and replacing `.` with `__`.

For eg: `app.update_interval` becomes `NOMAD_EXTERNAL_DNS_app__update_interval`.

For configuring Nomad API client, [these environment variables](https://www.nomadproject.io/docs/runtime/environment) can be set.

## Contribution

- Support for new providers can be added by registering more providers using [libdns](https://github.com/libdns/libdns).
- Feel free to report any bugs/feature requests.

## LICENSE

[LICENSE](./LICENSE)

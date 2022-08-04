# Deploying on Nomad

## Jobspec

This is an example jobspec that you can refer to for deploying `nomad-external-dns`. This example uses the `raw_exec` mode and pulls the binary from GitHub Releases.

```hcl
job "nomad-external-dns" {
  datacenters = ["dc1"]
  namespace   = "default"
  type        = "service"

  group "nomad-external-dns" {
    count = 1

    network {
      mode = "host"
    }

    task "app" {
      driver = "raw_exec"

      artifact {
        source = "https://github.com/mr-karan/nomad-external-dns/releases/download/v0.1.3/nomad-external-dns_0.1.3_linux_amd64.tar.gz"
      }

      env {
        NOMAD_TOKEN           = "xxx"
        AWS_ACCESS_KEY_ID     = "yyy"
        AWS_SECRET_ACCESS_KEY = "zzz"
      }

      config {
        command = "$${NOMAD_TASK_DIR}/nomad-external-dns.bin"
        args = [
          "--config",
          "$${NOMAD_TASK_DIR}/config.sample.toml"
        ]
      }

      resources {
        cpu    = 500
        memory = 500
      }
    }
  }
}
```

## Notes

- If ACL is enabled, then you must generate and provide a `NOMAD_TOKEN` variable.
- The service must be able to access the Nomad Cluster API. You can configure other Nomad variables using `env` stanza.

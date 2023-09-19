job "http" {
  datacenters = ["dc1"]
  type        = "service"

  group "http" {
    count = 1

    network {
      mode = "host"
      port "http" {
        to = 8000
      }
    }

    service {
      provider = "nomad"
      name     = "python-http"
      tags = [
        "external-dns/hostname=python.mrkaran.com",
        "external-dns/ttl=60s",
      ]
      port = "http"
    }

    task "app" {
      driver = "raw_exec"

      config {
        command = "/bin/bash"
        args    = ["-c", "python3 -m http.server"]
      }

      resources {
        cpu    = 100 # MHz
        memory = 128 # MB
      }
    }
  }
}

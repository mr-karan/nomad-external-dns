job "redis" {
  datacenters = ["dc1"]

  type = "service"

  group "cache" {
    count = 3

    network {
      port "db" {
        to = 6379
      }
    }

    service {
      provider = "nomad"
      name     = "redis-cache"
      tags = [
        "external-dns/hostname=redis.test.internal",
        "external-dns/ttl=30s",
      ]
      port = "db"

    }

    task "redis" {
      driver = "docker"

      config {
        image = "redis:7"

        ports = ["db"]
      }
    }
  }
}

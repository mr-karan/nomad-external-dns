FROM ubuntu:22.04
RUN apt-get -y update && apt install -y ca-certificates

WORKDIR /app
COPY nomad-external-dns.bin .
COPY config.sample.toml config.toml

ENTRYPOINT [ "./nomad-external-dns.bin" ]

# Cloudflare

## API Token

Refer to [libdns/cloudflare](https://github.com/libdns/cloudflare#authenticating) for more details.

Create a scoped API token with following permissions:

- `Zone / Zone / Read`
- `Zone / DNS / Edit`

## Note

- Check your Cloudflare account to see TTL granularity. Some business/enterprise accounts have TTLs of minimum 30s, but free accounts have minimum TTL of 1min. In that case, adjust that `external-dns/ttl` accordingly.

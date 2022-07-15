# Nomad External DNS

Update external DNS records for services registered with Nomad's service registry.

## Basic Idea

- Deploy an app as a service/system job.
- Register the service wit Nomad Service Registry
- A "controller" (event loop) is listening to new service events.
- Watches if a new service has come up, fetches the metadata/annotation/hostname and the IPs
- Calls dns.Update()
  - This is an interface for external providers. Handle Cloudflare and AWS R53 for 0.1

## Events to handle

- New service => New records 
- Remove service => Delete records
- Update service => Update records

## Notes

R53 can do multiple entries for A records. Simple load balancing strategy is enough.

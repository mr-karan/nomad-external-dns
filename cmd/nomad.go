package main

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
)

// fetchNomadServices retrieves all services from the Nomad API
// and returns a map where of services where the key is the unique ID of the service.
func (app *App) fetchNomadServices() (map[string]ServiceMeta, error) {
	services := make(map[string]ServiceMeta)

	// Fetch the list of services
	serviceList, err := app.fetchServiceList()
	if err != nil {
		return nil, err
	}

	// Iterate over each service to fetch its metadata
	for _, l := range serviceList {
		for _, s := range l.Services {
			svcMeta, err := app.fetchServiceMeta(l.Namespace, s.ServiceName)
			if err != nil {
				return nil, err
			}

			// If metadata exists, store it in the services map.
			if svcMeta != nil {
				services[EnsureFQDN(svcMeta.DNSName)] = *svcMeta
			}
		}
	}

	return services, nil
}

// fetchServiceList retrieves the list of services from the Nomad API.
func (app *App) fetchServiceList() ([]*api.ServiceRegistrationListStub, error) {
	servicesList, _, err := app.nomadClient.Services().List(&api.QueryOptions{Namespace: "*"})
	if err != nil {
		return nil, fmt.Errorf("error listing services: %w", err)
	}
	app.lo.Debug("Fetched service list with count", "count", len(servicesList))
	return servicesList, nil
}

// fetchServiceMeta fetches the metadata for a single service.
func (app *App) fetchServiceMeta(namespace, serviceName string) (*ServiceMeta, error) {
	// Fetch the service details
	svcRegistrations, _, err := app.nomadClient.Services().Get(serviceName, &api.QueryOptions{Namespace: namespace})
	if err != nil {
		return nil, fmt.Errorf("error fetching service detail: %w", err)
	}

	// If there are no service registrations or no tags, return nil.
	if len(svcRegistrations) == 0 || len(svcRegistrations[0].Tags) == 0 {
		return nil, nil
	}

	// Check if the service has a hostname annotation, if not, ignore the service.
	if !hasHostnameAnnotation(svcRegistrations[0].Tags) {
		app.lo.Debug("Hostname not found in tags, ignoring service", "service", svcRegistrations[0].ServiceName)
		return nil, nil
	}

	// Create a ServiceMeta object and return its reference.
	svcMeta := ServiceMeta{
		Name:      svcRegistrations[0].ServiceName,
		Namespace: svcRegistrations[0].Namespace,
		Job:       svcRegistrations[0].JobID,
		Tags:      svcRegistrations[0].Tags,
		Addresses: uniqueAddresses(svcRegistrations),
		DNSName:   getDNSNameFromTags(svcRegistrations[0].Tags),
	}
	return &svcMeta, nil
}

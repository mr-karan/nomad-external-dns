package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/libdns/libdns"
	"github.com/mr-karan/nomad-external-dns/internal/models"
	"github.com/mr-karan/nomad-external-dns/internal/utils"
	"github.com/zerodha/logf"
)

// DNSProvider wraps the required libdns interfaces.
// The providers must satisfy this interface.
type DNSProvider interface {
	libdns.RecordSetter
	libdns.RecordDeleter
}

type ServiceMeta struct {
	sync.RWMutex

	Name      string `json:"name"`
	Namespace string
	Job       string
	ID        string
	Domains   []string

	// Internal fields.
	log           logf.Logger
	pendingUpdate bool
	client        api.Client
	wg            *sync.WaitGroup
	events        chan *api.ServiceRegistration // Receive service events.
	delete        bool                          // Whether to delete the records in upstream DNS providers.
	provider      DNSProvider                   // Provider interface for managing records.
	addr          []string                      // Address of all backend services which is fetched by calling Nomad HTTP API.
	tags          []string                      // Tags in the given service.
}

type Opts struct {
	Name      string
	Namespace string
	Job       string
	ID        string
	Domains   []string
	Provider  DNSProvider
	Client    api.Client
	Logger    logf.Logger
}

func InitController(opts Opts) *ServiceMeta {
	svcMeta := &ServiceMeta{
		log:           opts.Logger,
		Name:          opts.Name,
		Namespace:     opts.Namespace,
		Job:           opts.Job,
		ID:            opts.ID,
		Domains:       opts.Domains,
		provider:      opts.Provider,
		client:        opts.Client,
		addr:          make([]string, 0),
		tags:          make([]string, 0),
		events:        make(chan *api.ServiceRegistration, 10),
		wg:            &sync.WaitGroup{},
		pendingUpdate: true,
	}

	return svcMeta
}

func (s *ServiceMeta) Start() {
	s.log.Info("starting service controller for", "svc", s.Name, "namespace", s.Namespace)
	// Start a worker for processing the events received.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.processEvent()
		fmt.Println("I am called!")
	}()

	// Start a background ticker for batching updates to DNS provider periodically.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.propogateDNSUpdates()
		fmt.Println("I am called!!!")
	}()
}

func (s *ServiceMeta) Destroy() {
}

// Receive adds new service registation objects to the queue.
// A worker routine is running to process this queue.
func (s *ServiceMeta) Receive(event *api.ServiceRegistration) {
	s.pendingUpdate = true
	s.events <- event
	s.log.Debug("adding new event", "svc", event.ServiceName, "namespace", event.Namespace)
}

// processEvent iterates on the events receiving queue and fetches the address
// for a given service from Nomad APIs.
// It's a blocking function.
func (s *ServiceMeta) processEvent() {
	defer close(s.events)

	// Receive new service events from the events stream.
	for svc := range s.events {
		s.log.Debug("processing event", "svc", svc.ServiceName, "namespace", svc.Namespace)
		// Fetch all the Address of this service object.
		svcObj, _, err := s.client.Services().Get(svc.ServiceName, &api.QueryOptions{Namespace: svc.Namespace})
		if err != nil {
			s.log.Error("error fetching service object", "error", err, "svc", svc.ServiceName, "namespace", svc.Namespace)
			continue
		}

		// Add all the addresses here.
		addr := make([]string, 0, len(svcObj))
		for _, s := range svcObj {
			// Only append the service if it's not added before. Otherwise AWS complains of a duplicate entry.
			if !utils.Contains(addr, s.Address) {
				addr = append(addr, s.Address)
			}
		}

		// Set the updated address and tags here.
		s.Lock()
		s.tags = svc.Tags
		// If there are no address from the API, then the service is completely de-registered.
		// In this case, delete the record from DNS Provider.
		// But AWS R53 provider needs the old records, so in that case we have to store old address.
		if len(addr) == 0 {
			s.delete = true
		} else {
			s.addr = addr
		}
		s.log.Debug("setting meta to service object as", "addr", s.addr, "tags", s.tags, "delete", s.delete)
		s.Unlock()
	}

}

// propogateDNSUpdates handles DNS updates
func (s *ServiceMeta) propogateDNSUpdates() {
	var (
		ticker = time.NewTicker(time.Second * 3)
	)
	defer ticker.Stop()
	defer fmt.Println("ticker is exiting bye")

	for range ticker.C {
		fmt.Println("ticker loop")
		s.Lock()
		if s.pendingUpdate {
			record, err := s.ToRecord()
			if err != nil {
				s.log.Error("error converting service to record", "error", err)
				continue
			}
			if s.delete {
				// A crude hack. If the program restarts the record.Value is empty.
				// We can check if it's empty and then set to 127.0.0.1 and then immediately delete.
				records, err := s.provider.DeleteRecords(context.Background(), record.Zone, []libdns.Record{record.Record})
				if err != nil {
					s.log.Error("error deleting records from zone", "error", err)
					continue
				}
				fmt.Println(records)
			}
			records, err := s.provider.SetRecords(context.Background(), record.Zone, []libdns.Record{record.Record})
			if err != nil {
				s.log.Error("error adding records to zone", "error", err)
				continue
			}
			fmt.Println(records)
		}
		s.pendingUpdate = false
		s.Unlock()
		s.log.Debug("hello")

	}

}

// Converts a service meta object to a libdns record.
// This is used to send to upstream DNS providers.
func (s *ServiceMeta) ToRecord() (models.RecordMeta, error) {
	var (
		host   string
		zone   string
		err    error
		ttl    time.Duration
		record models.RecordMeta
	)
	// We extract the hostname/TTL from the annotated tags, so return if they're not present in the svc object.
	if len(s.tags) == 0 {
		return record, fmt.Errorf("tags cannot be empty")
	}

	// Parse the hostname and TTL from tags.
	for _, tag := range s.tags {
		if strings.HasPrefix(tag, models.HostnameAnnotationKey) {
			host, zone, err = utils.GetRecordName(strings.Split(tag, models.HostnameAnnotationKey+"=")[1], s.Domains)
			if err != nil {
				return record, fmt.Errorf("error fetching hostname from tags: %w", err)
			}
		}
		if strings.HasPrefix(tag, models.TTLAnnotationKey) {
			ttl, err = utils.ParseTTL(strings.Split(tag, models.TTLAnnotationKey+"=")[1])
			if err != nil {
				s.log.Warn("error parsing ttl, setting default", "error", err)
				ttl = models.DefaultTTL
			}
		}
	}
	// At this point both the host and zone should be non empty.
	if host == "" && zone == "" {
		return record, fmt.Errorf("error parsing host: %s or zone: %s", host, zone)
	}

	// Format the zone as a proper FQDN.
	if zone[len(zone)-1:] != "." {
		zone = zone + "."
	}

	// Prepare a record object.
	record = models.RecordMeta{
		Record: libdns.Record{
			Type:  "A",
			Name:  strings.TrimSpace(host),
			Value: strings.Join(s.addr, ","),
			TTL:   ttl,
		},
		Zone: zone,
	}
	return record, nil

}

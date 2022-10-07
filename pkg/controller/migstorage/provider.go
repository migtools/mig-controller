package migstorage

import (
	"context"
	"time"

	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	event2 "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// A cloud provider `watch` source used to routinely run provider tests.
//
//	Client - A controller-runtime client.
//	Interval - The connection test interval.
//	Namespace - Only objects in this namespace are queued
type ProviderSource struct {
	Client     client.Client
	Interval   time.Duration
	Namespace  string
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
}

// Start the source.
func (p *ProviderSource) Start(
	ctx context.Context,
	handler handler.EventHandler,
	queue workqueue.RateLimitingInterface,
	predicates ...predicate.Predicate) error {

	p.handler = handler
	p.queue = queue
	p.predicates = predicates
	go p.run()

	return nil
}

// Run the scheduled connection tests.
func (p *ProviderSource) run() {
	for {
		time.Sleep(p.Interval)
		list, err := v1alpha1.ListStorage(p.Client)
		if err != nil {
			log.Trace(err)
			return
		}
		for _, storage := range list {
			if p.Namespace != "" && p.Namespace != storage.Namespace {
				continue
			}
			if storage.Status.HasAnyCondition(
				InvalidBSProvider,
				InvalidBSCredsSecretRef,
				InvalidBSFields,
				InvalidVSProvider,
				InvalidVSCredsSecretRef,
				InvalidVSFields) {
				continue
			}
			// Storage Provider
			secret, err := storage.GetBackupStorageCredSecret(p.Client)
			if err != nil {
				log.Trace(err)
				return
			}
			provider := storage.GetBackupStorageProvider()
			err = provider.Test(secret)
			if (!storage.Status.HasCondition(BSProviderTestFailed) && err != nil) ||
				(storage.Status.HasCondition(BSProviderTestFailed) && err == nil) {
				p.enqueue(storage)
				continue
			}
			// Snapshot Provider
			secret, err = storage.GetVolumeSnapshotCredSecret(p.Client)
			if err != nil {
				log.Trace(err)
				return
			}
			provider = storage.GetVolumeSnapshotProvider()
			err = provider.Test(secret)
			if (!storage.Status.HasCondition(VSProviderTestFailed) && err != nil) ||
				(storage.Status.HasCondition(VSProviderTestFailed) && err == nil) {
				p.enqueue(storage)
				continue
			}
		}
	}
}

// Enqueue a reconcile request.
func (p *ProviderSource) enqueue(storage v1alpha1.MigStorage) {
	event := event2.GenericEvent{
		Object: &storage,
	}
	for _, predicate := range p.predicates {
		if !predicate.Generic(event) {
			return
		}
	}

	p.handler.Generic(event, p.queue)
}

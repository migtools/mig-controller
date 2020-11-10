package container

import (
	"context"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

//
// A collection of k8s Backup resources.
type Backup struct {
	// Base
	BaseCollection
}

func (r *Backup) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &velero.Backup{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *Backup) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{
		Db: r.ds.Container.Db,
	}
	err := sr.Reconcile(r)
	if err != nil {
		Log.Trace(err)
		return err
	}
	r.hasReconciled = true
	Log.Info(
		"Backup (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *Backup) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := velero.BackupList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		backup := &model.Backup{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		backup.With(&discovered)
		models = append(models, backup)
	}

	return models, nil
}

func (r *Backup) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.Backup{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, backup := range list {
		models = append(models, backup)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *Backup) Create(e event.CreateEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*velero.Backup)
	if !cast {
		return false
	}
	backup := model.Backup{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	backup.With(object)
	r.ds.Create(&backup)

	return false
}

func (r *Backup) Update(e event.UpdateEvent) bool {
	Log.Reset()
	object, cast := e.ObjectNew.(*velero.Backup)
	if !cast {
		return false
	}
	restore := model.Backup{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *Backup) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*velero.Backup)
	if !cast {
		return false
	}
	backup := model.Backup{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	backup.With(object)
	r.ds.Delete(&backup)

	return false
}

func (r *Backup) Generic(e event.GenericEvent) bool {
	return false
}

//
// A collection of k8s Restore resources.
type Restore struct {
	// Base
	BaseCollection
}

func (r *Restore) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &velero.Restore{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *Restore) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{
		Db: r.ds.Container.Db,
	}
	err := sr.Reconcile(r)
	if err != nil {
		Log.Trace(err)
		return err
	}
	r.hasReconciled = true
	Log.Info(
		"Restore (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *Restore) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := velero.RestoreList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		restore := &model.Restore{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		restore.With(&discovered)
		models = append(models, restore)
	}

	return models, nil
}

func (r *Restore) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.Restore{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, restore := range list {
		models = append(models, restore)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *Restore) Create(e event.CreateEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*velero.Restore)
	if !cast {
		return false
	}
	restore := model.Restore{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	restore.With(object)
	r.ds.Create(&restore)

	return false
}

func (r *Restore) Update(e event.UpdateEvent) bool {
	Log.Reset()
	object, cast := e.ObjectNew.(*velero.Restore)
	if !cast {
		return false
	}
	restore := model.Restore{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *Restore) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*velero.Restore)
	if !cast {
		return false
	}
	restore := model.Restore{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	restore.With(object)
	r.ds.Delete(&restore)

	return false
}

func (r *Restore) Generic(e event.GenericEvent) bool {
	return false
}

//
// A collection of k8s PodVolumeBackup resources.
type PodVolumeBackup struct {
	// Base
	BaseCollection
}

func (r *PodVolumeBackup) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &velero.PodVolumeBackup{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *PodVolumeBackup) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{
		Db: r.ds.Container.Db,
	}
	err := sr.Reconcile(r)
	if err != nil {
		Log.Trace(err)
		return err
	}
	r.hasReconciled = true
	Log.Info(
		"PodVolumeBackup (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *PodVolumeBackup) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := velero.PodVolumeBackupList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		backup := &model.PodVolumeBackup{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		backup.With(&discovered)
		models = append(models, backup)
	}

	return models, nil
}

func (r *PodVolumeBackup) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.PodVolumeBackup{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, backup := range list {
		models = append(models, backup)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *PodVolumeBackup) Create(e event.CreateEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*velero.PodVolumeBackup)
	if !cast {
		return false
	}
	backup := model.PodVolumeBackup{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	backup.With(object)
	r.ds.Create(&backup)

	return false
}

func (r *PodVolumeBackup) Update(e event.UpdateEvent) bool {
	Log.Reset()
	object, cast := e.ObjectNew.(*velero.PodVolumeBackup)
	if !cast {
		return false
	}
	backup := model.PodVolumeBackup{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	backup.With(object)
	r.ds.Update(&backup)

	return false
}

func (r *PodVolumeBackup) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*velero.PodVolumeBackup)
	if !cast {
		return false
	}
	backup := model.PodVolumeBackup{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	backup.With(object)
	r.ds.Delete(&backup)

	return false
}

func (r *PodVolumeBackup) Generic(e event.GenericEvent) bool {
	return false
}

//
// A collection of k8s PodVolumeRestore resources.
type PodVolumeRestore struct {
	// Base
	BaseCollection
}

func (r *PodVolumeRestore) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &velero.PodVolumeRestore{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *PodVolumeRestore) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{
		Db: r.ds.Container.Db,
	}
	err := sr.Reconcile(r)
	if err != nil {
		Log.Trace(err)
		return err
	}
	r.hasReconciled = true
	Log.Info(
		"PodVolumeRestore (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *PodVolumeRestore) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := velero.PodVolumeRestoreList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		restore := &model.PodVolumeRestore{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		restore.With(&discovered)
		models = append(models, restore)
	}

	return models, nil
}

func (r *PodVolumeRestore) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.PodVolumeRestore{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, restore := range list {
		models = append(models, restore)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *PodVolumeRestore) Create(e event.CreateEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*velero.PodVolumeRestore)
	if !cast {
		return false
	}
	restore := model.PodVolumeRestore{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	restore.With(object)
	r.ds.Create(&restore)

	return false
}

func (r *PodVolumeRestore) Update(e event.UpdateEvent) bool {
	Log.Reset()
	object, cast := e.ObjectNew.(*velero.PodVolumeRestore)
	if !cast {
		return false
	}
	restore := model.PodVolumeRestore{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *PodVolumeRestore) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*velero.PodVolumeRestore)
	if !cast {
		return false
	}
	restore := model.PodVolumeRestore{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	restore.With(object)
	r.ds.Delete(&restore)

	return false
}

func (r *PodVolumeRestore) Generic(e event.GenericEvent) bool {
	return false
}

package web

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	auth "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
)

// Plan route root.
const (
	PlanParam = "plan"
	PlansRoot = Root + "/plans"
	PlanRoot  = PlansRoot + "/:" + PlanParam
)

//
// Plan (route) handler.
type PlanHandler struct {
	// Base
	BaseHandler
	// Plan referenced in the request.
	plan model.Plan
}

//
// Add routes.
func (h PlanHandler) AddRoutes(r *gin.Engine) {
	r.GET(PlansRoot, h.List)
	r.GET(PlansRoot+"/", h.List)
	r.GET(PlanRoot, h.Get)
	r.GET(PlanRoot+"/pods", h.Pods)
	r.GET(PlanRoot+"/tree", h.Tree)
}

//
// Prepare to fulfil the request.
// Fetch the referenced plan.
// Perform SAR authorization.
func (h *PlanHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(PlanParam)
	if name != "" {
		h.plan = model.Plan{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(PlanParam),
			},
		}
		err := h.plan.Get(h.container.Db)
		if err != nil {
			if err != sql.ErrNoRows {
				Log.Trace(err)
				return http.StatusInternalServerError
			} else {
				return http.StatusNotFound
			}
		}
	}
	status = h.allow(h.getSAR())
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

// Build the appropriate SAR object.
// The subject is the MigPlan.
func (h *PlanHandler) getSAR() auth.SelfSubjectAccessReview {
	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:     "apps",
				Resource:  "MigPlan",
				Namespace: h.plan.Namespace,
				Name:      h.plan.Name,
				Verb:      "get",
			},
		},
	}
}

//
// List all of the plans in the namespace.
func (h PlanHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Plan{}
	count, err := collection.Count(db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	list, err := collection.List(db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := PlanList{
		Count: count,
	}
	for _, m := range list {
		r := Plan{}
		r.With(m)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific plan.
func (h PlanHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.plan.Get(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		} else {
			ctx.Status(http.StatusNotFound)
			return
		}
	}
	r := Plan{}
	r.With(&h.plan)
	r.SelfLink = h.Link(&h.plan)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific migration tree.
func (h PlanHandler) Tree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	tree := PlanTree{
		db:   h.container.Db,
		plan: &h.plan,
	}
	root, err := tree.Build()
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		} else {
			ctx.Status(http.StatusNotFound)
			return
		}
	}

	content := root

	ctx.JSON(http.StatusOK, content)
}

//
// Get the `PlanPods` for the plan.
func (h PlanHandler) Pods(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	content := PlanPods{}
	content.With(ctx, &h)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link.
func (h PlanHandler) Link(m *model.Plan) string {
	return h.BaseHandler.Link(
		PlanRoot,
		Params{
			NsParam:   m.Namespace,
			PlanParam: m.Name,
		})
}

//
// Tree node.
type TreeNode struct {
	Kind       string     `json:"kind"`
	Namespace  string     `json:"namespace"`
	Name       string     `json:"name"`
	Children   []TreeNode `json:"children,omitempty"`
	ObjectLink string     `json:"objectLink"`
}

//
// Plan Tree.
type PlanTree struct {
	// Subject.
	plan *model.Plan
	// DB client.
	db model.DB
	// Plan clusters.
	cluster struct {
		// Source cluster.
		source model.Cluster
		// Destination cluster.
		destination model.Cluster
	}
}

//
// Build the tree.
func (t *PlanTree) Build() (*TreeNode, error) {
	err := t.setCluster()
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	root := t.getRoot()
	err = t.addMigrations(root)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}

	return root, nil
}

//
// Get the relevant correlation labels.
func (t *PlanTree) cLabel(r MigResource) model.Label {
	k, v := r.GetCorrelationLabel()
	return model.Label{
		Name:  k,
		Value: v,
	}
}

//
// Set the cluster field based on
// the referenced plan.
func (t *PlanTree) setCluster() error {
	planObject := t.plan.DecodeObject()
	sRef := planObject.Spec.SrcMigClusterRef
	dRef := planObject.Spec.DestMigClusterRef
	cluster := model.Cluster{
		CR: model.CR{
			Namespace: sRef.Namespace,
			Name:      sRef.Name,
		},
	}
	err := cluster.Get(t.db)
	if err != nil {
		Log.Trace(err)
		return err
	}
	t.cluster.source = cluster
	cluster = model.Cluster{
		CR: model.CR{
			Namespace: dRef.Namespace,
			Name:      dRef.Name,
		},
	}
	err = cluster.Get(t.db)
	if err != nil {
		Log.Trace(err)
		return err
	}
	t.cluster.destination = cluster

	return nil
}

//
// Build root node.
func (t *PlanTree) getRoot() *TreeNode {
	root := &TreeNode{
		Kind:       migref.ToKind(t.plan),
		ObjectLink: PlanHandler{}.Link(t.plan),
		Namespace:  t.plan.Namespace,
		Name:       t.plan.Name,
	}

	return root
}

//
// Add related migrations.
func (t *PlanTree) addMigrations(parent *TreeNode) error {
	collection := model.Migration{}
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		migration := m.DecodeObject()
		ref := migration.Spec.MigPlanRef
		if !migref.RefSet(ref) {
			continue
		}
		if ref.Namespace != t.plan.Namespace ||
			ref.Name != t.plan.Name {
			continue
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: MigrationHandler{}.Link(m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}

		err = t.addBackups(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		err = t.addRestores(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		err = t.addDirectVolumes(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		err = t.addDirectImages(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		err = t.addMigrationPods(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		parent.Children = append(parent.Children, node)
	}

	return nil
}

//
// Add related velero Backups.
func (t *PlanTree) addBackups(migration *model.Migration, parent *TreeNode) error {
	cluster := t.cluster.source
	collection := model.Backup{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	cLabel := t.cLabel(migration.DecodeObject())
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		if object.Labels == nil {
			continue
		}
		if v, found := object.Labels[cLabel.Name]; found {
			if v != cLabel.Value {
				continue
			}
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: BackupHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}
		err := t.addPvBackups(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		// Add move and snapshot PVCs if this is a stage backup
		_, found := object.Labels["migration-stage-backup"]
		if found {
			err = t.addMoveAndSnapshotPVCsForCluster(cluster, &node)
			if err != nil {
				Log.Trace(err)
				return err
			}
		}
		parent.Children = append(parent.Children, node)
	}

	return nil
}

// Add PVCs that will be moved or snapshotted
func (t *PlanTree) addMoveAndSnapshotPVCsForCluster(cluster model.Cluster, parent *TreeNode) error {
	collection := model.PVC{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	// Get list of PVC names that will be moved / snapshotted from MigPlan
	plan := t.plan.DecodeObject()
	pvcsToInclude := []migapi.PVC{}
	for _, pv := range plan.Spec.PersistentVolumes.List {
		// Add to list if not using file copy|skip method
		if pv.Selection.Action != migapi.PvCopyAction && pv.Selection.Action != migapi.PvSkipAction {
			pvcsToInclude = append(pvcsToInclude, pv.PVC)
		}
	}

	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		for _, pvc := range pvcsToInclude {
			if pvc.Name == object.Name && pvc.Namespace == object.Namespace {
				node := TreeNode{
					Kind:       migref.ToKind(m),
					ObjectLink: PvcHandler{}.Link(&cluster, m),
					Namespace:  m.Namespace,
					Name:       m.Name,
				}
				err := t.addPVForPVC(cluster, m, &node)
				if err != nil {
					Log.Trace(err)
					return err
				}
				parent.Children = append(parent.Children, node)
				break
			}
		}
	}
	return nil
}

//
// Add related velero Restores.
func (t *PlanTree) addRestores(migration *model.Migration, parent *TreeNode) error {
	cluster := t.cluster.destination
	collection := model.Restore{
		Base: model.Base{
			Cluster: model.Cluster{
				CR: model.CR{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
				}}.PK,
		},
	}
	cLabel := t.cLabel(migration.DecodeObject())
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		if object.Labels == nil {
			continue
		}
		if v, found := object.Labels[cLabel.Name]; found {
			if v != cLabel.Value {
				continue
			}
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: RestoreHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}
		err := t.addPvRestores(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		// Add move and snapshot PVCs if this is a stage restore
		_, found := object.Labels["migration-stage-restore"]
		if found {
			err = t.addMoveAndSnapshotPVCsForCluster(cluster, &node)
			if err != nil {
				Log.Trace(err)
				return err
			}
		}
		parent.Children = append(parent.Children, node)
	}

	return nil
}

// Add direct volume pods
func (t *PlanTree) addMigrationPods(migration *model.Migration, parent *TreeNode) error {
	// Source cluster pods
	err := t.addMigrationPodsForCluster(t.cluster.source, migration, parent)
	if err != nil {
		return err
	}
	// Destination cluster pods
	err = t.addMigrationPodsForCluster(t.cluster.destination, migration, parent)
	if err != nil {
		return err
	}
	return nil
}

// Add direct volume pods for a specific cluster
func (t *PlanTree) addMigrationPodsForCluster(cluster model.Cluster, migration *model.Migration, parent *TreeNode) error {
	cLabel := t.cLabel(migration.DecodeObject())

	collection := model.Pod{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	list, err := collection.List(t.db, model.ListOptions{Labels: model.Labels{
		cLabel.Name: cLabel.Value,
	}})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		if v, found := object.Labels[cLabel.Name]; found {
			if v != cLabel.Value {
				continue
			}
		}
		parent.Children = append(
			parent.Children,
			TreeNode{
				Kind:       migref.ToKind(m),
				ObjectLink: PodHandler{}.Link(&cluster, m),
				Namespace:  m.Namespace,
				Name:       m.Name,
			})
	}
	return nil
}

//
// Add direct volumes
func (t *PlanTree) addDirectVolumes(migration *model.Migration, parent *TreeNode) error {
	collection := model.DirectVolumeMigration{}
	cLabel := t.cLabel(migration.DecodeObject())
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		if object.Labels == nil {
			continue
		}
		if v, found := object.Labels[cLabel.Name]; found {
			if v != cLabel.Value {
				continue
			}
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: DirectVolumeMigrationHandler{}.Link(m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}
		err := t.addDirectVolumeProgresses(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		err = t.addDirectVolumePods(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		err = t.addDirectVolumeRoutes(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		parent.Children = append(parent.Children, node)
	}

	return nil
}

//
// Add direct images
func (t *PlanTree) addDirectImages(migration *model.Migration, parent *TreeNode) error {
	collection := model.DirectImageMigration{}
	cLabel := t.cLabel(migration.DecodeObject())
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		if object.Labels == nil {
			continue
		}
		if v, found := object.Labels[cLabel.Name]; found {
			if v != cLabel.Value {
				continue
			}
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: DirectImageMigrationHandler{}.Link(m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}
		err := t.addDirectImageStreams(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		parent.Children = append(parent.Children, node)
	}

	return nil
}

//
// Add direct volumes progress
func (t *PlanTree) addDirectVolumeProgresses(directVolume *model.DirectVolumeMigration, parent *TreeNode) error {
	collection := model.DirectVolumeMigrationProgress{}
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		isOwned := false
		for _, ref := range object.OwnerReferences {
			if ref.Kind == migref.ToKind(directVolume) &&
				ref.Name == directVolume.Name &&
				m.Namespace == directVolume.Namespace {
				isOwned = true
				break
			}
		}
		if !isOwned {
			continue
		}

		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: DirectVolumeMigrationProgressHandler{}.Link(m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}

		// Add linked PVCs
		err := t.addDirectVolumeProgressPVCs(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}

		// Add linked Pods
		err = t.addDirectVolumeProgressPods(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}

		parent.Children = append(parent.Children, node)
	}
	return nil
}

// Add direct volume pods
func (t *PlanTree) addDirectVolumePods(directVolume *model.DirectVolumeMigration, parent *TreeNode) error {
	// Source cluster pods
	err := t.addDirectVolumePodsForCluster(t.cluster.source, directVolume, parent)
	if err != nil {
		return err
	}
	// Destination cluster pods
	err = t.addDirectVolumePodsForCluster(t.cluster.destination, directVolume, parent)
	if err != nil {
		return err
	}
	return nil
}

// Add direct volume pods for a specific cluster
func (t *PlanTree) addDirectVolumePodsForCluster(cluster model.Cluster, directVolume *model.DirectVolumeMigration, parent *TreeNode) error {
	cLabel := t.cLabel(directVolume.DecodeObject())

	collection := model.Pod{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	list, err := collection.List(t.db, model.ListOptions{Labels: model.Labels{
		cLabel.Name: cLabel.Value,
	}})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		if v, found := object.Labels[cLabel.Name]; found {
			if v != cLabel.Value {
				continue
			}
		}
		parent.Children = append(
			parent.Children,
			TreeNode{
				Kind:       migref.ToKind(m),
				ObjectLink: PodHandler{}.Link(&cluster, m),
				Namespace:  m.Namespace,
				Name:       m.Name,
			})
	}
	return nil
}

// Add direct volume progress pods
func (t *PlanTree) addDirectVolumeProgressPods(directVolumeProgress *model.DirectVolumeMigrationProgress, parent *TreeNode) error {
	// Source cluster pods
	err := t.addDirectVolumeProgressPodsForCluster(t.cluster.source, directVolumeProgress, parent)
	if err != nil {
		return err
	}
	// Destination cluster pods
	err = t.addDirectVolumeProgressPodsForCluster(t.cluster.destination, directVolumeProgress, parent)
	if err != nil {
		return err
	}
	return nil
}

// Add direct volume progress pods
func (t *PlanTree) addDirectVolumeProgressPVCs(directVolumeProgress *model.DirectVolumeMigrationProgress, parent *TreeNode) error {
	// Source cluster PVC
	err := t.addDirectVolumeProgressPVCForCluster(t.cluster.source, directVolumeProgress, parent)
	if err != nil {
		return err
	}
	// Destination cluster PVC
	err = t.addDirectVolumeProgressPVCForCluster(t.cluster.destination, directVolumeProgress, parent)
	if err != nil {
		return err
	}
	return nil
}

// Add direct volume pods for a specific cluster
func (t *PlanTree) addDirectVolumeProgressPodsForCluster(cluster model.Cluster,
	directVolumeProgress *model.DirectVolumeMigrationProgress, parent *TreeNode) error {

	podSelector := directVolumeProgress.DecodeObject().Spec.PodSelector
	if podSelector == nil {
		return nil
	}
	collection := model.Pod{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	list, err := collection.List(t.db, model.ListOptions{Labels: podSelector})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		parent.Children = append(
			parent.Children,
			TreeNode{
				Kind:       migref.ToKind(m),
				ObjectLink: PodHandler{}.Link(&cluster, m),
				Namespace:  m.Namespace,
				Name:       m.Name,
			})
	}
	return nil
}

// Add direct volume PVC for a specific cluster
func (t *PlanTree) addDirectVolumeProgressPVCForCluster(cluster model.Cluster,
	directVolumeProgress *model.DirectVolumeMigrationProgress, parent *TreeNode) error {

	dvmpObject := directVolumeProgress.DecodeObject()
	podSelector := dvmpObject.Spec.PodSelector
	if podSelector == nil {
		return nil
	}

	pvcNamespace := dvmpObject.Spec.PodNamespace
	pvcName, ok := podSelector["migration.openshift.io/created-for-pvc"]
	if !ok {
		return nil
	}

	collection := model.PVC{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		if object.Name != pvcName || object.Namespace != pvcNamespace {
			continue
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: PvcHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}

		err := t.addPVForPVC(cluster, m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}

		parent.Children = append(parent.Children, node)
	}
	return nil
}

// Add direct volume routes
func (t *PlanTree) addDirectVolumeRoutes(directVolume *model.DirectVolumeMigration, parent *TreeNode) error {
	// Source
	err := t.addDirectVolumeRoutesForCluster(t.cluster.source, directVolume, parent)
	if err != nil {
		return err
	}
	// Destination
	err = t.addDirectVolumeRoutesForCluster(t.cluster.destination, directVolume, parent)
	if err != nil {
		return err
	}
	return nil
}

// Add direct volume routes for a specific cluster
func (t *PlanTree) addDirectVolumeRoutesForCluster(cluster model.Cluster, directVolume *model.DirectVolumeMigration, parent *TreeNode) error {
	cLabel := t.cLabel(directVolume.DecodeObject())

	collection := model.Route{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	list, err := collection.List(t.db, model.ListOptions{Labels: model.Labels{
		cLabel.Name: cLabel.Value,
	}})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		if v, found := object.Labels[cLabel.Name]; found {
			if v != cLabel.Value {
				continue
			}
		}
		parent.Children = append(
			parent.Children,
			TreeNode{
				Kind:       migref.ToKind(m),
				ObjectLink: RouteHandler{}.Link(&cluster, m),
				Namespace:  m.Namespace,
				Name:       m.Name,
			})
	}
	return nil
}

//
// Add direct images
func (t *PlanTree) addDirectImageStreams(directImage *model.DirectImageMigration, parent *TreeNode) error {
	collection := model.DirectImageStreamMigration{}
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		isOwned := false
		for _, ref := range object.OwnerReferences {
			if ref.Kind == migref.ToKind(directImage) &&
				ref.Name == directImage.Name &&
				m.Namespace == directImage.Namespace {
				isOwned = true
				break
			}
		}
		if !isOwned {
			continue
		}
		parent.Children = append(
			parent.Children,
			TreeNode{
				Kind:       migref.ToKind(m),
				ObjectLink: DirectImageStreamMigrationHandler{}.Link(m),
				Namespace:  m.Namespace,
				Name:       m.Name,
			})
	}
	return nil
}

//
// Add related velero PodVolumeBackups.
func (t *PlanTree) addPvBackups(backup *model.Backup, parent *TreeNode) error {
	cluster := t.cluster.source
	collection := model.PodVolumeBackup{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		isOwned := false
		for _, ref := range object.OwnerReferences {
			if ref.Kind == migref.ToKind(backup) &&
				ref.Name == backup.Name &&
				m.Namespace == backup.Namespace {
				isOwned = true
				break
			}
		}
		if !isOwned {
			continue
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: PvBackupHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}

		err = t.addPVCsForPVB(m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		parent.Children = append(parent.Children, node)
	}

	return nil
}

//
// Add related velero PodVolumeRestores.
func (t *PlanTree) addPvRestores(restore *model.Restore, parent *TreeNode) error {
	cluster := t.cluster.destination
	collection := model.PodVolumeRestore{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		isOwned := false
		for _, ref := range object.OwnerReferences {
			if ref.Kind == migref.ToKind(restore) &&
				ref.Name == restore.Name &&
				m.Namespace == restore.Namespace {
				isOwned = true
				break
			}
		}
		if !isOwned {
			continue
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: PvRestoreHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}

		err = t.addPVCsForPVR(restore, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}
		parent.Children = append(parent.Children, node)
	}

	return nil
}

//
// Add PVCs related to velero restore.
func (t *PlanTree) addPVCsForPVR(restore *model.Restore, parent *TreeNode) error {
	cluster := t.cluster.destination
	collection := model.PVC{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	// list, err := collection.List(t.db, model.ListOptions{Labels: model.Labels{
	// 	"velero.io/restore-name": restore.Name,
	// }})
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range list {
		object := m.DecodeObject()
		restoreName, ok := object.Labels["velero.io/restore-name"]
		if !ok {
			continue
		}
		if restoreName != restore.Name {
			continue
		}

		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: PvcHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}
		err = t.addPVForPVC(cluster, m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}

		parent.Children = append(parent.Children, node)
	}

	return nil
}

//
// Add PVCs moved by a velero backup.
func (t *PlanTree) addPVCsForBackup(backup *model.Backup, parent *TreeNode) error {
	cluster := t.cluster.source
	collection := model.PVC{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	// list, err := collection.List(t.db, model.ListOptions{Labels: model.Labels{
	// 	"velero.io/restore-name": restore.Name,
	// }})
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	expectedBackupName := backup.DecodeObject().Name
	for _, m := range list {
		object := m.DecodeObject()
		backupName, exists := object.Labels["migration.openshift.io/migrated-by-backup"]
		if !exists {
			continue
		}
		if backupName != expectedBackupName {
			continue
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: PvcHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}

		err = t.addPVForPVC(cluster, m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}

		parent.Children = append(parent.Children, node)
	}

	return nil
}

//
// Add PVCs moved by a velero restore.
func (t *PlanTree) addPVCsForRestore(restore *model.Restore, parent *TreeNode) error {
	cluster := t.cluster.destination
	collection := model.PVC{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	// list, err := collection.List(t.db, model.ListOptions{Labels: model.Labels{
	// 	"velero.io/restore-name": restore.Name,
	// }})
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	expectedBackupName := restore.DecodeObject().Spec.BackupName
	for _, m := range list {
		object := m.DecodeObject()
		backupName, exists := object.Labels["migration.openshift.io/migrated-by-backup"]
		if !exists {
			continue
		}
		if backupName != expectedBackupName {
			continue
		}
		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: PvcHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}

		err = t.addPVForPVC(cluster, m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}

		parent.Children = append(parent.Children, node)
	}

	return nil
}

//
// Add PVCs related to velero restore.
func (t *PlanTree) addPVCsForPVB(pvb *model.PodVolumeBackup, parent *TreeNode) error {
	cluster := t.cluster.source
	collection := model.PVC{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	// list, err := collection.List(t.db, model.ListOptions{Labels: model.Labels{
	// 	"velero.io/restore-name": restore.Name,
	// }})
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}
	pvcUID, ok := pvb.DecodeObject().Labels["velero.io/pvc-uid"]
	if !ok {
		return nil
	}
	for _, m := range list {
		object := m.DecodeObject()
		if string(object.UID) != pvcUID {
			continue
		}

		node := TreeNode{
			Kind:       migref.ToKind(m),
			ObjectLink: PvcHandler{}.Link(&cluster, m),
			Namespace:  m.Namespace,
			Name:       m.Name,
		}

		err = t.addPVForPVC(cluster, m, &node)
		if err != nil {
			Log.Trace(err)
			return err
		}

		parent.Children = append(parent.Children, node)
	}

	return nil
}

// Add PV bound to PVC.
func (t *PlanTree) addPVForPVC(cluster model.Cluster, pvc *model.PVC, parent *TreeNode) error {
	collection := model.PV{
		Base: model.Base{
			Cluster: cluster.PK,
		},
	}
	// list, err := collection.List(t.db, model.ListOptions{Labels: model.Labels{
	// 	"velero.io/restore-name": restore.Name,
	// }})
	list, err := collection.List(t.db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return err
	}

	pvcName := pvc.DecodeObject().Spec.VolumeName

	for _, m := range list {
		object := m.DecodeObject()
		if object.Name != pvcName {
			continue
		}
		parent.Children = append(
			parent.Children,
			TreeNode{
				Kind:       migref.ToKind(m),
				ObjectLink: PvHandler{}.Link(&cluster, m),
				// Namespace:  m.Namespace,
				Name: m.Name,
			})
	}

	return nil
}

//
// Update the model `with` a PlanHandler
// Fetch and build the pod lists.
func (p *PlanPods) With(ctx *gin.Context, h *PlanHandler) error {
	var err error
	p.Namespace = h.plan.Namespace
	p.Name = h.plan.Name
	p.Controller = []PlanPod{}
	p.Source = []PlanPod{}
	p.Destination = []PlanPod{}
	p.Controller, err = p.buildController(ctx, h)
	object := h.plan.DecodeObject()
	if err != nil {
		Log.Trace(err)
		return err
	}
	p.Source, err = p.buildPods(h, object.Spec.SrcMigClusterRef)
	if err != nil {
		Log.Trace(err)
		return err
	}
	p.Destination, err = p.buildPods(h, object.Spec.DestMigClusterRef)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

//
// Build the controller pods list.
// Finds pods by label.
func (p *PlanPods) buildController(ctx *gin.Context, h *PlanHandler) ([]PlanPod, error) {
	pods := []PlanPod{}
	list, err := model.Pod{
		Base: model.Base{
			Namespace: h.plan.Namespace,
		},
	}.List(
		h.container.Db,
		model.ListOptions{
			Labels: model.Labels{
				"control-plane": "controller-manager",
				"app":           "migration",
			},
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return nil, err
	}
	if len(list) == 0 {
		return pods, nil
	}
	m := list[0]
	cluster, err := m.GetCluster(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			return nil, err
		} else {
			return pods, nil
		}
	}
	pod := PlanPod{}
	filter := func(c *v1.Container) bool { return c.Name == "mtc" }
	pod.With(m, cluster, filter)
	pods = append(pods, pod)

	return pods, nil
}

//
// Build list of relevant pods on a cluster.
// Finds pods by label. Currently includes velero and restic pods.
func (p *PlanPods) buildPods(h *PlanHandler, ref *v1.ObjectReference) ([]PlanPod, error) {
	pods := []PlanPod{}
	cluster := model.Cluster{
		CR: model.CR{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
	}
	err := cluster.Get(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			return nil, err
		} else {
			return nil, nil
		}
	}
	labels := []model.Labels{
		{"component": "velero"},
		{"name": "restic"},
	}
	for _, lb := range labels {
		podModels, err := model.Pod{
			Base: model.Base{
				Cluster:   cluster.PK,
				Namespace: cluster.Namespace,
			},
		}.List(
			h.container.Db,
			model.ListOptions{
				Labels: lb,
			})
		if err != nil {
			Log.Trace(err)
			return nil, err
		}
		for _, model := range podModels {
			pod := PlanPod{}
			pod.With(model, &cluster)
			pods = append(pods, pod)
		}
	}

	return pods, nil
}

//
// Plan REST resource.
type Plan struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *migapi.MigPlan `json:"object,omitempty"`
}

//
// Build the resource.
func (r *Plan) With(m *model.Plan) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Plan collection REST resource.
type PlanList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Plan `json:"resources"`
}

//
// PlanPods REST resource.
type PlanPods struct {
	// The k8s namespace.
	Namespace string `json:"namespace"`
	// The k8s name.
	Name string `json:"name"`
	// List of controller pods.
	Controller []PlanPod `json:"controller"`
	// List of relevant pods on source cluster.
	Source []PlanPod `json:"source"`
	// List of relevant pods on the destination cluster.
	Destination []PlanPod `json:"destination"`
}

//
// Container REST resource.
type Container struct {
	// Pod k8s name.
	Name string `json:"name"`
	// The URI used to obtain logs.
	Log string `json:"log"`
}

//
// PlanPod REST resource.
type PlanPod struct {
	// Pod k8s namespace.
	Namespace string `json:"namespace"`
	// Pod k8s name.
	Name string `json:"name"`
	// List of containers.
	Containers []Container `json:"containers"`
}

//
// Container filter.
type ContainerFilter func(*v1.Container) bool

//
// Update fields using the specified models.
func (p *PlanPod) With(pod *model.Pod, cluster *model.Cluster, filters ...ContainerFilter) {
	p.Containers = []Container{}
	p.Namespace = pod.Namespace
	p.Name = pod.Name
	path := LogRoot
	path = strings.Replace(path, ":ns1", cluster.Namespace, 1)
	path = strings.Replace(path, ":cluster", cluster.Name, 1)
	path = strings.Replace(path, ":ns2", p.Namespace, 1)
	path = strings.Replace(path, ":pod", p.Name, 1)
	for _, container := range p.filterContainers(pod, filters) {
		lp := fmt.Sprintf("%s?container=%s", path, container.Name)
		p.Containers = append(
			p.Containers, Container{
				Name: container.Name,
				Log:  lp,
			})
	}
}

//
// Get a filtered list of containers.
func (p *PlanPod) filterContainers(pod *model.Pod, filters []ContainerFilter) []v1.Container {
	list := []v1.Container{}
	v1pod := pod.DecodeObject()
	podContainers := v1pod.Spec.Containers
	if podContainers == nil {
		return list
	}
	for _, container := range podContainers {
		excluded := false
		for _, filter := range filters {
			if !filter(&container) {
				excluded = true
				break
			}
		}
		if !excluded {
			list = append(list, container)
		}
	}

	return list
}

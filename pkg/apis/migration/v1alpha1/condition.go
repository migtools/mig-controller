package v1alpha1

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// Types
const (
	ReconcileFailed = "ReconcileFailed"
	Ready           = "Ready"
	Running         = "Running"
	Failed          = "Failed"
	Succeeded       = "Succeeded"
)

// Status
const (
	True  = "True"
	False = "False"
)

// Reasons
const (
	NotFound    = "NotFound"
	NotSet      = "NotSet"
	NotDistinct = "NotDistinct"
	NotReady    = "NotReady"
)

// Category
// Critical - Errors that block Reconcile() and the `Ready` condition.
// Error - Errors that block the `Ready` condition.
// Warn - Warnings that do not block the `Ready` condition.
// Required - Required for the `Ready` condition.
// Advisory - An advisory condition.
const (
	Critical = "Critical"
	Error    = "Error"
	Warn     = "Warn"
	Required = "Required"
	Advisory = "Advisory"
)

// Condition
// Type - The condition type.
// Status - The condition status.
// Reason - The reason for the condition.
// Message - The human readable description of the condition.
// Durable - The condition is not un-staged.
// Items - A list of `items` associated with the condition used to replace [] in `Message`.
// staging - A condition has been explicitly set/updated.
type Condition struct {
	Type               string      `json:"type"`
	Status             string      `json:"status"`
	Reason             string      `json:"reason,omitempty"`
	Category           string      `json:"category"`
	Message            string      `json:"message,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	Durable            bool        `json:"durable,omitempty"`
	Items              []string    `json:"-"`
	staged             bool        `json:"-"`
}

// Update this condition with another's fields.
func (r *Condition) Update(other Condition) {
	r.staged = true
	if r.Equal(other) {
		return
	}
	r.Type = other.Type
	r.Status = other.Status
	r.Reason = other.Reason
	r.Category = other.Category
	r.Message = other.Message
	r.Durable = other.Durable
	r.Items = other.Items
	r.LastTransitionTime = metav1.NewTime(time.Now())
}

// Get whether the conditions are equal.
func (r *Condition) Equal(other Condition) bool {
	return r.Type == other.Type &&
		r.Status == other.Status &&
		r.Category == other.Category &&
		r.Reason == other.Reason &&
		r.Message == other.Message &&
		r.Durable == other.Durable &&
		reflect.DeepEqual(r.Items, other.Items)
}

// Replace [] in `Message` with the content of `Items`.
func (r *Condition) ExpandItems() {
	re := regexp.MustCompile(`\[\]`)
	list := fmt.Sprintf("[%s]", strings.Join(r.Items, ","))
	r.Message = re.ReplaceAllString(r.Message, list)
}

// Build the `Items` list by parsing the `Message`.
func (r *Condition) BuildItems() {
	re := regexp.MustCompile(`\[[^]]+\]`)
	found := re.FindString(r.Message)
	if found == "" {
		return
	}
	r.Items = []string{}
	found = strings.Trim(found, "[]")
	r.Message = re.ReplaceAllString(r.Message, "[]")
	for _, s := range strings.Split(found, ",") {
		r.Items = append(r.Items, strings.TrimSpace(s))
	}
}

// Managed collection of conditions.
// Intended to be included in resource Status.
// List - The list of conditions.
// staging - In `staging` mode, the search methods like
//
//	HasCondition() filter out un-staging conditions.
//
// -------------------
// Example:
//
// thing.Status.BeginStagingConditions()
// thing.Status.SetCondition(c)
// thing.Status.SetCondition(c)
// thing.Status.SetCondition(c)
// thing.Status.EndStagingConditions()
// thing.Status.SetReady(
//
//	!thing.Status.HasBlockerCondition(),
//	"Resource Ready.")
type Conditions struct {
	List    []Condition `json:"conditions,omitempty"`
	staging bool        `json:"-"`
}

// Begin staging conditions.
func (r *Conditions) BeginStagingConditions() {
	r.staging = true
	if r.List == nil {
		return
	}
	for index := range r.List {
		condition := &r.List[index]
		condition.BuildItems()
		condition.staged = condition.Durable
	}
}

// End staging conditions. Un-staged conditions are deleted.
func (r *Conditions) EndStagingConditions() {
	r.staging = false
	if r.List == nil {
		return
	}
	kept := []Condition{}
	for index := range r.List {
		condition := r.List[index]
		if condition.staged {
			condition.ExpandItems()
			kept = append(kept, condition)
		}
	}
	r.List = kept
}

// Find a condition by type.
// Staging is ignored.
func (r *Conditions) find(cndType string) *Condition {
	if r.List == nil {
		return nil
	}
	for i := range r.List {
		condition := &r.List[i]
		if condition.Type == cndType {
			return condition
		}
	}
	return nil
}

// Find a condition by type.
func (r *Conditions) FindCondition(cndType string) *Condition {
	if r.List == nil {
		return nil
	}
	condition := r.find(cndType)
	if condition == nil {
		return nil
	}
	if r.staging && !condition.staged {
		return nil
	}

	return condition
}

func (r *Conditions) FindConditionByCategory(cndCategory string) []*Condition {
	cndList := []*Condition{}
	if r.List == nil {
		return cndList
	}
	for _, c := range r.List {
		if c.Category == cndCategory {
			cndList = append(cndList, &c)
		}
	}
	return cndList
}

// Set (add/update) the specified condition to the collection.
func (r *Conditions) SetCondition(condition Condition) {
	if r.List == nil {
		r.List = []Condition{}
	}
	condition.staged = true
	found := r.find(condition.Type)
	if found == nil {
		condition.LastTransitionTime = metav1.NewTime(time.Now().UTC())
		r.List = append(r.List, condition)
	} else {
		found.Update(condition)
	}
}

// Stage an existing condition by type.
func (r *Conditions) StageCondition(types ...string) {
	if r.List == nil {
		return
	}
	filter := make(map[string]bool)
	for _, t := range types {
		filter[t] = true
	}
	for i := range r.List {
		condition := &r.List[i]
		if _, found := filter[condition.Type]; found {
			condition.staged = true
		}
	}
}

// Delete conditions by type.
func (r *Conditions) DeleteCondition(types ...string) {
	if r.List == nil {
		return
	}
	filter := make(map[string]bool)
	for _, t := range types {
		filter[t] = true
	}
	kept := []Condition{}
	for i := range r.List {
		condition := r.List[i]
		_, matched := filter[condition.Type]
		if !matched {
			kept = append(kept, condition)
			continue
		}
		if r.staging {
			condition.staged = false
			kept = append(kept, condition)
		}
	}
	r.List = kept
}

// The collection has ALL of the specified conditions.
func (r *Conditions) HasCondition(types ...string) bool {
	if r.List == nil {
		return false
	}
	for _, cndType := range types {
		condition := r.FindCondition(cndType)
		if condition == nil || condition.Status != True {
			return false
		}
	}

	return len(types) > 0
}

// The collection has Any of the specified conditions.
func (r *Conditions) HasAnyCondition(types ...string) bool {
	if r.List == nil {
		return false
	}
	for _, cndType := range types {
		condition := r.FindCondition(cndType)
		if condition == nil || condition.Status != True {
			continue
		}
		return true
	}

	return false
}

// The collection contains any conditions with category.
func (r *Conditions) HasConditionCategory(names ...string) bool {
	if r.List == nil {
		return false
	}
	catSet := map[string]bool{}
	for _, name := range names {
		catSet[name] = true
	}
	for _, condition := range r.List {
		_, found := catSet[condition.Category]
		if !found || condition.Status != True {
			continue
		}
		if r.staging && !condition.staged {
			continue
		}
		return true
	}

	return false
}

// The collection contains a `Critical` error condition.
// Resource reconcile() should not continue.
func (r *Conditions) HasCriticalCondition(category ...string) bool {
	return r.HasConditionCategory(Critical)
}

// The collection contains an `Error` condition.
func (r *Conditions) HasErrorCondition(category ...string) bool {
	return r.HasConditionCategory(Error)
}

// The collection contains a `Warn` condition.
func (r *Conditions) HasWarnCondition(category ...string) bool {
	return r.HasConditionCategory(Warn)
}

// The collection contains a `Ready` blocker condition.
func (r *Conditions) HasBlockerCondition() bool {
	return r.HasConditionCategory(Critical, Error)
}

// Set `Ready` condition.
func (r *Conditions) SetReady(ready bool, message string) {
	if ready {
		r.SetCondition(Condition{
			Type:     Ready,
			Status:   True,
			Category: Required,
			Message:  message,
		})
	} else {
		r.DeleteCondition(Ready)
	}
}

// The collection contains the `Ready` condition.
func (r *Conditions) IsReady() bool {
	condition := r.FindCondition(Ready)
	if condition == nil || condition.Status != True {
		return false
	}
	return true
}

// Set the `ReconcileFailed` condition.
// Clear the `Ready` condition.
// Ends staging.
func (r *Conditions) SetReconcileFailed(err error) {
	r.DeleteCondition(Ready)
	r.SetCondition(Condition{
		Type:     ReconcileFailed,
		Status:   True,
		Category: Critical,
		Message:  "Reconcile failed: []. See controller logs for details.",
		Items:    []string{err.Error()},
	})
	r.EndStagingConditions()
}

func (r *Conditions) RecordEvents(obj runtime.Object, recorder record.EventRecorder) {
	for _, cond := range r.List {
		eventType := ""
		switch cond.Category {
		case Critical, Warn, Error:
			eventType = kapi.EventTypeWarning
		default:
			eventType = kapi.EventTypeNormal
		}
		recorder.Event(obj, eventType, cond.Type, cond.Message)
	}
}

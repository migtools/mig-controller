package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Types
const (
	Ready = "Ready"
)

// Status
const (
	True  = "True"
	False = "False"
)

// Category
// Critical - Errors that block Reconcile() and the `Ready` condition.
// Error - Errors that block the `Ready` condition.
// Warn - Warnings that do not block the `Ready` condition.
// Required - Required for the `Ready` condition.
const (
	Critical = "Critical"
	Error    = "Error"
	Warn     = "Warn"
	Required = "Required"
)

// Condition
// Type - The condition type.
// Status - The condition status.
// Reason - The reason for the condition.
// Message - The human readable description of the condition.
// staging - A condition has been explicitly set/updated.
type Condition struct {
	Type               string      `json:"type"`
	Status             string      `json:"status"`
	Reason             string      `json:"reason,omitempty"`
	Category           string      `json:"category"`
	Message            string      `json:"message,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	staged             bool
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
	r.LastTransitionTime = metav1.NewTime(time.Now())
}

func (r Condition) Equal(other Condition) bool {
	return r.Type == other.Type &&
		r.Status == other.Status &&
		r.Category == other.Category &&
		r.Reason == other.Reason &&
		r.Message == other.Message
}

// Managed collection of conditions.
// Intended to be included in resource Status.
// List - The list of conditions.
// staging - In `staging` mode, the search methods like
//          HasCondition() filter out un-staging conditions.
// -------------------
// Example:
//
// thing.Status.BeginStagingConditions()
// thing.Status.SetCondition(c)
// thing.Status.SetCondition(c)
// thing.Status.SetCondition(c)
// thing.Status.EndStagingConditions()
// thing.Status.SetReady(
//     !thing.Status.HasBlockerCondition(),
//     "Resource Ready.")
//
type Conditions struct {
	List    []Condition `json:"conditions"`
	staging bool
}

// Begin staging conditions.
func (r *Conditions) BeginStagingConditions() {
	r.staging = true
	if r.List == nil {
		return
	}
	for index := range r.List {
		condition := &r.List[index]
		condition.staged = false
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
			kept = append(kept, condition)
		}
	}
	r.List = kept
}

// Find a condition by type.
func (r *Conditions) FindCondition(cndType string) *Condition {
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

// Set (add/update) the specified condition to the collection.
func (r *Conditions) SetCondition(condition Condition) {
	if r.List == nil {
		r.List = []Condition{}
	}
	condition.staged = true
	found := r.FindCondition(condition.Type)
	if found == nil {
		condition.LastTransitionTime = metav1.NewTime(time.Now())
		r.List = append(r.List, condition)
	} else {
		found.Update(condition)
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
		_, found := filter[condition.Type]
		if !found {
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
		if r.staging && !condition.staged {
			return false
		}
	}

	return len(types) > 0
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
	if r.staging && !condition.staged {
		return false
	}
	return true
}

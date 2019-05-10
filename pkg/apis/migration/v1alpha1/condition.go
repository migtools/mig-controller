package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
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

// Condition
// Type - The condition type.
// Status - The condition status.
// Reason - The reason for the condition.
// Message - The human readable description of the condition.
// staged - The condition is staged for commit.
type Condition struct {
	Type               string      `json:"type"`
	Status             string      `json:"status"`
	Reason             string      `json:"reason,omitempty"`
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
	r.Message = other.Message
	r.LastTransitionTime = metav1.NewTime(time.Now())
}

func (r Condition) Equal(other Condition) bool {
	return r.Type == other.Type &&
		r.Status == other.Status &&
		r.Reason == other.Reason &&
		r.Message == other.Message
}

// Managed collection of conditions.
// Intended to be included in resource Status.
type Conditions struct {
	Conditions []Condition `json:"conditions"`
}

// Find a condition by type.
func (r *Conditions) FindCondition(cndType string) (int, *Condition) {
	if r.Conditions == nil {
		return 0, nil
	}
	for index := range r.Conditions {
		condition := &r.Conditions[index]
		if condition.Type == cndType {
			return index, condition
		}
	}
	return 0, nil
}

// Set (add/update) the specified condition to the collection.
func (r *Conditions) SetCondition(condition Condition) {
	if r.Conditions == nil {
		r.Conditions = []Condition{}
	}
	condition.staged = true
	_, found := r.FindCondition(condition.Type)
	if found == nil {
		condition.LastTransitionTime = metav1.NewTime(time.Now())
		r.Conditions = append(r.Conditions, condition)
	} else {
		found.Update(condition)
	}
}

// Delete conditions by type.
func (r *Conditions) DeleteCondition(cndTypes ...string) {
	if r.Conditions == nil {
		return
	}
	kept := []Condition{}
	for _, name := range cndTypes {
		for index := range r.Conditions {
			condition := r.Conditions[index]
			if condition.Type != name {
				kept = append(kept, condition)
			}
		}
	}
	r.Conditions = kept
}

// Un-stage all conditions.
func (r *Conditions) UnstageConditions() {
	if r.Conditions == nil {
		return
	}
	for index := range r.Conditions {
		condition := &r.Conditions[index]
		condition.staged = false
	}
}

// Commit staged conditions. Un-staged conditions are deleted.
func (r *Conditions) CommitConditions() {
	if r.Conditions == nil {
		return
	}
	kept := []Condition{}
	for index := range r.Conditions {
		condition := r.Conditions[index]
		if condition.staged {
			kept = append(kept, condition)
		}
	}
	r.Conditions = kept
}

// Set `Ready` condition.
func (r *Conditions) SetReady(ready bool, message string) {
	if ready {
		r.SetCondition(Condition{
			Type:    Ready,
			Status:  True,
			Message: message,
		})
	} else {
		r.DeleteCondition(Ready)
	}
}

// Get if `Ready` condition is `True`.
func (r *Conditions) IsReady() bool {
	_, condition := r.FindCondition(Ready)
	return condition != nil && condition.Status == True
}

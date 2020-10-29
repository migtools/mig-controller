package v1alpha1

import meta "k8s.io/apimachinery/pkg/apis/meta/v1"

// Timed records started and completed timestamps.
type Timed struct {
	// Started timestamp.
	Started *meta.Time `json:"started,omitempty"`
	// Completed timestamp.
	Completed *meta.Time `json:"completed,omitempty"`
}

// MarkReset mark as reset.
func (r *Timed) MarkReset() {
	r.Started = nil
	r.Completed = nil
}

// MarkStarted mark as started.
func (r *Timed) MarkStarted() {
	if r.Started == nil {
		r.Started = r.now()
		r.Completed = nil
	}
}

// MarkCompleted mark as completed.
func (r *Timed) MarkCompleted() {
	r.MarkStarted()
	if r.Completed == nil {
		r.Completed = r.now()
	}
}

// MarkedStarted check if started.
func (r *Timed) MarkedStarted() bool {
	return r.Started != nil
}

// Running check if running
func (r *Timed) Running() bool {
	return r.MarkedStarted() && !r.MarkedCompleted()
}

// MarkedCompleted check if completed.
func (r *Timed) MarkedCompleted() bool {
	return r.Completed != nil
}

func (r *Timed) now() *meta.Time {
	now := meta.Now()
	return &now
}

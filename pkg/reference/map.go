package reference

import (
	kapi "k8s.io/api/core/v1"
	"sync"
)

// singleton
var instance *RefMap
var create sync.Once

// A resource that contains an ObjectReference.
type RefOwner kapi.ObjectReference

// The resource that is the target of an ObjectReference.
type RefTarget kapi.ObjectReference

// A 1-n mapping of RefTarget => [RefOwner, ...].
type RefMap struct {
	Content map[RefTarget][]RefOwner
	mutex   sync.RWMutex
}

// Get the singleton.
func GetMap() *RefMap {
	create.Do(func() {
		instance = &RefMap{
			Content: map[RefTarget][]RefOwner{},
		}
	})
	return instance
}

// Add mapping of a ref-owner to a ref-target.
func (r *RefMap) Add(refOwner RefOwner, refTarget RefTarget) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	list, found := r.Content[refTarget]
	if !found {
		list = []RefOwner{}
		r.Content[refTarget] = list
	}
	for i := range list {
		if list[i] == refOwner {
			return
		}
	}
	list = append(list, refOwner)
	r.Content[refTarget] = list
}

// Delete mapping of a ref-owner to a ref-target.
func (r *RefMap) Delete(refOwner RefOwner, refTarget RefTarget) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	list, found := r.Content[refTarget]
	if !found {
		return
	}
	newList := []RefOwner{}
	for i := range list {
		if list[i] == refOwner {
			continue
		}
		newList = append(newList, list[i])
	}
	if len(newList) < len(list) {
		r.Content[refTarget] = newList
	}
}

// Find all refOwners for a refTarget.
func (r *RefMap) Find(refTarget RefTarget, refOwner RefOwner) []RefOwner {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	list, found := r.Content[refTarget]
	if !found {
		return []RefOwner{}
	}
	matched := []RefOwner{}
	for i := range list {
		owner := list[i]
		if owner.Kind == refOwner.Kind {
			matched = append(matched, owner)
		}
	}
	return matched
}

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

// A mapping of reference RefSources to a RefTarget.
type RefMap struct {
	Content map[RefTarget][]RefOwner
	mutex   sync.RWMutex
}

func GetMap() *RefMap {
	create.Do(func() {
		instance = &RefMap{
			Content: map[RefTarget][]RefOwner{},
		}
	})
	return instance
}

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

func (r *RefMap) Delete(refOwner RefOwner, refTarget RefTarget) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	list, found := r.Content[refTarget]
	if !found {
		return
	}
	for i := range list {
		if list[i] == refOwner {
			list = append(list[:i], list[i+1:]...)
			r.Content[refTarget] = list
		}
	}
}

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

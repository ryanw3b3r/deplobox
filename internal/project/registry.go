package project

import (
	"fmt"
	"sync"
)

// Registry manages the collection of loaded projects
type Registry struct {
	mu       sync.RWMutex
	projects map[string]*Project
}

// NewRegistry creates a new project registry
func NewRegistry(projects map[string]*Project) *Registry {
	return &Registry{
		projects: projects,
	}
}

// Get retrieves a project by name
func (r *Registry) Get(name string) (*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	project, exists := r.projects[name]
	if !exists {
		return nil, fmt.Errorf("project '%s' not found", name)
	}

	return project, nil
}

// List returns all project names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.projects))
	for name := range r.projects {
		names = append(names, name)
	}

	return names
}

// Count returns the number of projects
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.projects)
}

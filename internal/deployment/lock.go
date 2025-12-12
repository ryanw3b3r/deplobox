package deployment

import "sync"

// LockManager manages per-project deployment locks to prevent concurrent deployments.
//
// This uses a two-level locking strategy:
// 1. The outer mutex (mu) protects the locks map itself from concurrent access
// 2. Each project has its own mutex for actual deployment locking
//
// This design allows different projects to deploy concurrently while ensuring
// that only one deployment can run for a given project at a time.
type LockManager struct {
	mu    sync.Mutex            // Protects the locks map
	locks map[string]*sync.Mutex // Per-project locks
}

// NewLockManager creates a new lock manager
func NewLockManager() *LockManager {
	return &LockManager{
		locks: make(map[string]*sync.Mutex),
	}
}

// TryLock attempts to acquire a deployment lock for the given project.
//
// Returns true if the lock was successfully acquired (deployment can proceed).
// Returns false if the project is already locked (another deployment is in progress).
//
// This method is non-blocking - it returns immediately whether or not the lock
// was acquired. The caller should check the return value and reject the deployment
// if false is returned.
func (lm *LockManager) TryLock(projectName string) bool {
	// First, acquire the map lock to safely access/create the project lock
	lm.mu.Lock()
	lock, exists := lm.locks[projectName]
	if !exists {
		// Create a new lock for this project on first use
		lock = &sync.Mutex{}
		lm.locks[projectName] = lock
	}
	lm.mu.Unlock()

	// Try to acquire the project-specific lock (non-blocking)
	return lock.TryLock()
}

// Unlock releases the deployment lock for the given project.
//
// This should be called after a deployment completes (success or failure).
// Typically used with defer: defer lockManager.Unlock(projectName)
//
// It is safe to call this even if the lock doesn't exist (no-op).
func (lm *LockManager) Unlock(projectName string) {
	lm.mu.Lock()
	lock := lm.locks[projectName]
	lm.mu.Unlock()

	if lock != nil {
		lock.Unlock()
	}
}

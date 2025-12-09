package deployment

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLockManager_BasicLocking(t *testing.T) {
	lm := NewLockManager()

	// First lock should succeed
	if !lm.TryLock("project1") {
		t.Fatal("First TryLock should succeed")
	}

	// Second lock on same project should fail
	if lm.TryLock("project1") {
		t.Error("Second TryLock on same project should fail")
	}

	// Unlock
	lm.Unlock("project1")

	// Lock should succeed again after unlock
	if !lm.TryLock("project1") {
		t.Error("TryLock should succeed after unlock")
	}

	lm.Unlock("project1")
}

func TestLockManager_MultipleProjects(t *testing.T) {
	lm := NewLockManager()

	// Multiple different projects should be able to lock concurrently
	if !lm.TryLock("project1") {
		t.Error("project1 lock should succeed")
	}

	if !lm.TryLock("project2") {
		t.Error("project2 lock should succeed")
	}

	if !lm.TryLock("project3") {
		t.Error("project3 lock should succeed")
	}

	// But second lock on any project should fail
	if lm.TryLock("project1") {
		t.Error("Second lock on project1 should fail")
	}

	if lm.TryLock("project2") {
		t.Error("Second lock on project2 should fail")
	}

	// Unlock all
	lm.Unlock("project1")
	lm.Unlock("project2")
	lm.Unlock("project3")

	// All should be lockable again
	if !lm.TryLock("project1") {
		t.Error("project1 should be lockable after unlock")
	}
	lm.Unlock("project1")
}

func TestLockManager_UnlockNonExistent(t *testing.T) {
	lm := NewLockManager()

	// Unlocking a non-existent lock should not panic
	lm.Unlock("nonexistent")

	// Should still be able to lock it afterwards
	if !lm.TryLock("nonexistent") {
		t.Error("Should be able to lock after unlocking non-existent")
	}

	lm.Unlock("nonexistent")
}

func TestLockManager_ConcurrentLockAttempts(t *testing.T) {
	lm := NewLockManager()

	projectName := "concurrent-project"
	successCount := int32(0)
	failureCount := int32(0)

	// Launch multiple goroutines trying to lock the same project
	const goroutineCount = 100
	var wg sync.WaitGroup
	wg.Add(goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func() {
			defer wg.Done()

			if lm.TryLock(projectName) {
				atomic.AddInt32(&successCount, 1)
				// Hold lock briefly
				time.Sleep(10 * time.Millisecond)
				lm.Unlock(projectName)
			} else {
				atomic.AddInt32(&failureCount, 1)
			}
		}()
	}

	wg.Wait()

	// Only one goroutine should have succeeded in acquiring the lock at a time
	// But we can't predict exactly how many will succeed vs fail due to timing
	// However, we can verify that at least some failed (since we launched 100 concurrent attempts)
	if failureCount == 0 {
		t.Error("Expected at least some lock attempts to fail due to concurrency")
	}

	if successCount == 0 {
		t.Error("Expected at least one lock attempt to succeed")
	}

	// Total should equal goroutineCount
	if int(successCount+failureCount) != goroutineCount {
		t.Errorf("Success + failure count (%d + %d = %d) should equal goroutine count (%d)",
			successCount, failureCount, successCount+failureCount, goroutineCount)
	}

	t.Logf("Concurrent lock test: %d succeeded, %d failed", successCount, failureCount)
}

func TestLockManager_ConcurrentDifferentProjects(t *testing.T) {
	lm := NewLockManager()

	const projectCount = 50
	const attemptsPerProject = 10

	var wg sync.WaitGroup
	successCounts := make([]int32, projectCount)

	// Launch goroutines for each project
	for i := 0; i < projectCount; i++ {
		projectName := string(rune('A' + i)) // A, B, C, ...

		// Multiple attempts per project
		for j := 0; j < attemptsPerProject; j++ {
			wg.Add(1)
			go func(projectIndex int, project string) {
				defer wg.Done()

				if lm.TryLock(project) {
					atomic.AddInt32(&successCounts[projectIndex], 1)
					time.Sleep(1 * time.Millisecond)
					lm.Unlock(project)
				}
			}(i, projectName)
		}
	}

	wg.Wait()

	// Each project should have had at least one successful lock
	for i, count := range successCounts {
		if count == 0 {
			t.Errorf("Project %c had no successful locks", rune('A'+i))
		}
	}

	t.Logf("Concurrent different projects test completed. Success counts: %v", successCounts)
}

func TestLockManager_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	lm := NewLockManager()

	const (
		iterationsPerGoroutine = 1000
		goroutineCount         = 10
		projectCount           = 5
	)

	var wg sync.WaitGroup

	totalLocks := int64(0)
	totalUnlocks := int64(0)

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			projectName := string(rune('0' + (id % projectCount)))

			for j := 0; j < iterationsPerGoroutine; j++ {
				if lm.TryLock(projectName) {
					atomic.AddInt64(&totalLocks, 1)
					// Simulate some work
					time.Sleep(time.Microsecond)
					lm.Unlock(projectName)
					atomic.AddInt64(&totalUnlocks, 1)
				}
				// Yield to avoid busy loop
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify locks and unlocks match
	if totalLocks != totalUnlocks {
		t.Errorf("Lock count (%d) doesn't match unlock count (%d)", totalLocks, totalUnlocks)
	}

	t.Logf("Stress test completed: %d locks/unlocks", totalLocks)
}

func TestLockManager_DeadlockPrevention(t *testing.T) {
	lm := NewLockManager()

	// This test verifies that the two-level locking strategy prevents deadlocks
	const projectCount = 10
	var wg sync.WaitGroup

	// Create locks for all projects concurrently
	for i := 0; i < projectCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			projectName := string(rune('a' + id))

			for j := 0; j < 100; j++ {
				if lm.TryLock(projectName) {
					lm.Unlock(projectName)
				}
			}
		}(i)
	}

	// Set a timeout to detect potential deadlock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - potential deadlock detected")
	}
}

// Benchmark tests

func BenchmarkLockManager_TryLock(b *testing.B) {
	lm := NewLockManager()
	projectName := "bench-project"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lm.TryLock(projectName)
		lm.Unlock(projectName)
	}
}

func BenchmarkLockManager_ConcurrentLocks(b *testing.B) {
	lm := NewLockManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			projectName := string(rune('0' + (i % 10)))
			if lm.TryLock(projectName) {
				lm.Unlock(projectName)
			}
			i++
		}
	})
}

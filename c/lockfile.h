// C implementation of vstodo Lockfile management
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/lockfile package interface but does not
// implement file locking without actual filesystem operations.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/lockfile/lockfile.go.

#ifndef VSTODO_C_LOCKFILE_H
#define VSTODO_C_LOCKFILE_H

#include <stdbool.h>

// ErrLocked is returned when lock file already exists.
#define VSTODO_LOCKFILE_ERR_LOCKED ((int)-1)

// Lock provides exclusive, crash-safe file replacement using
// Git-style sibling .lock files.
typedef struct {
    const char *path;       // path to the file being locked
    const char *lock_path;  // path to the .lock file
    void *file;             // FILE* or platform-specific handle
    bool committed;
} vstodo_lock;

// Acquire creates the Git-compatible <path>.lock file exclusively.
vstodo_lock *vstodo_lock_acquire(const char *path, bool perm);

// Commit writes data to the lock file and atomically replaces the original.
int vstodo_lock_commit(vstodo_lock *lock, const uint8_t *data, size_t data_len);

// Close releases the lock. If not committed, removes the .lock file.
int vstodo_lock_close(vstodo_lock *lock);

// --- Convenience ---

const char *vstodo_lock_strerror(int err);

#endif // VSTODO_C_LOCKFILE_H
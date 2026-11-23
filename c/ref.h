// C implementation of vstodo Reference management
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/ref package interface but does not
// implement full ref storage without the git directory layout.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/ref/ref.go.

#ifndef VSTODO_C_REF_H
#define VSTODO_C_REF_H

#include "sha1.h"
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

// ErrNotFound is returned when a ref does not exist.
#define VSTODO_REF_ERR_NOT_FOUND ((int)-1)

// ErrReferenceChanged indicates a reference no longer matches the expected value.
#define VSTODO_REF_ERR_CHANGED ((int)-2)

// ErrLockExists indicates another writer owns the reference lock.
#define VSTODO_REF_ERR_LOCK_EXISTS ((int)-3)

// Store persists refs under a gitDir.
typedef struct {
    const char *git_dir;
} vstodo_ref_store;

// NewStore opens the ref store under gitDir.
vstodo_ref_store *vstodo_ref_store_new(const char *git_dir);

// validateRefName rejects path traversal and invalid ref path components.
bool vstodo_ref_validate_name(const char *name);

// refPath returns the path for a reference name.
const char *vstodo_ref_ref_path(vstodo_ref_store *store, const char *ref_name);

// ReadRef resolves a direct or symbolic reference to an object digest.
vstodo_sha1_digest vstodo_ref_read_ref(vstodo_ref_store *store, const char *ref_name);

// acquireLock creates the Git-compatible <ref>.lock file exclusively.
 // Returns: (lock_path, lock, error_code)
 // error_code: 0 = ok, negative = error
int vstodo_ref_acquire_lock(vstodo_ref_store *store, const char *ref_name,
                            const char **lock_path, void **lock);

// UpdateRef atomically updates a direct reference only when its current value
// matches expected. A nil expected value means the reference must not exist.
// Symbolic references are rejected; callers must update the resolved branch ref.
int vstodo_ref_update_ref(vstodo_ref_store *store, const char *ref_name,
                           const vstodo_sha1_digest *expected, vstodo_sha1_digest next);

// WriteRef atomically sets refName to digest (non-symbolic).
int vstodo_ref_write_ref(vstodo_ref_store *store, const char *ref_name, vstodo_sha1_digest d);

// WriteRefSymbolic atomically makes refName point to another ref.
int vstodo_ref_write_ref_symbolic(vstodo_ref_store *store, const char *ref_name, const char *target);

// Head returns the digest HEAD points to, resolving symbolic refs.
vstodo_sha1_digest vstodo_ref_head(vstodo_ref_store *store);

// SetHead sets HEAD to a digest or symbolic target.
int vstodo_ref_set_head(vstodo_ref_store *store, const char *target);

// ListRefs lists all refs under refs/heads and refs/tags.
 // Output: allocated array of {name, id} pairs, *out_count set to count
 // Caller must free with vstodo_free()
void **vstodo_ref_list_refs(vstodo_ref_store *store, size_t *out_count);

// ListAllRefs lists every regular reference under refs/, including remote-tracking
// refs and namespaces such as notes. Lock files and symlinks are not references.
void **vstodo_ref_list_all_refs(vstodo_ref_store *store, size_t *out_count);

// DeleteRef removes refName while holding its lock.
int vstodo_ref_delete_ref(vstodo_ref_store *store, const char *ref_name);

// CurrentBranch returns the short branch name HEAD points to, or "" if detached.
const char *vstodo_ref_current_branch(vstodo_ref_store *store);

// --- Internal data structures (opaque to users of the header) ---

// lock represents a Git-compatible reference lock.
typedef struct {
    const char *path;      // path to the ref file
    const char *lock_path; // path to the .lock file
    void *file;            // FILE* or int fd
    bool committed;
} vstodo_ref_lock;

// --- Convenience: error message strings ---

const char *vstodo_ref_strerror(int err);

#endif // VSTODO_C_REF_H
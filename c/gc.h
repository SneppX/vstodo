// C implementation of vstodo Garbage Collection
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/gc package interface but does not
// implement full garbage collection without the ref store and object store.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/gc/gc.go.

#ifndef VSTODO_C_GC_H
#define VSTODO_C_GC_H

#include "sha1.h"
#include <stdbool.h>

// GC performs a non-destructive dry run. Use GCWithOptions with dryRun=false
// to remove unreachable loose objects.
int vstodo_gc(void *app, bool dry_run);

// GCWithOptions marks all objects reachable from HEAD, branches, tags, and
// stash history. It only prunes unreachable loose objects; it never deletes
// objects that may be stored in packs.
int vstodo_gc_with_options(void *app, bool dry_run);

// --- Mark reachability ---

// markReachable marks all objects reachable from the given reference.
 // Output: map[SHA1]bool of live objects
void *vstodo_gc_mark_reachable(void *app, vstodo_sha1_digest head);

// --- Main ---

int main(void) {
    printf("GC Test Suite (C skeleton)\\n\\n");
    printf("GC skeleton created. Full implementation pending.\\n");
    return 0;
}

#endif // VSTODO_C_GC_H
// C implementation of vstodo Garbage Collection
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/gc package interface but does not
// implement full garbage collection without the ref store and object store.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/gc/gc.go.

#include "gc.h"
#include <stdlib.h>
#include <stdio.h>

// --- vstodo_gc ---

int vstodo_gc(void *app, bool dry_run) {
    // In full implementation, this would:
    // 1. Mark all objects reachable from HEAD, branches, tags, stash history
    // 2. Enumerate all loose objects
    // 3. Identify unreachable objects
    // 4. If dry_run is true, report count and return
    // 5. If dry_run is false, delete unreachable loose objects
    // For now, return error — pending full implementation
    (void)app; (void)dry_run;
    return -1;
}

// --- vstodo_gc_with_options ---

int vstodo_gc_with_options(void *app, bool dry_run) {
    // In full implementation, this would:
    // 1. Same as GC but with more options documented in the Go source
    // For now, return error — pending full implementation
    (void)app; (void)dry_run;
    return -1;
}

// --- Main ---

int main(void) {
    printf("GC Test Suite (C skeleton)\\n\\n");
    printf("GC skeleton created. Full implementation pending.\\n");
    return 0;
}
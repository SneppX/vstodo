// C implementation of vstodo Index (Git-compatible staging area)
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/index package interface but does not
// implement full index persistence without the DIRC file layout.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/index/index.go.

#ifndef VSTODO_C_INDEX_H
#define VSTODO_C_INDEX_H

#include "sha1.h"
#include <stddef.h>
#include <stdlib.h>
#include <stdbool.h>

// Index maps repository-relative paths to staged blob digests.
typedef struct {
    // In a full implementation, this would contain:
    // - entries: map[path]sha1.Digest
    // - modes: map[path]uint32
    // - path: string (index file path)
    // - snapshot: byte slice (last loaded/saved snapshot)
    // - loaded: bool
} vstodo_index;

// NewIndex creates a new Index.
vstodo_index *vstodo_index_new(const char *path);

// Load reads the index from disk.
int vstodo_index_load(vstodo_index *idx);

// Save writes the index to disk.
int vstodo_index_save(vstodo_index *idx);

// Add stages file contents into the index.
int vstodo_index_add(vstodo_index *idx, const char *path, vstodo_sha1_digest id);

// AddMode stages file contents with a mode.
int vstodo_index_add_mode(vstodo_index *idx, const char *path, vstodo_sha1_digest id, uint32_t mode);

// Mode returns the mode for a path.
uint32_t vstodo_index_mode(vstodo_index *idx, const char *path);

// Remove removes a path from the index.
int vstodo_index_remove(vstodo_index *idx, const char *path);

// Get returns the digest for a path.
vstodo_sha1_digest vstodo_index_get(vstodo_index *idx, const char *path);

// Entries returns the index entries map.
// Output: allocated array of {path, id} pairs, *out_count set to count
 // Caller must free with vstodo_free()
void **vstodo_index_entries(vstodo_index *idx, size_t *out_count);

// Contains reports whether the given path is in the index.
bool vstodo_index_contains(vstodo_index *idx, const char *path);

// Entries count
size_t vstodo_index_count(vstodo_index *idx);

// Index signature constant
#define VSTODO_INDEX_SIGNATURE "DIRC"

// Index version constant
#define VSTODO_INDEX_VERSION 2

#endif // VSTODO_C_INDEX_H
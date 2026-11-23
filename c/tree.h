// C implementation of vstodo Tree types and parsing
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/tree package interface but does not
// implement full tree persistence or parsing without the object store.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/tree/tree.go.

#ifndef VSTODO_C_TREE_H
#define VSTODO_C_TREE_H

#include "object.h"
#include "sha1.h"
#include <stddef.h>
#include <stdlib.h>
#include <string.h>

// Entry is a tree entry.
typedef struct {
    const char *mode;   // "100644", "100755", "040000", "120000"
    const char *name;
    vstodo_sha1_digest id;
} vstodo_tree_entry;

// Tree is a decoded tree object.
typedef struct {
    vstodo_tree_entry *entries;
    size_t num_entries;
    // raw holds the on-disk bytes if needed
    const uint8_t *raw;
    size_t raw_len;
} vstodo_tree;

// Parse decodes tree bytes.
// Input: id (object digest), data (raw tree bytes)
// Output: tree with entries populated, or error (non-zero return)
int vstodo_tree_parse(const uint8_t *data, size_t data_len, vstodo_sha1_digest id, vstodo_tree *t);

// Marshal encodes the tree (entries sorted by name, git convention).
// Output: allocated buffer, *out_len set to result size
uint8_t *vstodo_tree_marshal(const vstodo_tree *t, size_t *out_len);

// ID returns the content-addressed digest.
vstodo_sha1_digest vstodo_tree_id(vstodo_tree *t);

// Build constructs a tree from entries.
vstodo_tree *vstodo_tree_build(vstodo_tree_entry *entries, size_t num_entries);

// Helper: create a single tree entry
vstodo_tree_entry vstodo_tree_entry_make(const char *mode, const char *name, vstodo_sha1_digest id);

// Validate an entry name (no traversal, no dots, no slashes)
int vstodo_tree_validate_name(const char *name);

#endif // VSTODO_C_TREE_H
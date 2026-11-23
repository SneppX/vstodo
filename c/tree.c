// C implementation of vstodo Tree types and parsing
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/tree package interface but does not
// implement full tree parsing (requires SHA-1 and object type handling)
// or marshaling without the full object framework.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/tree/tree.go.

#include "tree.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// --- vstodo_tree_entry ---

vstodo_tree_entry vstodo_tree_entry_make(const char *mode, const char *name, vstodo_sha1_digest id) {
    vstodo_tree_entry entry;
    entry.mode = mode ? mode : "";
    entry.name = name ? name : "";
    entry.id = id;
    return entry;
}

// --- vstodo_tree ---

int vstodo_tree_validate_name(const char *name) {
    if (name == NULL || name[0] == '\0' || name[0] == '.' || name[strlen(name)-1] == '.') {
        return -1;  // invalid
    }
    // Check for path traversal and slashes
    if (strstr(name, "..") != NULL) return -1;
    if (strstr(name, "/") != NULL) return -1;
    if (strstr(name, "\\") != NULL) return -1;
    return 0;  // valid
}

int vstodo_tree_parse(const uint8_t *data, size_t data_len, vstodo_sha1_digest id, vstodo_tree *t) {
    if (data == NULL || data_len == 0 || t == NULL) return -1;
    
    t->entries = NULL;
    t->num_entries = 0;
    t->raw = data;
    t->raw_len = data_len;
    
    // In full implementation, this would:
    // 1. Iterate through the tree byte format: "mode name\0<20-byte-id>" repeated
    // 2. Parse each entry using sha1.FromBytes on the 20-byte hash
    // 3. Validate names (no traversal, no dots)
    // 4. Sort entries by name (git convention)
    // For now, return error — pending full implementation
    
    (void)id;
    return -1;  // stub: pending full implementation
}

uint8_t *vstodo_tree_marshal(const vstodo_tree *t, size_t *out_len) {
    if (t == NULL || out_len == NULL) return NULL;
    
    // In full implementation, this would:
    // 1. Sort entries by name (git convention)
    // 2. Format each entry: "mode name\0<20-byte-id>"
    // 3. Concatenate all entries
    // 4. Return the buffer
    
    // For now, just return a buffer with entry count info
    *out_len = 0;
    uint8_t *buf = (uint8_t *)malloc(4);
    if (buf == NULL) return NULL;
    memcpy(buf, &t->num_entries, 4);
    return buf;
}

vstodo_sha1_digest vstodo_tree_id(vstodo_tree *t) {
    // In full implementation, compute SHA-1 of marshaled tree
    (void)t;
    static vstodo_sha1_digest zero = {0};
    return zero;
}

vstodo_tree *vstodo_tree_build(vstodo_tree_entry *entries, size_t num_entries) {
    vstodo_tree *t = (vstodo_tree *)malloc(sizeof(vstodo_tree));
    if (t == NULL) return NULL;
    
    t->entries = entries;
    t->num_entries = num_entries;
    t->raw = NULL;
    t->raw_len = 0;
    return t;
}

// --- C legacy compatibility: Parse from hex id + data ---
// This mirrors the Go func Parse(id sha1.Digest, data []byte) (*Tree, error)

int vstodo_tree_parse_legacy(vstodo_sha1_digest id, const uint8_t *data, size_t data_len, vstodo_tree **out_tree) {
    if (out_tree == NULL) return -1;
    
    // Create tree
    vstodo_tree *t = vstodo_tree_build(NULL, 0);
    if (t == NULL) return -1;
    
    *out_tree = t;
    return 0;
}

int main(void) {
    printf("Tree Test Suite (C skeleton)\\n\\n");
    printf("Tree skeleton created. Full implementation pending.\\n");
    return 0;
}
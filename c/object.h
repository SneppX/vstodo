// C implementation of vstodo Object types and Store interface
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/object package interface but does not
// implement full object persistence (that requires the storage layer).
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/object/object.go.

#ifndef VSTODO_C_OBJECT_H
#define VSTODO_C_OBJECT_H

#include "sha1.h"
#include <stddef.h>
#include <stdlib.h>

// Type is the type of an object.
typedef struct {
    const char *value;
} vstodo_object_type;

// Object type constants
static const vstodo_object_type vstodo_blob_type = {"blob"};
static const vstodo_object_type vstodo_tree_type = {"tree"};
static const vstodo_object_type vstodo_commit_type = {"commit"};
static const vstodo_object_type vstodo_tag_type = {"tag"};

// validTypes is the set of object types vstodo supports.
// In C, we use a simple array + count for the "set".
#define VSTODO_OBJECT_TYPE_BLOB   0
#define VSTODO_OBJECT_TYPE_TREE   1
#define VSTODO_OBJECT_TYPE_COMMIT 2
#define VSTODO_OBJECT_TYPE_TAG    3

#define VSTODO_OBJECT_TYPE_COUNT 4

// Object holds decoded object content and its identity.
typedef struct {
    vstodo_object_type type;
    void *data;        // raw bytes — size known by caller
    vstodo_sha1_digest id;  // content-addressed SHA-1
} vstodo_object;

// ObjectStore persists objects content-addressed by SHA-1.
// This is an interface/abstract type — concrete implementation
// lives in the storage layer (c/storage.h).
typedef struct {
    // Concrete store implements Store/Load/Has
} vstodo_object_store;

// Store writes obj and returns its digest.
// Returns the SHA-1 digest of the stored object.
vstodo_sha1_digest vstodo_object_store_store(vstodo_object_store *store, const vstodo_object *obj);

// Load returns the decoded object for id.
const vstodo_object *vstodo_object_store_load(vstodo_object_store *store, vstodo_sha1_digest id);

// Has returns true if id is present.
bool vstodo_object_store_has(vstodo_object_store *store, vstodo_sha1_digest id);

// ComputeID derives the git-compatible SHA-1 address of content.
// Format: "<type> <size>\0<data>"
void vstodo_sha1_compute_id(const char *typ, const uint8_t *data, size_t data_len, vstodo_sha1_digest result);

// Marshal serializes an object into its wire/on-disk bytes: header + NUL + payload.
 // Output: allocated buffer, *out_len set to result size
uint8_t *vstodo_object_marshal(const vstodo_object *obj, size_t *out_len);

// Unmarshal splits header bytes into type and payload.
// Returns: type, data, error (as int: 0=ok, -1=malformed, -2=unknown type)
int vstodo_object_unmarshal(const uint8_t *raw, size_t raw_len,
                            vstodo_object_type *type, uint8_t **data, size_t *data_len);

// Compress returns the zlib-compressed on-disk form of an object.
uint8_t *vstodo_object_compress(const vstodo_object *obj, size_t *out_len);

// Decompress reverses Compress.
vstodo_object *vstodo_object_decompress(const uint8_t *compressed, size_t compressed_len, size_t *out_obj_data_len);

// HashBlob computes the SHA-1 address of file content (git byte-identical).
vstodo_sha1_digest vstodo_sha1_hash_blob(const uint8_t *data, size_t data_len);

// HashTree computes the SHA-1 address of a tree object.
vstodo_sha1_digest vstodo_sha1_hash_tree(const uint8_t *data, size_t data_len);

// HashCommit computes the SHA-1 address of a commit object.
vstodo_sha1_digest vstodo_sha1_hash_commit(const uint8_t *data, size_t data_len);

// TreeEntry is one entry in a tree object.
typedef struct {
    const char *mode;  // e.g. "100644", "100755", "040000", "120000"
    const char *name;
    vstodo_sha1_digest id;
} vstodo_tree_entry;

// Tree represents a directory object.
typedef struct {
    vstodo_tree_entry *entries;
    size_t num_entries;
} vstodo_tree;

// Build constructs a tree from entries.
vstodo_tree *vstodo_tree_build(vstodo_tree_entry *entries, size_t num_entries);

// ID returns the content-addressed digest.
vstodo_sha1_digest vstodo_tree_id(vstodo_tree *t);

// Commit represents a snapshot object.
typedef struct {
    vstodo_sha1_digest tree;
    vstodo_sha1_digest *parents;
    size_t num_parents;
    const char *author;
    const char *committer;
    int64_t author_ts;
    int64_t committer_ts;
    const char *message;
} vstodo_commit;

// Build constructs a Git-compatible commit object.
vstodo_commit *vstodo_commit_build(vstodo_sha1_digest tree, vstodo_sha1_digest *parents,
                                    size_t num_parents, const char *author, const char *committer,
                                    int64_t author_ts, int64_t committer_ts, const char *message);

// ID returns the content-addressed digest.
vstodo_sha1_digest vstodo_commit_id(vstodo_commit *c);

// Tag represents an annotated tag object.
typedef struct {
    vstodo_sha1_digest object;
    vstodo_object_type type;
    const char *tagger;
    int64_t tagger_ts;
    const char *message;
} vstodo_tag;

// Build constructs an annotated tag object.
vstodo_tag *vstodo_tag_build(vstodo_sha1_digest obj, vstodo_object_type type,
                              const char *tagger, int64_t tagger_ts, const char *message);

// ID returns the content-addressed digest.
vstodo_sha1_digest vstodo_tag_id(vstodo_tag *t);

#endif // VSTODO_C_OBJECT_H
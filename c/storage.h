// C implementation of vstodo Storage (Loose object store)
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/storage package interface but does not
// implement full object persistence without the filesystem layout.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/storage/storage.go.

#ifndef VSTODO_C_STORAGE_H
#define VSTODO_C_STORAGE_H

#include "object.h"
#include "sha1.h"
#include <stdbool.h>

// LooseStore persists objects as immutable loose files.
// Objects live at .vstodo/objects/<2-hex>/<38-hex>, zlib-compressed.
typedef struct {
    const char *objects_dir;
} vstodo_loose_store;

// NewLooseStore opens (creating) the loose object store.
vstodo_loose_store *vstodo_loose_store_new(const char *objects_dir);

// Store writes obj as an immutable object.
// Returns the SHA-1 digest of the stored object.
vstodo_sha1_digest vstodo_loose_store_store(vstodo_loose_store *store, const vstodo_object *obj);

// Load reads, decompresses, and verifies the object for id.
const vstodo_object *vstodo_loose_store_load(vstodo_loose_store *store, vstodo_sha1_digest id);

// Has reports whether id exists.
bool vstodo_loose_store_has(vstodo_loose_store *store, vstodo_sha1_digest id);

// objectPath returns .vstodo/objects/<2>/<38>.
const char *vstodo_loose_store_object_path(vstodo_loose_store *store, vstodo_sha1_digest id);

// Objects enumerates validly named regular loose-object files in stable order.
 // Output: allocated array of digests, *out_count set to count
 // Caller must free with vstodo_free()
vstodo_sha1_digest *vstodo_loose_store_objects(vstodo_loose_store *store, size_t *out_count);

// ReadObject reads raw payload bytes for id (decompressed).
uint8_t *vstodo_loose_store_read_object(vstodo_loose_store *store, vstodo_sha1_digest id, size_t *out_len);

// ReadRaw returns the compressed on-disk bytes for id.
uint8_t *vstodo_loose_store_read_raw(vstodo_loose_store *store, vstodo_sha1_digest id, size_t *out_len);

// StoreRaw validates compressed bytes against id before atomically storing them.
// Existing immutable objects are never overwritten.
int vstodo_loose_store_store_raw(vstodo_loose_store *store, vstodo_sha1_digest id, const uint8_t *compressed, size_t compressed_len);

// WriteRaw stores raw payload as type and returns digest.
vstodo_sha1_digest vstodo_loose_store_write_raw(vstodo_loose_store *store, int obj_type, const uint8_t *data, size_t data_len);

// Compress returns the zlib-compressed on-disk form of an object.
uint8_t *vstodo_loose_store_compress(vstodo_loose_store *store, const vstodo_object *obj, size_t *out_len);

// Decompress reverses Compress.
const vstodo_object *vstodo_loose_store_decompress(vstodo_loose_store *store, const uint8_t *compressed, size_t compressed_len, size_t *out_obj_data_len);

// --- Helper: object path computation ---

// objectPath returns .vstodo/objects/<2>/<38>.
const char *vstodo_loose_store_compute_object_path(vstodo_loose_store *store, vstodo_sha1_digest id);

// --- Object type constants ---

#define VSTODO_OBJECT_TYPE_BLOB 0
#define VSTODO_OBJECT_TYPE_TREE 1
#define VSTODO_OBJECT_TYPE_COMMIT 2
#define VSTODO_OBJECT_TYPE_TAG 3

#endif // VSTODO_C_STORAGE_H
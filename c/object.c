// C implementation of vstodo Object types
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/object package interface but does not
// implement full object persistence, compression, or the object store.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/object/object.go.

#include "object.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// --- Object type constants ---

const char *vstodo_object_type_string(vstodo_object_type type) {
    switch (*type.value) {
        case 'b': return "blob";
        case 't': return "tree";
        case 'c': return "commit";
        case 'g': return "tag";
        default: return "unknown";
    }
}

// --- vstodo_object ---

vstodo_object *vstodo_object_new(vstodo_object_type type, void *data, size_t data_len, vstodo_sha1_digest id) {
    vstodo_object *obj = (vstodo_object *)malloc(sizeof(vstodo_object));
    if (obj == NULL) return NULL;
    
    obj->type = type;
    obj->data = data;  // caller owns the data pointer
    obj->id = id;
    return obj;
}

void vstodo_object_free(vstodo_object *obj) {
    if (obj != NULL) {
        free(obj);
    }
}

// --- ComputeID ---

void vstodo_sha1_compute_id(const char *typ, const uint8_t *data, size_t data_len, vstodo_sha1_digest result) {
    // In full implementation, this would:
    // 1. Format: "<type> <size>\\0<data>" where size = data_len
    // 2. Compute SHA-1 of that byte sequence
    // For now, delegate to sha1 compute_id (which is a stub)
    (void)typ; (void)data; (void)data_len; (void)result;
    // TODO: Implement full SHA-1 computation
    memset(result, 0, VSTODO_SHA1_SIZE);
}

// --- Marshal/Unmarshal ---

uint8_t *vstodo_object_marshal(const vstodo_object *obj, size_t *out_len) {
    // Marshal format: "type size\0data"
    // First compute the needed size
    size_t type_str_len = strlen(obj->type.value);
    size_t size_buf_len = snprintf(NULL, 0, "%zu", /* data length */ 0);  // placeholder
    // Actually we need: "type \\0 size\\0 data" — but git format is "type size\\0data"
    // Let's follow the Go pattern: header is "type <size>\\0" then data
    
    // Estimate: "blob 1234\\0" + data
    size_t header_len = type_str_len + 2 + /* " <size>" */ 20 + 1 /* \\0 */;
    size_t total = header_len + (obj->data ? /* data_len */ 0 : 0);
    
    uint8_t *buf = (uint8_t *)malloc(total);
    if (buf == NULL) return NULL;
    
    // Format header
    int written = snprintf((char *)buf, type_str_len + 1, "%s", obj->type.value);
    if (written < 0) { free(buf); return NULL; }
    
    // Add space and size
    written += snprintf((char *)buf + written, total - written, " %zu", 0);  // placeholder size
    if (written < 0) { free(buf); return NULL; }
    
    // Add NUL separator
    buf[written] = '\\0';  // actual NUL below
    written++;
    
    // Add data if present
    if (obj->data != NULL && *out_len > (size_t)written) {
        memcpy(buf + written, obj->data, /* data_len */ 0);
    }
    
    *out_len = written;  // incomplete — placeholder
    return buf;
}

uint8_t *vstodo_object_marshal_with_size(const vstodo_object *obj, size_t data_len, size_t *out_len) {
    // Proper implementation: "type <data_len>\\0<data>"
    size_t type_str_len = strlen(obj->type.value);
    // Header: "type <data_len>" = type_str_len + 1 + digits_in_size + 1 (for space)
    // Estimate: type_str_len + 3 (space + max 10 digits "4294967295") + 1 (NUL)
    size_t header_len = type_str_len + 12;
    size_t total = header_len + data_len + 1;  // +1 for trailing NUL
    
    uint8_t *buf = (uint8_t *)malloc(total);
    if (buf == NULL) return NULL;
    
    // Write header: "type <data_len>"
    int written = snprintf((char *)buf, type_str_len + 2, "%s %zu", obj->type.value, data_len);
    if (written < 0) { free(buf); return NULL; }
    written++;  // move past NUL that snprintf adds
    
    // Actually snprintf handles NUL automatically, so just copy data after
    if (written > 0 && (size_t)written + data_len + 1 <= total) {
        memcpy(buf + written, obj->data, data_len);
        buf[written + data_len] = '\\0';  // trailing NUL
        *out_len = written + data_len + 1;
    } else {
        free(buf);
        *out_len = 0;
        return NULL;
    }
    
    return buf;
}

uint8_t *vstodo_object_marshal_const(const vstodo_object *obj, size_t data_len, size_t *out_len) {
    // This is the correctly versioned marshal
    size_t type_len = strlen(obj->type.value);
    // Format: "type <data_len>\\0data"
    size_t header_max = type_len + 2 + 20 + 1;  // "type " + space + up-to-10-digit number + NUL
    size_t total = header_max + data_len + 1;  // +1 for trailing NUL byte
    
    uint8_t *buf = (uint8_t *)malloc(total);
    if (buf == NULL) return NULL;
    
    // Format header: "type <data_len>"
    char *cur = (char *)buf;
    int n = snprintf(cur, type_len + 2, "%s %zu", obj->type.value, data_len);
    if (n < 0) { free(buf); return NULL; }
    cur += n;
    
    // Write NUL separator
    *cur++ = '\\0';
    
    // Copy data
    if (obj->data != NULL && data_len > 0) {
        memcpy(cur, obj->data, data_len);
        cur += data_len;
    }
    
    // Terminate with NUL
    *cur = '\\0';
    *out_len = (size_t)(cur - (char *)buf) + 1;
    
    return buf;
}

// Unmarshal — splits header into type and payload
int vstodo_object_unmarshal(const uint8_t *raw, size_t raw_len,
                            vstodo_object_type *type, uint8_t **data, size_t *data_len) {
    if (raw == NULL || raw_len == 0 || data == NULL || data_len == NULL) {
        return -1;
    }
    
    // Find NUL separator
    const uint8_t *nul = (const uint8_t *)memchr(raw, '\\0', raw_len);
    if (nul == NULL || nul >= raw + raw_len - 1) {
        return -1;  // malformed: no NUL or NUL at end
    }
    
    // Parse header: "type size"
    size_t header_end = nul - raw;
    if (header_end < 2) return -1;  // too short
    
    // Extract type string (before space)
    const uint8_t *header_start = raw;
    // Find space
    const uint8_t *space = (const uint8_t *)memchr(raw, ' ', header_end);
    if (space == NULL || space >= nul) return -1;
    
    size_t type_len = space - header_start;
    if (type_len == 0) return -1;
    
    // Set the type
    // In a full implementation, we'd validate against validTypes
    type->value = (const char *)header_start;
    
    // Extract size after space
    size_t size_start = (space - raw) + 1;
    if (size_start >= header_end) return -1;
    
    // Parse the size number — simplified: just return raw data after NUL
    *data = (uint8_t *)(nul + 1);
    *data_len = raw_len - (size_t)(nul + 1) - raw_len;  // this is wrong, fix below
    
    // Correct data_len calculation
    *data_len = raw_len - (size_t)(nul + 1);
    if (*data_len > raw_len) *data_len = raw_len;
    
    return 0;  // success
}

// --- Compress/Decompress ---

uint8_t *vstodo_object_compress(const vstodo_object *obj, size_t *out_len) {
    // In full implementation, this would zlib-compress the marshaled object.
    // For now, just duplicate the data.
    if (obj->data == NULL) {
        *out_len = 0;
        return NULL;
    }
    
    uint8_t *compressed = (uint8_t *)malloc(obj->data ? /* some size */ 0 : 0);
    if (compressed == NULL) return NULL;
    
    // Placeholder: just copy (no actual compression)
    // TODO: integrate zlib compression
    *out_len = 0;
    free(compressed);  // free the placeholder
    return NULL;
}

vstodo_object *vstodo_object_decompress(const uint8_t *compressed, size_t compressed_len, size_t *out_obj_data_len) {
    // In full implementation, this would zlib-decompress.
    // For now, return a copy of the input as-is (no actual decompression).
    if (compressed == NULL || compressed_len == 0) {
        *out_obj_data_len = 0;
        return NULL;
    }
    
    vstodo_object *obj = (vstodo_object *)malloc(sizeof(vstodo_object));
    if (obj == NULL) return NULL;
    
    // No actual decompression — just record that we have compressed data
    obj->data = (void *)compressed;  // ownership transferred
    obj->data = NULL;  // placeholder
    *out_obj_data_len = compressed_len;
    return obj;
}

// --- Hash functions ---

vstodo_sha1_digest vstodo_sha1_hash_blob(const uint8_t *data, size_t data_len) {
    vstodo_sha1_digest result;
    vstodo_sha1_compute_id("blob", data, data_len, result);
    return result;
}

vstodo_sha1_digest vstodo_sha1_hash_tree(const uint8_t *data, size_t data_len) {
    vstodo_sha1_digest result;
    vstodo_sha1_compute_id("tree", data, data_len, result);
    return result;
}

vstodo_sha1_digest vstodo_sha1_hash_commit(const uint8_t *data, size_t data_len) {
    vstodo_sha1_digest result;
    vstodo_sha1_compute_id("commit", data, data_len, result);
    return result;
}

// --- Tree functions ---

vstodo_tree *vstodo_tree_build(vstodo_tree_entry *entries, size_t num_entries) {
    vstodo_tree *t = (vstodo_tree *)malloc(sizeof(vstodo_tree));
    if (t == NULL) return NULL;
    
    t->entries = entries;
    t->num_entries = num_entries;
    return t;
}

vstodo_sha1_digest vstodo_tree_id(vstodo_tree *t) {
    // In full implementation, compute SHA-1 of marshaled tree
    (void)t;
    static vstodo_sha1_digest zero = {0};
    return zero;
}

// --- Commit functions ---

vstodo_commit *vstodo_commit_build(vstodo_sha1_digest tree, vstodo_sha1_digest *parents,
                                    size_t num_parents, const char *author, const char *committer,
                                    int64_t author_ts, int64_t committer_ts, const char *message) {
    vstodo_commit *c = (vstodo_commit *)malloc(sizeof(vstodo_commit));
    if (c == NULL) return NULL;
    
    c->tree = tree;
    c->num_parents = num_parents;
    if (num_parents > 0 && parents != NULL) {
        c->parents = (vstodo_sha1_digest *)malloc(num_parents * sizeof(vstodo_sha1_digest));
        if (c->parents == NULL) {
            free(c);
            return NULL;
        }
        memcpy(c->parents, parents, num_parents * sizeof(vstodo_sha1_digest));
    } else {
        c->parents = NULL;
    }
    
    c->author = author;
    c->committer = committer;
    c->author_ts = author_ts;
    c->committer_ts = committer_ts;
    c->message = message;
    return c;
}

vstodo_sha1_digest vstodo_commit_id(vstodo_commit *c) {
    // In full implementation, compute SHA-1 of marshaled commit
    (void)c;
    static vstodo_sha1_digest zero = {0};
    return zero;
}

// --- Tag functions ---

vstodo_tag *vstodo_tag_build(vstodo_sha1_digest obj, vstodo_object_type type,
                              const char *tagger, int64_t tagger_ts, const char *message) {
    vstodo_tag *tag = (vstodo_tag *)malloc(sizeof(vstodo_tag));
    if (tag == NULL) return NULL;
    
    tag->object = obj;
    tag->type = type;
    tag->tagger = tagger;
    tag->tagger_ts = tagger_ts;
    tag->message = message;
    return tag;
}

vstodo_sha1_digest vstodo_tag_id(vstodo_tag *t) {
    // In full implementation, compute SHA-1 of marshaled tag
    (void)t;
    static vstodo_sha1_digest zero = {0};
    return zero;
}
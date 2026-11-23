// C implementation of vstodo Commit types and parsing
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/commit package interface but does not
// implement full commit persistence or parsing without the object store.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/commit/commit.go.

#ifndef VSTODO_C_COMMIT_H
#define VSTODO_C_COMMIT_H

#include "object.h"
#include "sha1.h"
#include <stddef.h>
#include <stdlib.h>
#include <string.h>

// Commit is a decoded commit object.
typedef struct {
    vstodo_sha1_digest tree;
    vstodo_sha1_digest *parents;
    size_t num_parents;
    const char *author;
    const char *author_tz;    // timezone suffix (e.g. "+0000")
    int64_t author_ts;        // Unix timestamp
    const char *committer;
    const char *committer_tz;
    int64_t committer_ts;
    const char *message;
    int author_tz_set;
    int committer_tz_set;
} vstodo_commit;

// Parse decodes a Git commit object while retaining its original bytes.
// Keeping the original representation preserves unknown headers,
// signatures, header ordering, and the exact message when the
// object is marshalled again.
int vstodo_commit_parse(const uint8_t *data, size_t data_len, vstodo_sha1_digest id, vstodo_commit *c);

// Marshals the exact original bytes for parsed commits. For newly
// constructed commits it serializes the fields in Git's canonical header order.
uint8_t *vstodo_commit_marshal(const vstodo_commit *c, size_t *out_len);

// ID returns the content-addressed digest.
vstodo_sha1_digest vstodo_commit_id(vstodo_commit *c);

// formatIdentity separates the identity, Unix timestamp, and timezone from a
// Git author/committer header. Malformed suffixes are retained as identity text
// rather than silently inventing metadata.
int vstodo_commit_parse_identity(const char *value, const char **name, int64_t *ts, const char **tz);

// parseIdentity (internal) — separates the identity, Unix timestamp, and timezone
// from a Git author/committer header.
int vstodo_commit_internal_parse_identity(const char *value, const char **name, int64_t *ts, const char **tz);

// formatIdentity (internal) — format an identity string
int vstodo_commit_internal_format_identity(char *buf, size_t buf_len,
                                          const char *name, int64_t ts, const char *tz);

// normalizeIdentity ensures the identity has the form "Name <email>"
int vstodo_commit_normalize_identity(const char *identity, char *buf, size_t buf_len);

// formatTZ formats the timezone offset from a timestamp
int vstodo_commit_format_tz(int64_t timestamp, char *buf, size_t buf_len);

// Build constructs a Git-compatible commit object.
// An identity may be supplied as "Name <email>" or as a short name, in which
// case the name is also used as the email address.
vstodo_commit *vstodo_commit_build(vstodo_sha1_digest tree, vstodo_sha1_digest *parents,
                                    size_t num_parents, const char *author, const char *committer,
                                    const char *message);

// Author/committer identity formatting helpers

#endif // VSTODO_C_COMMIT_H
// C implementation of vstodo Commit types and parsing
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/commit package interface but does not
// implement full commit parsing (requires SHA-1, object type handling,
// and Git header format knowledge).
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/commit/commit.go.

#include "commit.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// --- vstodo_commit ---

int vstodo_commit_parse_identity(const char *value, const char **name, int64_t *ts, const char **tz) {
    if (value == NULL) return -1;
    
    // In full implementation, this would parse "name email timestamp tz"
    // e.g. "John Doe <john@example.com> 1234567890 +0000"
    // For now, stub: return the full value as name, 0 timestamp, empty tz
    
    *name = value;
    *ts = 0;
    *tz = "";
    return 0;
}

int vstodo_commit_internal_parse_identity(const char *value, const char **name, int64_t *ts, const char **tz) {
    return vstodo_commit_parse_identity(value, name, ts, tz);
}

int vstodo_commit_internal_format_identity(char *buf, size_t buf_len,
                                          const char *name, int64_t ts, const char *tz) {
    if (buf == NULL || buf_len == 0) return -1;
    if (name == NULL) { name = ""; }
    
    int n;
    if (tz != NULL && tz[0] != '\0') {
        n = snprintf(buf, buf_len, "%s %d %s", name, (int)ts, tz);
    } else {
        n = snprintf(buf, buf_len, "%s %d", name, (int)ts);
    }
    if (n < 0 || n >= (int)buf_len) return -1;
    return 0;
}

int vstodo_commit_normalize_identity(const char *identity, char *buf, size_t buf_len) {
    // In full implementation, this would ensure "Name <email>" format
    // For now, just copy the identity
    if (identity == NULL || buf == NULL || buf_len == 0) return -1;
    if (buf_len > 0) {
        buf[0] = '\0';
        strncat(buf, identity, buf_len - 1);
    }
    return 0;
}

int vstodo_commit_format_tz(int64_t timestamp, char *buf, size_t buf_len) {
    // In full implementation, this would format the timezone offset
    // e.g. "+0000", "-0500"
    // For now, return "+0000"
    if (buf == NULL || buf_len == 0) return -1;
    strncpy(buf, "+0000", buf_len);
    buf[buf_len - 1] = '\0';
    return 0;
}

// --- vstodo_commit_parse ---

int vstodo_commit_parse(const uint8_t *data, size_t data_len, vstodo_sha1_digest id, vstodo_commit *c) {
    if (data == NULL || data_len == 0 || c == NULL) return -1;
    
    // Zero-initialize the commit struct
    memset(c, 0, sizeof(*c));
    
    // In full implementation, this would:
    // 1. Find the "\n\n" separator between header and message
    // 2. Parse each header line: "key value"
    // 3. Handle "tree", "parent", "author", "committer", "message"
    // 4. Parse identities (author/committer) with timestamps and timezones
    // 5. Store the raw data for round-trip preservation
    // For now, return error — pending full implementation
    
    (void)data; (void)data_len; (void)id; (void)c;
    return -1;
}

// --- vstodo_commit_marshal ---

uint8_t *vstodo_commit_marshal(const vstodo_commit *c, size_t *out_len) {
    if (c == NULL || out_len == NULL) return NULL;
    
    // In full implementation, this would serialize in Git's canonical header order:
    // "tree <tree-hash>\nparent <parent-hash>\nauthor <name> <email> <timestamp> <tz>\n"
    // "committer <name> <email> <timestamp> <tz>\n\n<message>"
    // For now, return a stub buffer
    
    *out_len = 0;
    uint8_t *buf = (uint8_t *)malloc(4);
    if (buf == NULL) return NULL;
    memcpy(buf, "COMM", 4);  // placeholder magic
    return buf;
}

// --- vstodo_commit_id ---

vstodo_sha1_digest vstodo_commit_id(vstodo_commit *c) {
    // In full implementation, compute SHA-1 of marshaled commit
    (void)c;
    static vstodo_sha1_digest zero = {0};
    return zero;
}

// --- vstodo_commit_build ---

vstodo_commit *vstodo_commit_build(vstodo_sha1_digest tree, vstodo_sha1_digest *parents,
                                    size_t num_parents, const char *author, const char *committer,
                                    const char *message) {
    vstodo_commit *c = (vstodo_commit *)malloc(sizeof(vstodo_commit));
    if (c == NULL) return NULL;
    
    c->tree = tree;
    c->num_parents = num_parents;
    c->author = author ? author : "";
    c->committer = committer ? committer : "";
    c->message = message ? message : "";
    c->author_ts = 0;
    c->committer_ts = 0;
    c->author_tz_set = 0;
    c->committer_tz_set = 0;
    
    // Allocate parents array if needed
    if (num_parents > 0) {
        c->parents = (vstodo_sha1_digest *)malloc(num_parents * sizeof(vstodo_sha1_digest));
        if (c->parents == NULL) {
            free(c);
            return NULL;
        }
        memcpy(c->parents, parents, num_parents * sizeof(vstodo_sha1_digest));
    } else {
        c->parents = NULL;
    }
    
    return c;
}

// --- Author/committer helper functions (stubs) ---

int vstodo_commit_parse_identity(const char *value, const char **name, int64_t *ts, const char **tz) {
    return vstodo_commit_parse_identity(value, name, ts, tz);
}

int vstodo_commit_normalize_identity(const char *identity, char *buf, size_t buf_len) {
    return vstodo_commit_normalize_identity(identity, buf, buf_len);
}

int vstodo_commit_format_tz(int64_t timestamp, char *buf, size_t buf_len) {
    return vstodo_commit_format_tz(timestamp, buf, buf_len);
}

// --- Main ---

int main(void) {
    printf("Commit Test Suite (C skeleton)\\n\\n");
    printf("Commit skeleton created. Full implementation pending.\\n");
    return 0;
}
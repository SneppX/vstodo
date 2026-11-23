// C implementation of SHA-1 hash algorithm used by vstodo
// (and by git) for object addressing. Objects are addressed by the
// SHA-1 of their header-prefixed content: "<type> <size>\0<content>".

#ifndef VSTODO_C_SHA1_H
#define VSTODO_C_SHA1_H

#include <stdint.h>
#include <stddef.h>
#include <string.h>

// Size is the byte length of a SHA-1 digest (20 bytes, 40 hex chars).
#define VSTODO_SHA1_SIZE 20

// Digest is a 20-byte SHA-1 hash.
typedef uint8_t vstodo_sha1_digest[VSTODO_SHA1_SIZE];

// Zero is the empty digest.
static const vstodo_sha1_digest vstodo_sha1_zero = {0};

// Hasher wraps a SHA-1 implementation.
typedef struct {
    // Internal state — platform-specific implementation
    // In a pure-C implementation without OpenSSL, we implement
    // the SHA-1 padding and compression manually.
} vstodo_sha1_hasher;

// NewHasher returns a fresh Hasher.
vstodo_sha1_hasher vstodo_sha1_new_hasher(void);

// Write feeds data into the hasher.
void vstodo_sha1_hasher_write(vstodo_sha1_hasher *h, const uint8_t *p, size_t len);

// SumDigest returns the current digest.
vstodo_sha1_digest vstodo_sha1_hasher_sum_digest(vstodo_sha1_hasher *h);

// Reset clears the hasher state.
void vstodo_sha1_hasher_reset(vstodo_sha1_hasher *h);

// Hex encodes the digest as a 40-character lowercase hex string.
// Returns a statically allocated string (valid until next call).
const char *vstodo_sha1_digest_hex(const vstodo_sha1_digest d);

// String returns the hex form.
const char *vstodo_sha1_digest_string(const vstodo_sha1_digest d);

// FromHex parses a 40-char hex string into a Digest.
// Returns 0 on success, -1 on failure.
int vstodo_sha1_from_hex(const char *s, vstodo_sha1_digest d);

// FromBytes copies 20 bytes into a Digest.
void vstodo_sha1_from_bytes(const uint8_t *b, vstodo_sha1_digest d);

// Bytes returns the raw 20 bytes.
void vstodo_sha1_digest_bytes(const vstodo_sha1_digest d, uint8_t *out);

// Prefix returns the first n bytes used for loose-object directory naming.
const char *vstodo_sha1_digest_prefix(const vstodo_sha1_digest d, int n);

// ComputeID derives the git-compatible SHA-1 address of content.
// Input: type string (e.g. "blob", "tree", "commit", "tag"), data bytes
// Output: 20-byte digest
void vstodo_sha1_compute_id(const char *typ, const uint8_t *data, size_t data_len, vstodo_sha1_digest result);

// objHeader produces the pack object header: type in high nibble of first byte,
// low 4 size bits in the first byte, then 7-bit groups with continuation.
// Used by packfile module.
void vstodo_sha1_obj_header(uint8_t type, uint64_t size, uint8_t *out, size_t *out_len);

#endif // VSTODO_C_SHA1_H
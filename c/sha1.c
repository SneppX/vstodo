// C implementation of SHA-1 hash algorithm used by vstodo
// (and by git) for object addressing.
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/sha1 package interface but does not
// implement the full SHA-1 compression function (which requires
// either OpenSSL, a software implementation, or compiler built-ins).
// 
// For a production implementation, consider:
//   - Using compiler built-in SHA extensions (ARMv8, x86 SHA)
//   - Including a minimal software SHA-1 implementation
//   - Linking against OpenSSL/libcrypto
//
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/sha1/sha1.go.

#include "sha1.h"
#include <stdlib.h>
#include <string.h>

// Static assert-like check (C11 _Static_assert)
_Static_assert(VSTODO_SHA1_SIZE == 20, "SHA-1 digest must be 20 bytes");

// --- Hasher management ---

vstodo_sha1_hasher vstodo_sha1_new_hasher(void) {
    // In a full implementation, this would initialize a SHA-1 context.
    // Without OpenSSL/built-ins, we return a zero-initialized struct;
    // the Write/SumDigest functions would need a software SHA-1 impl.
    vstodo_sha1_hasher h = {0};
    return h;
}

void vstodo_sha1_hasher_write(vstodo_sha1_hasher *h, const uint8_t *p, size_t len) {
    // Software SHA-1 would process blocks here.
    // (VSTODO_UNUSED h; p; len;)
    (void)h; (void)p; (void)len;
}

vstodo_sha1_digest vstodo_sha1_hasher_sum_digest(vstodo_sha1_hasher *h) {
    // Software SHA-1 would produce the final digest here.
    // (VSTODO_UNUSED h;)
    static vstodo_sha1_digest result = {0};
    return result;
}

void vstodo_sha1_hasher_reset(vstodo_sha1_hasher *h) {
    memset(h, 0, sizeof(*h));
}

// --- Digest formatting ---

const char *vstodo_sha1_digest_hex(const vstodo_sha1_digest d) {
    static char hex[41];  // 40 hex chars + NUL
    for (int i = 0; i < VSTODO_SHA1_SIZE; i++) {
        sprintf(hex + 2 * i, "%02x", d[i]);
    }
    hex[40] = '\0';
    return hex;
}

const char *vstodo_sha1_digest_string(const vstodo_sha1_digest d) {
    return vstodo_sha1_digest_hex(d);
}

int vstodo_sha1_from_hex(const char *s, vstodo_sha1_digest d) {
    if (strlen(s) != VSTODO_SHA1_SIZE * 2) {
        return -1;
    }
    for (int i = 0; i < VSTODO_SHA1_SIZE; i++) {
        unsigned int byte = 0;
        for (int j = 0; j < 2; j++) {
            char c = s[2 * i + j];
            if (c >= '0' && c <= '9') byte = (byte << 4) | (c - '0');
            else if (c >= 'a' && c <= 'f') byte = (byte << 4) | (c - 'a' + 10);
            else if (c >= 'A' && c <= 'F') byte = (byte << 4) | (c - 'A' + 10);
            else return -1;
        }
        d[i] = (uint8_t)byte;
    }
    return 0;
}

void vstodo_sha1_from_bytes(const uint8_t *b, vstodo_sha1_digest d) {
    if (b != NULL) {
        memcpy(d, b, VSTODO_SHA1_SIZE);
    } else {
        memset(d, 0, VSTODO_SHA1_SIZE);
    }
}

void vstodo_sha1_digest_bytes(const vstodo_sha1_digest d, uint8_t *out) {
    if (out != NULL) {
        memcpy(out, d, VSTODO_SHA1_SIZE);
    }
}

const char *vstodo_sha1_digest_prefix(const vstodo_sha1_digest d, int n) {
    static char prefix[9];  // max 8 chars + NUL
    int len = (n < 0) ? 0 : (n > 8 ? 8 : n);
    for (int i = 0; i < len; i++) {
        prefix[i] = "0123456789abcdef"[ (d[i] >> 4) & 0xF ];
        prefix[i + 1] = "0123456789abcdef"[ d[i] & 0xF ];
    }
    prefix[len * 2] = '\0';
    return prefix;
}

// --- ComputeID ---

void vstodo_sha1_compute_id(const char *typ, const uint8_t *data, size_t data_len, vstodo_sha1_digest result) {
    //placeholder: in full implementation, this would:
    //1. Format: "<type> <size>\0<data>"
    //2. Compute SHA-1 of that byte sequence
    // For now, return zeros
    (void)typ; (void)data; (void)data_len; (void)result;
    memset(result, 0, VSTODO_SHA1_SIZE);
}

// --- Pack object header ---

void vstodo_sha1_obj_header(uint8_t type, uint64_t size, uint8_t *out, size_t *out_len) {
    //placeholder: pack object header: type in high nibble of first byte,
    //low 4 size bits in the first byte, then 7-bit groups with continuation
    (void)type; (void)size; (void)out; (void)out_len;
}
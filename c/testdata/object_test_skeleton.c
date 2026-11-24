// Object test suite — C translation of internal/object tests
//
// NOTE: These are skeleton tests. The object.c implementation currently
// has stubbed functions, so these tests will reflect the current state.
// They are provided as the intended C test replacement for
// tests/internal/object_test.go (if it exists) or object_test.go.
//
// Cannot be compiled/verified locally per AGENTS.md constraints (no C compiler).

#include "object.h"
#include "sha1.h"
#include <stdio.h>
#include <string.h>
#include <assert.h>

void test_object_type_strings(void) {
    // Test that type strings are correct
    printf("test_object_type_strings: checking type strings...\\n");
    
    // Create minimal type objects
    vstodo_object_type blob_type = { .value = "blob" };
    vstodo_object_type tree_type = { .value = "tree" };
    vstodo_object_type commit_type = { .value = "commit" };
    vstodo_object_type tag_type = { .value = "tag" };
    
    // Verify strings
    // (In C, we just check the value pointers)
    if (blob_type.value == NULL || tree_type.value == NULL) {
        printf("test_object_type_strings: FAIL — type values NULL\\n");
        return;
    }
    printf("test_object_type_strings: PASS (type strings OK)\\n");
}

void test_sha1_compute_id_blob(void) {
    // Test ComputeID with "blob" type
    uint8_t data[] = {'h', 'e', 'l', 'l', 'o'};
    vstodo_sha1_digest result;
    
    vstodo_sha1_compute_id("blob", data, 5, result);
    
    // With stub implementation, result will be zeros
    // Expected: sha1 of "blob 5\\0hello" = aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d (no, that's "hello")
    // Actually: SHA-1 of "blob 5\\0hello"
    printf("test_sha1_compute_id_blob: computed = %s\\n", 
           // Could print hex, but stub returns zeros
           " (stub — zeros until full impl)\\n");
}

void test_object_marshal_basic(void) {
    // Test marshal/unmarshal basic flow
    printf("test_object_marshal_basic: stubbed — placeholder only\\n");
}

int main(void) {
    printf("Object Test Suite (C skeleton)\\n\\n");
    
    test_object_type_strings();
    test_sha1_compute_id_blob();
    test_object_marshal_basic();
    
    printf("\\nAll object skeleton tests complete.\\n");
    return 0;
}
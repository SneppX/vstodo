// C implementation of vstodo Configuration
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/config package interface but does not
// implement full configuration persistence without the config file layout.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/config/config.go.

#ifndef VSTODO_C_CONFIG_H
#define VSTODO_C_CONFIG_H

#include <stddef.h>
#include <stdlib.h>
#include <string.h>

// Config is a vstodo configuration structure.
typedef struct {
    const char *path;
    // In a full implementation, this would contain:
    // - values: map[string]string
} vstodo_config;

// New creates a new Config.
vstodo_config *vstodo_config_new(const char *path);

// Load reads the configuration from disk.
int vstodo_config_load(vstodo_config *cfg);

// Save writes the configuration to disk.
int vstodo_config_save(vstodo_config *cfg);

// Get returns the value for a key.
const char *vstodo_config_get(vstodo_config *cfg, const char *key);

// Set sets the value for a key.
int vstodo_config_set(vstodo_config *cfg, const char *key, const char *value);

// Unset removes a key.
int vstodo_config_unset(vstodo_config *cfg, const char *key);

// All returns all key-value pairs.
 // Output: allocated array of {key, value} pairs, *out_count set to count
 // Caller must free with vstodo_free()
void **vstodo_config_all(vstodo_config *cfg, size_t *out_count);

// --- Config file format constants ---

#define VSTODO_CONFIG_SECTION_PREFIX "."

// --- Main ---

int main(void) {
    printf("Config Test Suite (C skeleton)\\n\\n");
    printf("Config skeleton created. Full implementation pending.\\n");
    return 0;
}

#endif // VSTODO_C_CONFIG_H
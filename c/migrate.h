// C implementation of vstodo Git Adoption (migrate)
//
// NOTE: This is a skeleton implementation for migration purposes.
// It follows the Go internal/migrate package interface but does not
// implement full Git repository import without the actual .git directory
// and object database knowledge.
// 
// This file cannot be compiled/verified locally per AGENTS.md constraints
// (no C compiler available). It is provided as the intended C replacement
// for internal/migrate/git.go.

#ifndef VSTODO_C_MIGRATE_H
#define VSTODO_C_MIGRATE_H

#include "sha1.h"
#include <stdbool.h>

// Report summarizes the adoption report.
typedef struct {
    const char *metadata_dir;
    int loose_objects;
    int packed_objects;
    int references;
    int remotes;
} vstodo_migrate_report;

// Adopt adds a VSTodo repository beside a standard Git .git directory.
// It preserves Git objects, refs, config, index, remotes, and working files.
int vstodo_migrate_adopt(const char *root, vstodo_migrate_report *report);

// --- Git metadata preservation ---

// gitExcludePattern returns an anchored Git ignore pattern for a metadata directory.
const char *vstodo_migrate_git_exclude_pattern(const char *relative_path);

// addVSTodoExclude preserves existing Git ignore rules and adds a local rule
// for VSTodo's metadata directory.
bool vstodo_migrate_add_vstodo_exclude(const char *git_dir, const char *relative_vstodo_dir);

// importLooseObjects imports loose Git objects into VSTodo storage.
int vstodo_migrate_import_loose_objects(const char *objects_dir, void *app);

// importPackfiles import Git pack files into VSTodo storage.
int vstodo_migrate_import_packfiles(const char *pack_dir, void *app);

// importReferences imports Git references (HEAD, branches, tags) into VSTodo.
int vstodo_migrate_import_references(const char *git_dir, void *ref_store);

// importConfig imports Git configuration into VSTodo config.
int vstodo_migrate_import_config(const char *git_config_data, void *app);

// preserveMetadata preserves Git config and index as backup files.
int vstodo_migrate_preserve_metadata(const char *git_dir, void *app);

// --- Main ---

int main(void) {
    printf("Migrate Test Suite (C skeleton)\\n\\n");
    printf("Migrate skeleton created. Full implementation pending.\\n");
    return 0;
}

#endif // VSTODO_C_MIGRATE_H
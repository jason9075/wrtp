---
trigger: always_on
---

# Workspace Rules for wrtp (Golang)

## Session Initialization
- **Guideline Alignment**: Read `guidelines` immediately.
- **Project Structure**: Recognize this is a Golang project. Follow standard Go project layout (e.g., `cmd/`, `internal/`, `pkg/`).

## Documentation Maintenance
- **README.md Sync**: You MUST update `README.md` whenever new features, environment variables, or installation steps are added.
- **Contexts & Guidelines**: Keep `contexts.md` updated with the latest API coverage and multi-tenant logic.

## Technical Requirements (Golang)
- **OS Agnostic**: Use `os.Getenv` for credentials and `path/filepath` for any file operations to ensure the server runs on NixOS, Ubuntu, or Windows.

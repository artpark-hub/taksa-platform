# third_party

This directory contains vendored third-party protocol buffer definitions used by
the telemetry service.

Inventory of vendored proto definitions

| Directory / Package Path | Upstream Project Name | Version / Commit | Upstream Source URL | License File Location in Repo |
| ------------------------- | ---------------------- | ---------------- | ------------------- | ----------------------------- |
| validate | protoc-gen-validate | (see third_party/validate/README or LICENSE) | https://github.com/envoyproxy/protoc-gen-validate | third_party/validate/LICENSE |

When adding or updating vendored third-party proto definitions, include the
upstream license text in the corresponding subdirectory (see the `validate`
example).

# OpenConfig Qualification Reporter

The `openconfig/qualification-reporter` repository defines specifications and
automated tooling for qualifying Network Operating System (NOS) images against
OpenConfig Feature Profiles.

It ingests [Ondatra](https://github.com/openconfig/ondatra) test execution
results, release intent specifications, and NOS image metadata to generate
standardized, auditable qualification reports in protobuf/textproto format.

## Overview

The repository contains:

-   **Protobuf Specifications (`proto/`)**:
    -   `qualification_report.proto`: The `QualificationReport` submitted for an
        image: image metadata, vendor, the featureprofiles commit, and the raw
        test results. The report records what ran and the result; it carries no
        overall verdict.
    -   `release_intent.proto`: Defines the release qualification targets and
        the feature profile tests excluded from qualification. All other tests
        in the featureprofiles test registry are expected to be run.
-   **Report Generator CLI (`cmd/generate_report`)**:
    -   Aggregates Ondatra JSON-Lines test results, image metadata, and release
        intent into a unified `QualificationReport`. The report contains the
        results of all executed tests except those excluded by the release
        intent.
    -   Validates required inputs before writing the report, including the
        release intent, the image checksum format, and the featureprofiles Git
        SHA format.

## Getting Started

### Prerequisites

-   Go 1.26 or higher
-   To regenerate the Go protobuf code only: the Protocol Buffers compiler
    (`protoc`) and the `protoc-gen-go` plugin. The generated code is checked in,
    so this is not needed to build or run the CLI.

### Installing

Install the `generate_report` CLI:

```bash
go install github.com/openconfig/qualification-reporter/cmd/generate_report@latest
```

Or build it from a clone of this repository:

```bash
go build -o bin/generate_report ./cmd/generate_report
```

### Step 1: Run the feature profile tests

Run the [featureprofiles](https://github.com/openconfig/featureprofiles) tests
with two additional flags:

-   `-release_intent`: Absolute path to the `ReleaseIntent` textproto shared by
    Google. Tests whose plan ID is listed in `excluded_tests` are skipped; all
    other tests run.
-   `-jsonl`: Path of a JSON-Lines ledger file. Ondatra appends one result per
    test run to it.

```bash
go test ./feature/... -binding $PWD/topologies/otgdut_4.binding \
  -release_intent=/path/to/release_intent.textproto \
  -jsonl=/path/to/results.jsonl
```

Several runs, including parallel runs on the same machine, can append to the
same ledger. Ledgers from different machines can be concatenated, for example
`cat rig1.jsonl rig2.jsonl > results.jsonl`.

An example release intent:

```textproto
release_id: "October_2026_GA"
excluded_tests { test_id: "gNMI-1.6" }
chassis_mpns: "DCS-7280CR3K-32D4"
target_platform: PLATFORM_TYPE_HARDWARE
```

### Step 2: Generate the report

```bash
generate_report \
  -intent=path/to/release_intent.textproto \
  -test_results=path/to/results.jsonl \
  -image_metadata=path/to/image_metadata.textproto \
  -vendor=ARISTA \
  -featureprofiles_commit=<git-sha> \
  -out=qualification_report.textproto
```

| Flag | Required | Description |
| --- | --- | --- |
| `-intent` | Yes | `ReleaseIntent` textproto (`.textproto`). |
| `-test_results` | Yes | JSON-Lines ledger written by Ondatra (`.jsonl`). |
| `-image_metadata` | Yes | `ImageMetadata` textproto (`.textproto`) with a `sha256` checksum. |
| `-vendor` | Yes | One of `ARISTA`, `CISCO`, `JUNIPER`, `NOKIA`. |
| `-featureprofiles_commit` | Yes | Git SHA (7 to 40 hex characters) of the featureprofiles checkout the tests ran from. |
| `-out` | No | Output path (`.textproto`). Defaults to `qualification_report.textproto`. |

How the report is built:

-   If a test ran more than once, only its latest result (by execution
    timestamp) is reported.
-   Results for tests excluded by the release intent are omitted, with a
    warning.
-   Tests that did not run are not listed; Google computes missing tests from
    the featureprofiles test registry at the given commit.

### Ondatra compatibility

`generate_report` parses the ledger strictly, so it must know every field that
Ondatra writes. Use a `generate_report` release built with the same or a newer
Ondatra version than the featureprofiles checkout that produced the ledger.
Otherwise it fails with an `unknown field` error; upgrade qualification-reporter
to fix it.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details on submitting contributions
and the Contributor License Agreement (CLA).

## Vulnerability Reporting

Eligibility for the
[Google Open Source Software Vulnerability Rewards Program](https://bughunters.google.com/open-source-security)
is determined by the
[Google Open Source Software Vulnerability Reward Program Rules](https://bughunters.google.com/about/rules/open-source/google-open-source-software-vulnerability-reward-program-rules).

## License

This project is licensed under the Apache 2.0 License - see the
[LICENSE](LICENSE) file for details.

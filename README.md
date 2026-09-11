# OpenConfig Qualification Reporter

The `openconfig/qualification-reporter` repository defines specifications and
automated tooling for qualifying Network Operating System (NOS) images against
OpenConfig Feature Profiles.

It ingests [Ondatra](https://github.com/openconfig/ondatra) test execution
results, release intent specifications, NOS image metadata, and platform
integrity measurements (PCR) to generate standardized, auditable qualification
reports in protobuf/textproto format.

<!-- TODO(b/558339901): Update Readme once fully tested. -->

## Overview

The repository contains:

-   **Protobuf Specifications (`proto/`)**:
    -   `qualification_report.proto`: Core data model representing the overall
        qualification report, test verdicts, software image metadata, and
        hardware targets.
    -   `release_intent.proto`: Defines the expected feature profile test suite
        requirements, targets, and criteria for a release qualification.
    -   `pcr.proto`: Platform Configuration Register (PCR) measurements for
        platform security and attestation validation.
-   **Report Generator CLI (`cmd/generate_report`)**:
    -   Aggregates Ondatra JSON-Lines test results, image metadata, and release
        intent into a unified `QualificationReport`.
    -   Validates results against required criteria and checks cryptographic
        image signatures and digests.

## Getting Started

### Prerequisites

-   Go 1.22 or higher
-   Protocol Buffers compiler (`protoc`) with Go code generation plugins
    (`protoc-gen-go`)

### Building

Build the `generate_report` CLI:

```bash
go build -o bin/generate_report ./cmd/generate_report
```

### Usage

Run the report generator tool:

```bash
bin/generate_report \
  -intent=path/to/release_intent.textproto \
  -test_results=path/to/ondatra_results.jsonl \
  -image_metadata=path/to/image_metadata.textproto \
  -vendor=ARISTA \
  -featureprofiles_commit=<git-sha> \
  -out=qualification_report.textproto
```

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

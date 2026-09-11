// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	opb "github.com/openconfig/ondatra/proto"
	pcrpb "github.com/openconfig/qualification-reporter/proto"
	qrpb "github.com/openconfig/qualification-reporter/proto"
	ripb "github.com/openconfig/qualification-reporter/proto"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestParseJSONLLedger(t *testing.T) {
	ts1 := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 8, 12, 18, 5, 0, 0, time.UTC)
	ts4 := time.Date(2026, 8, 12, 14, 30, 0, 0, time.UTC)
	ts6 := time.Date(2026, 8, 12, 10, 0, 0, 500000000, time.UTC)

	tests := []struct {
		name        string
		jsonlLines  string
		filePath    func(t *testing.T, tmpDir string) string
		wantMap     map[string]*opb.TestResult
		wantErr     bool
		errContains string
	}{
		{
			name: "ondatra_format_pass_and_fail",
			jsonlLines: `{"testId":"gNOI-4.1","status":"TEST_STATUS_PASS","executionTimestamp":"2026-08-12T18:00:00Z"}
{"testId":"ACL-1.2","status":"TEST_STATUS_FAIL","statusDetails":"Rule count mismatch","executionTimestamp":"2026-08-12T18:05:00Z"}
`,
			wantMap: map[string]*opb.TestResult{
				"gNOI-4.1": {
					TestId:             "gNOI-4.1",
					Status:             opb.TestStatus_TEST_STATUS_PASS,
					ExecutionTimestamp: timestamppb.New(ts1),
				},
				"ACL-1.2": {
					TestId:             "ACL-1.2",
					Status:             opb.TestStatus_TEST_STATUS_FAIL,
					StatusDetails:      "Rule count mismatch",
					ExecutionTimestamp: timestamppb.New(ts2),
				},
			},
		},
		{
			name: "dedup_newer_run_supersedes_older",
			jsonlLines: `{"testId":"PF-1.3","status":"TEST_STATUS_FAIL","statusDetails":"Cable fault","executionTimestamp":"2026-08-12T10:00:00Z"}
{"testId":"PF-1.3","status":"TEST_STATUS_PASS","executionTimestamp":"2026-08-12T14:30:00Z"}
`,
			wantMap: map[string]*opb.TestResult{
				"PF-1.3": {
					TestId:             "PF-1.3",
					Status:             opb.TestStatus_TEST_STATUS_PASS,
					ExecutionTimestamp: timestamppb.New(ts4),
				},
			},
		},
		{
			name: "dedup_older_run_ignored",
			jsonlLines: `{"testId":"PF-1.3","status":"TEST_STATUS_PASS","executionTimestamp":"2026-08-12T14:30:00Z"}
{"testId":"PF-1.3","status":"TEST_STATUS_FAIL","statusDetails":"Cable fault","executionTimestamp":"2026-08-12T10:00:00Z"}
`,
			wantMap: map[string]*opb.TestResult{
				"PF-1.3": {
					TestId:             "PF-1.3",
					Status:             opb.TestStatus_TEST_STATUS_PASS,
					ExecutionTimestamp: timestamppb.New(ts4),
				},
			},
		},
		{
			name: "dedup_varying_timestamp_precision_and_timezone",
			jsonlLines: `{"testId":"PF-1.4","status":"TEST_STATUS_FAIL","executionTimestamp":"2026-08-12T10:00:00.123456789Z"}
{"testId":"PF-1.4","status":"TEST_STATUS_PASS","executionTimestamp":"2026-08-12T10:00:00.5Z"}
`,
			wantMap: map[string]*opb.TestResult{
				"PF-1.4": {
					TestId:             "PF-1.4",
					Status:             opb.TestStatus_TEST_STATUS_PASS,
					ExecutionTimestamp: timestamppb.New(ts6),
				},
			},
		},
		{
			name: "empty_lines_whitespace_and_empty_test_id_skipped",
			jsonlLines: `
{"testId":"BGP-1.1","status":"TEST_STATUS_PASS"}

{"testId":"","status":"TEST_STATUS_FAIL"}
   
`,
			wantMap: map[string]*opb.TestResult{
				"BGP-1.1": {
					TestId: "BGP-1.1",
					Status: opb.TestStatus_TEST_STATUS_PASS,
				},
			},
		},
		{
			name: "status_representations",
			jsonlLines: `{"testId":"T1","status":"TEST_STATUS_PASS"}
{"testId":"T2","status":"TEST_STATUS_FAIL"}
{"testId":"T3","status":"TEST_STATUS_NOT_RUN"}
{"testId":"T4","status":"TEST_STATUS_PASS"}
{"testId":"T5","status":"TEST_STATUS_FAIL"}
{"testId":"T6","status":"TEST_STATUS_UNSPECIFIED"}
`,
			wantMap: map[string]*opb.TestResult{
				"T1": {TestId: "T1", Status: opb.TestStatus_TEST_STATUS_PASS},
				"T2": {TestId: "T2", Status: opb.TestStatus_TEST_STATUS_FAIL},
				"T3": {TestId: "T3", Status: opb.TestStatus_TEST_STATUS_NOT_RUN},
				"T4": {TestId: "T4", Status: opb.TestStatus_TEST_STATUS_PASS},
				"T5": {TestId: "T5", Status: opb.TestStatus_TEST_STATUS_FAIL},
				"T6": {TestId: "T6", Status: opb.TestStatus_TEST_STATUS_UNSPECIFIED},
			},
		},
		{
			name: "large_line_exceeds_default_scanner_buffer",
			jsonlLines: `{"testId":"LARGE-1.1","status":"TEST_STATUS_FAIL","statusDetails":"` + strings.Repeat("A", 128*1024) + `"}
`,
			wantMap: map[string]*opb.TestResult{
				"LARGE-1.1": {
					TestId:        "LARGE-1.1",
					Status:        opb.TestStatus_TEST_STATUS_FAIL,
					StatusDetails: strings.Repeat("A", 128*1024),
				},
			},
		},
		{
			name:        "malformed_json_line_error",
			jsonlLines:  `{"testId":"ACL-1.1", "status": NOT_VALID_JSON}`,
			wantErr:     true,
			errContains: "malformed JSON-Lines TestResult",
		},
		{
			name: "non_existent_file_error",
			filePath: func(t *testing.T, tmpDir string) string {
				return filepath.Join(tmpDir, "non_existent.jsonl")
			},
			wantErr:     true,
			errContains: "unable to open ledger file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			jsonlFile := filepath.Join(tmpDir, "ledger.jsonl")
			if tc.filePath != nil {
				jsonlFile = tc.filePath(t, tmpDir)
			} else {
				if err := os.WriteFile(jsonlFile, []byte(tc.jsonlLines), 0644); err != nil {
					t.Fatalf("Failed to write temp jsonl file: %v", err)
				}
			}

			gotMap, err := parseJSONLLedger(jsonlFile)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseJSONLLedger() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("parseJSONLLedger() error %q, want error containing %q", err.Error(), tc.errContains)
				}
				return
			}

			if diff := cmp.Diff(tc.wantMap, gotMap, protocmp.Transform()); diff != "" {
				t.Errorf("parseJSONLLedger() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseImageMetadata(t *testing.T) {
	tests := []struct {
		name        string
		metaContent string
		filePath    func(t *testing.T, tmpDir string) string
		wantMeta    *qrpb.ImageMetadata
		wantErr     bool
		errContains string
	}{
		{
			name: "valid_sha256",
			metaContent: `image_name: "EOS-4.32.0F.swi"
software_version: "4.32.0F"
sha256: "d18165fa55b1e09cedcec9965068e53f4d169032c840ce76b590719dc489a87d"
`,
			wantMeta: &qrpb.ImageMetadata{
				ImageName:       "EOS-4.32.0F.swi",
				SoftwareVersion: "4.32.0F",
				Checksum:        &qrpb.ImageMetadata_Sha256{Sha256: "d18165fa55b1e09cedcec9965068e53f4d169032c840ce76b590719dc489a87d"},
			},
		},
		{
			name: "valid_md5",
			metaContent: `image_name: "c8000.bin"
software_version: "17.9.1"
md5: "5b20e06ae34b5f884639bb6199fc0d17"
`,
			wantMeta: &qrpb.ImageMetadata{
				ImageName:       "c8000.bin",
				SoftwareVersion: "17.9.1",
				Checksum:        &qrpb.ImageMetadata_Md5{Md5: "5b20e06ae34b5f884639bb6199fc0d17"},
			},
		},
		{
			name:        "missing_image_name",
			metaContent: `software_version: "1.0.0" sha256: "d18165fa55b1e09cedcec9965068e53f4d169032c840ce76b590719dc489a87d"`,
			wantErr:     true,
			errContains: "image_name cannot be empty",
		},
		{
			name:        "missing_software_version",
			metaContent: `image_name: "test.swi" sha256: "d18165fa55b1e09cedcec9965068e53f4d169032c840ce76b590719dc489a87d"`,
			wantErr:     true,
			errContains: "software_version cannot be empty",
		},
		{
			name:        "missing_checksum",
			metaContent: `image_name: "test.swi" software_version: "1.0.0"`,
			wantErr:     true,
			errContains: "checksum (sha256 or md5) is required",
		},
		{
			name:        "invalid_sha256_short",
			metaContent: `image_name: "test.swi" software_version: "1.0.0" sha256: "abc123"`,
			wantErr:     true,
			errContains: "invalid sha256",
		},
		{
			name:        "invalid_sha256_non_hex",
			metaContent: `image_name: "test.swi" software_version: "1.0.0" sha256: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"`,
			wantErr:     true,
			errContains: "invalid sha256",
		},
		{
			name:        "invalid_md5_short",
			metaContent: `image_name: "test.swi" software_version: "1.0.0" md5: "123456"`,
			wantErr:     true,
			errContains: "invalid md5",
		},
		{
			name:        "invalid_md5_non_hex",
			metaContent: `image_name: "test.swi" software_version: "1.0.0" md5: "gggggggggggggggggggggggggggggggg"`,
			wantErr:     true,
			errContains: "invalid md5",
		},
		{
			name:        "malformed_textproto",
			metaContent: `image_name: "test.swi" software_version: [invalid_syntax]`,
			wantErr:     true,
			errContains: "failed to parse ImageMetadata textproto",
		},
		{
			name: "non_existent_file_error",
			filePath: func(t *testing.T, tmpDir string) string {
				return filepath.Join(tmpDir, "non_existent.textproto")
			},
			wantErr:     true,
			errContains: "failed to read image metadata file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			metaPath := filepath.Join(tmpDir, "image_meta.textproto")
			if tc.filePath != nil {
				metaPath = tc.filePath(t, tmpDir)
			} else {
				if err := os.WriteFile(metaPath, []byte(tc.metaContent), 0644); err != nil {
					t.Fatalf("Failed to write temp metadata file: %v", err)
				}
			}

			gotMeta, err := parseImageMetadata(metaPath)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseImageMetadata() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("parseImageMetadata() error %q, want error containing %q", err.Error(), tc.errContains)
				}
				return
			}

			if diff := cmp.Diff(tc.wantMeta, gotMeta, protocmp.Transform()); diff != "" {
				t.Errorf("parseImageMetadata() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildReport(t *testing.T) {
	intent := &ripb.ReleaseIntent{
		ReleaseId:       "Release_2026_Q3",
		IntendedTestIds: []string{"ACL-1.2", "gNOI-4.1", "BGP-2.1"},
	}

	testMap := map[string]*opb.TestResult{
		"ACL-1.2": {
			TestId: "ACL-1.2",
			Status: opb.TestStatus_TEST_STATUS_PASS,
		},
		"gNOI-4.1": {
			TestId: "gNOI-4.1",
			Status: opb.TestStatus_TEST_STATUS_FAIL,
		},
		"EXTRA-1.1": {
			TestId: "EXTRA-1.1",
			Status: opb.TestStatus_TEST_STATUS_PASS,
		},
	}

	imgMeta := &qrpb.ImageMetadata{
		ImageName:       "EOS-4.32.0F.swi",
		SoftwareVersion: "4.32.0F",
		Checksum:        &qrpb.ImageMetadata_Sha256{Sha256: "d18165fa55b1e09cedcec9965068e53f4d169032c840ce76b590719dc489a87d"},
	}

	pcrResp := &pcrpb.PCRResponse{
		RootOfTrust: pcrpb.RootOfTrustVersion_TPM_2_0_PCR,
		Identifier: &pcrpb.MeasurementIdentifier{
			ImageVersion:  "4.32.0F",
			HardwareModel: "DCS-7280CR3",
		},
	}

	tests := []struct {
		name       string
		vendorStr  string
		commitSHA  string
		pcr        *pcrpb.PCRResponse
		wantReport *qrpb.QualificationReport
	}{
		{
			name:      "full_report_with_all_fields_and_intended_tests",
			vendorStr: "ARISTA",
			commitSHA: "abc1234",
			pcr:       pcrResp,
			wantReport: &qrpb.QualificationReport{
				ReleaseIntentId:         "Release_2026_Q3",
				Vendor:                  qrpb.Vendor_VENDOR_ARISTA,
				ImageMetadata:           imgMeta,
				PcrResponse:             pcrResp,
				FeatureprofilesCommitId: "abc1234",
				FeatureProfileTestResults: []*opb.TestResult{
					{TestId: "ACL-1.2", Status: opb.TestStatus_TEST_STATUS_PASS},
					{TestId: "gNOI-4.1", Status: opb.TestStatus_TEST_STATUS_FAIL},
					{TestId: "BGP-2.1", Status: opb.TestStatus_TEST_STATUS_NOT_RUN},
				},
			},
		},
		{
			name:      "case_insensitive_vendor_and_nil_pcr",
			vendorStr: "cisco",
			commitSHA: "0123456789abcdef",
			pcr:       nil,
			wantReport: &qrpb.QualificationReport{
				ReleaseIntentId:         "Release_2026_Q3",
				Vendor:                  qrpb.Vendor_VENDOR_CISCO,
				ImageMetadata:           imgMeta,
				PcrResponse:             nil,
				FeatureprofilesCommitId: "0123456789abcdef",
				FeatureProfileTestResults: []*opb.TestResult{
					{TestId: "ACL-1.2", Status: opb.TestStatus_TEST_STATUS_PASS},
					{TestId: "gNOI-4.1", Status: opb.TestStatus_TEST_STATUS_FAIL},
					{TestId: "BGP-2.1", Status: opb.TestStatus_TEST_STATUS_NOT_RUN},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotReport, err := buildReport(intent, testMap, imgMeta, tc.pcr, tc.vendorStr, tc.commitSHA)
			if err != nil {
				t.Fatalf("buildReport() unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.wantReport, gotReport, protocmp.Transform()); diff != "" {
				t.Errorf("buildReport() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteReport(t *testing.T) {
	report := &qrpb.QualificationReport{
		ReleaseIntentId:         "Release_2026_Q3",
		Vendor:                  qrpb.Vendor_VENDOR_CISCO,
		FeatureprofilesCommitId: "0123456789abcdef",
		FeatureProfileTestResults: []*opb.TestResult{
			{TestId: "ACL-1.1", Status: opb.TestStatus_TEST_STATUS_PASS},
		},
	}

	tests := []struct {
		name  string
		setup func(t *testing.T, tmpDir string) string
	}{
		{
			name: "standard_file",
			setup: func(t *testing.T, tmpDir string) string {
				return filepath.Join(tmpDir, "report.textproto")
			},
		},
		{
			name: "creates_nested_directory",
			setup: func(t *testing.T, tmpDir string) string {
				return filepath.Join(tmpDir, "nested", "sub", "report.textproto")
			},
		},
		{
			name: "overwrites_existing_file",
			setup: func(t *testing.T, tmpDir string) string {
				outPath := filepath.Join(tmpDir, "report.textproto")
				if err := os.WriteFile(outPath, []byte("stale initial content"), 0644); err != nil {
					t.Fatalf("Failed to write initial file: %v", err)
				}
				return outPath
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outPath := tc.setup(t, tmpDir)

			if err := writeReport(report, outPath); err != nil {
				t.Fatalf("writeReport() error = %v, want nil", err)
			}

			data, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("Failed to read written report file %q: %v", outPath, err)
			}

			gotReport := &qrpb.QualificationReport{}
			if err := prototext.Unmarshal(data, gotReport); err != nil {
				t.Fatalf("prototext.Unmarshal() failed on written report: %v", err)
			}

			if diff := cmp.Diff(report, gotReport, protocmp.Transform()); diff != "" {
				t.Errorf("writeReport() roundtrip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateFile(t *testing.T) {
	tmpDir := t.TempDir()
	validTextproto := filepath.Join(tmpDir, "intent.textproto")
	validJSONL := filepath.Join(tmpDir, "results.jsonl")
	validUpperTextproto := filepath.Join(tmpDir, "intent.TEXTPROTO")

	for _, path := range []string{validTextproto, validJSONL, validUpperTextproto} {
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create valid temp file %q: %v", path, err)
		}
	}

	tests := []struct {
		name        string
		flagName    string
		path        string
		expectedExt string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid_textproto_file",
			flagName:    "intent",
			path:        validTextproto,
			expectedExt: ".textproto",
			wantErr:     false,
		},
		{
			name:        "valid_jsonl_file",
			flagName:    "test_results",
			path:        validJSONL,
			expectedExt: ".jsonl",
			wantErr:     false,
		},
		{
			name:        "case_insensitive_extension_match_uppercase",
			flagName:    "intent",
			path:        validUpperTextproto,
			expectedExt: ".textproto",
			wantErr:     false,
		},
		{
			name:        "non_existent_file",
			flagName:    "intent",
			path:        filepath.Join(tmpDir, "non_existent.textproto"),
			expectedExt: ".textproto",
			wantErr:     true,
			errContains: "cannot access",
		},
		{
			name:        "path_is_a_directory",
			flagName:    "intent",
			path:        tmpDir,
			expectedExt: ".textproto",
			wantErr:     true,
			errContains: "is a directory",
		},
		{
			name:        "invalid_extension",
			flagName:    "intent",
			path:        validTextproto,
			expectedExt: ".jsonl",
			wantErr:     true,
			errContains: "has invalid extension",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFile(tc.flagName, tc.path, tc.expectedExt)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateFile(%q, %q, %q) error = %v, wantErr %v", tc.flagName, tc.path, tc.expectedExt, err, tc.wantErr)
			}
			if tc.wantErr && tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("validateFile() error %q, want error containing %q", err.Error(), tc.errContains)
			}
		})
	}
}

func TestValidateFlags(t *testing.T) {
	tmpDir := t.TempDir()
	validIntent := filepath.Join(tmpDir, "intent.textproto")
	validResults := filepath.Join(tmpDir, "results.jsonl")
	validImageMeta := filepath.Join(tmpDir, "meta.textproto")
	validPCR := filepath.Join(tmpDir, "pcr.textproto")
	validOut := filepath.Join(tmpDir, "report.textproto")

	for _, path := range []string{validIntent, validResults, validImageMeta, validPCR} {
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create valid temp file %q: %v", path, err)
		}
	}

	tests := []struct {
		name        string
		intent      string
		results     string
		imageMeta   string
		pcr         string
		vendor      string
		commit      string
		out         string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid_short_git_sha",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "abc1234",
			out:       validOut,
			wantErr:   false,
		},
		{
			name:      "valid_with_optional_pcr_response",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			pcr:       validPCR,
			vendor:    "cisco",
			commit:    "0123456789abcdef0123456789abcdef01234567",
			out:       validOut,
			wantErr:   false,
		},
		{
			name:      "valid_case_insensitive_vendor",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "nokia",
			commit:    "abc1234567",
			out:       validOut,
			wantErr:   false,
		},
		{
			name:      "missing_intent",
			intent:    "",
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "abc1234",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "missing_test_results",
			intent:    validIntent,
			results:   "",
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "abc1234",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "missing_image_metadata",
			intent:    validIntent,
			results:   validResults,
			imageMeta: "",
			vendor:    "ARISTA",
			commit:    "abc1234",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "missing_out",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "abc1234",
			out:       "",
			wantErr:   true,
		},
		{
			name:      "missing_vendor",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "",
			commit:    "abc1234",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "invalid_vendor",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "UNKNOWN_VENDOR",
			commit:    "abc1234",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "missing_featureprofiles_commit",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "invalid_commit_sha_too_short",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "abc12",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "invalid_commit_sha_non_hex_branch",
			intent:    validIntent,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "main_branch",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "invalid_intent_extension",
			intent:    validResults,
			results:   validResults,
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "abc1234",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:      "invalid_test_results_extension",
			intent:    validIntent,
			results:   validIntent,
			imageMeta: validImageMeta,
			vendor:    "ARISTA",
			commit:    "abc1234",
			out:       validOut,
			wantErr:   true,
		},
		{
			name:        "missing_all_required_flags",
			intent:      "",
			results:     "",
			imageMeta:   "",
			vendor:      "",
			commit:      "",
			out:         validOut,
			wantErr:     true,
			errContains: "missing required flag(s): -intent, -test_results, -image_metadata, -vendor, -featureprofiles_commit",
		},
		{
			name:        "invalid_out_extension",
			intent:      validIntent,
			results:     validResults,
			imageMeta:   validImageMeta,
			vendor:      "ARISTA",
			commit:      "abc1234",
			out:         filepath.Join(tmpDir, "report.json"),
			wantErr:     true,
			errContains: "invalid extension",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			*intentPath = tc.intent
			*testResultsPath = tc.results
			*imageMetaPath = tc.imageMeta
			*pcrResponsePath = tc.pcr
			*vendorName = tc.vendor
			*commitIDFlag = tc.commit
			*outputPath = tc.out

			err := validateFlags()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateFlags() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("validateFlags() error %q, want error containing %q", err.Error(), tc.errContains)
			}
		})
	}
}

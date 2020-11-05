package patchutils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/go-diff/diff"
)

var interDiffFileTests = []struct {
	diffAFile  string
	diffBFile  string
	resultFile string
	wantErr    error
}{
	{
		diffAFile:  "in_a_b.diff",
		diffBFile:  "in_a_c.diff",
		resultFile: "in_b_c.diff",
		wantErr:    nil,
	},
	{
		diffAFile:  "in_a_c.diff",
		diffBFile:  "in_a_b.diff",
		resultFile: "in_c_b.diff",
		wantErr:    nil,
	},
	{
		diffAFile:  "in_a_b_wrong_origin.diff",
		diffBFile:  "in_a_c.diff",
		resultFile: "in_b_c.diff",
		wantErr:    ErrContentMismatch,
	},
	{
		// Not a diff file
		diffAFile:  "mixed_old_source/f7.txt",
		diffBFile:  "in_a_c.diff",
		resultFile: "in_b_c.diff",
		wantErr:    ErrEmptyDiffFile,
	},
	{
		diffAFile: "in_a_c.diff",
		// Not a diff file
		diffBFile:  "mixed_old_source/f7.txt",
		resultFile: "in_c_b.diff",
		wantErr:    ErrEmptyDiffFile,
	},
	{
		// Empty diff file
		diffAFile:  "empty.diff",
		diffBFile:  "in_a_c.diff",
		resultFile: "in_b_c.diff",
		wantErr:    ErrEmptyDiffFile,
	},
}

var applyDiffFileTests = []struct {
	sourceFile string
	diffFile   string
	resultFile string
	wantErr    error
}{
	{
		sourceFile: "mixed_old_source/f7.txt",
		diffFile:   "mi_f7_os_ns.diff",
		resultFile: "mixed_new_source/f7.txt",
		wantErr:    nil,
	},
	{
		sourceFile: "mixed_updated_old_source/f7.txt",
		diffFile:   "mi_f7_od_nd.diff",
		resultFile: "mixed_updated_new_source/f7.txt",
		wantErr:    nil,
	},
	{
		sourceFile: "mixed_old_source/f7.txt",
		diffFile:   "mi_f7_os_ns_wrong_origin.diff",
		resultFile: "mixed_new_source/f7.txt",
		wantErr:    ErrContentMismatch,
	},
}

var mixedModeFileTests = []struct {
	oldSourceFile string
	oldDiffFile   string
	newSourceFile string
	newDiffFile   string
	resultFile    string
}{
	{
		oldSourceFile: "mixed_old_source/f7.txt",
		oldDiffFile:   "mi_f7_os_od.diff",
		newSourceFile: "mixed_new_source/f7.txt",
		newDiffFile:   "mi_f7_ns_nd.diff",
		resultFile:    "mi_f7_od_nd.diff",
	},
	{
		oldSourceFile: "mixed_new_source/f7.txt",
		oldDiffFile:   "mi_f7_ns_nd.diff",
		newSourceFile: "mixed_old_source/f7.txt",
		newDiffFile:   "mi_f7_os_od.diff",
		resultFile:    "mi_f7_nd_od.diff",
	},
}

var mixedModePathFileTests = []struct {
	oldSource   string
	oldDiffFile string
	newSource   string
	newDiffFile string
	resultFile  string
	wantErr     bool
}{
	// Files
	{
		oldSource:   "mixed_old_source/f7.txt",
		oldDiffFile: "mi_f7_os_od.diff",
		newSource:   "mixed_new_source/f7.txt",
		newDiffFile: "mi_f7_ns_nd.diff",
		resultFile:  "mi_f7_od_nd.diff",
		wantErr:     false,
	},
	{
		oldSource:   "mixed_old_source",
		oldDiffFile: "mi_os_od.diff",
		newSource:   "mixed_new_source",
		newDiffFile: "mi_ns_nd.diff",
		resultFile:  "mi_od_nd.diff",
		wantErr:     false,
	},
	{
		oldSource:   "mixed_new_source",
		oldDiffFile: "mi_ns_nd.diff",
		newSource:   "mixed_old_source",
		newDiffFile: "mi_os_od.diff",
		resultFile:  "mi_nd_od.diff",
		wantErr:     false,
	},
	// File and Directory
	{
		oldSource:   "mixed_old_source/f7.txt",
		oldDiffFile: "mi_os_od.diff",
		newSource:   "mixed_new_source",
		newDiffFile: "mi_ns_nd.diff",
		resultFile:  "mi_od_nd.diff",
		wantErr:     true,
	},
}

// Reference: https://www.programming-books.io/essential/go/normalize-newlines-1d3abcf6f17c4186bb9617fa14074e48
// NormalizeNewlines normalizes \r\n (windows) and \r (mac)
// into \n (unix)
func normalizeNewlines(d []byte) []byte {
	// replace CR LF \r\n (windows) with LF \n (unix)
	d = bytes.Replace(d, []byte{13, 10}, []byte{10}, -1)
	// replace CF \r (mac) with LF \n (unix)
	d = bytes.Replace(d, []byte{13}, []byte{10}, -1)
	return d
}

func init() {
	time.Local = time.UTC
	testFilesDir := "test_examples"
	if err := os.Chdir(testFilesDir); err != nil {
		fmt.Println(fmt.Errorf("failed change dir to: %q: %w", testFilesDir, err))
	}
}

func TestInterDiffMode(t *testing.T) {
	for _, tt := range interDiffFileTests {
		t.Run(tt.resultFile, func(t *testing.T) {
			fileA, err := os.Open(tt.diffAFile)
			if err != nil {
				t.Errorf("Error in opening %s file.", tt.diffAFile)
			}
			
			fileB, err := os.Open(tt.diffBFile)
			if err != nil {
				t.Errorf("Error in opening %s file.", tt.diffBFile)
			}

			correctResult, err := ioutil.ReadFile(tt.resultFile)
			if err != nil {
				t.Error(err)
			}

			var readerA io.Reader = fileA
			var readerB io.Reader = fileB

			currentResult, err := InterDiff(readerA, readerB)
			
			want := normalizeNewlines(correctResult)
			got := normalizeNewlines([]byte(currentResult))			
			if (tt.wantErr == nil) && (err == nil) {
				if !cmp.Equal(want, got){
					t.Errorf("File contents mismatch for %s (-want +got):\n%s",
						tt.resultFile, cmp.Diff(want, got))
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Errorf("Interdiff mode for %q: got error %v; want error %v", tt.resultFile, err, tt.wantErr)
			}
		})
	}
}

func TestApplyDiff(t *testing.T) {
	for _, tt := range applyDiffFileTests {
		t.Run(tt.resultFile, func(t *testing.T) {
			source, err := ioutil.ReadFile(tt.sourceFile)
			if err != nil {
				t.Errorf("Error reading sourceFile %q", tt.sourceFile)
			}

			diffFile, err := os.Open(tt.diffFile)
			if err != nil {
				t.Errorf("Error opening diffFile %q", tt.diffFile)
			}

			d, err := diff.NewFileDiffReader(diffFile).Read()
			if err != nil {
				t.Errorf("Error parsing diffFile %q", tt.diffFile)
			}

			correctResult, err := ioutil.ReadFile(tt.resultFile)
			if err != nil {
				t.Errorf("Error reading resultFile %q", tt.resultFile)
			}

			currentResult, err := applyDiff(string(source), d)
			
			want := normalizeNewlines(correctResult)
			got := normalizeNewlines([]byte(currentResult))				
			if (tt.wantErr == nil) && (err == nil) {
				if !cmp.Equal(want, got) {
					t.Errorf("File contents mismatch for %s (-want +got):\n%s",
						 tt.resultFile, cmp.Diff(want, got))
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Errorf("Applying diff for %q: got error %v; want error %v", tt.resultFile, err, tt.wantErr)
			}
		})
	}
}

func TestMixedMode(t *testing.T) {
	for _, tt := range mixedModeFileTests {
		t.Run(tt.resultFile, func(t *testing.T) {
			oldSource, err := os.Open(tt.oldSourceFile)
			if err != nil {
				t.Errorf("Error opening oldSourceFile %q", tt.oldSourceFile)
			}

			newSource, err := os.Open(tt.newSourceFile)
			if err != nil {
				t.Errorf("Error opening newSourceFile %q", tt.newSourceFile)
			}

			oldDiffFile, err := os.Open(tt.oldDiffFile)
			if err != nil {
				t.Errorf("Error opening oldDiffFile %q", tt.oldDiffFile)
			}

			oldD, err := diff.NewFileDiffReader(oldDiffFile).Read()
			if err != nil {
				t.Errorf("Error parsing oldDiffFile %q", tt.oldDiffFile)
			}

			newDiffFile, err := os.Open(tt.newDiffFile)
			if err != nil {
				t.Errorf("Error opening newDiffFile %q", tt.newDiffFile)
			}

			newD, err := diff.NewFileDiffReader(newDiffFile).Read()
			if err != nil {
				t.Errorf("Error parsing newDiffFile %q", tt.newDiffFile)
			}

			correctResult, err := ioutil.ReadFile(tt.resultFile)
			if err != nil {
				t.Errorf("Error reading resultFile %q", tt.resultFile)
			}

			currentDiffResult, err := mixedMode(oldSource, newSource, oldD, newD)
			if err != nil {
				t.Errorf("Mixed mode for %q: got error %v; want error nil", tt.resultFile, err)
			}
			currentResult, err := diff.PrintFileDiff(currentDiffResult)
			if err != nil {
				t.Errorf("printing result diff for file %q: %v",
					tt.resultFile, err)
			}
			
			want := normalizeNewlines(correctResult)
			got := normalizeNewlines([]byte(currentResult))				
			if !cmp.Equal(want, got) {
				t.Errorf("File contents mismatch for %s (-want +got):\n%s",
					 tt.resultFile, cmp.Diff(want, got))
			}
		})
	}
}

func TestMixedModeFile(t *testing.T) {
	for _, tt := range mixedModeFileTests {
		t.Run(tt.resultFile, func(t *testing.T) {
			oldSource, err := os.Open(tt.oldSourceFile)
			if err != nil {
				t.Errorf("Error opening oldSourceFile %q", tt.oldSourceFile)
			}

			newSource, err := os.Open(tt.newSourceFile)
			if err != nil {
				t.Errorf("Error opening newSourceFile %q", tt.newSourceFile)
			}

			oldDiffFile, err := os.Open(tt.oldDiffFile)
			if err != nil {
				t.Errorf("Error opening oldDiffFile %q", tt.oldDiffFile)
			}

			newDiffFile, err := os.Open(tt.newDiffFile)
			if err != nil {
				t.Errorf("Error opening newDiffFile %q", tt.newDiffFile)
			}

			correctResult, err := ioutil.ReadFile(tt.resultFile)
			if err != nil {
				t.Errorf("Error reading resultFile %q", tt.resultFile)
			}

			currentResult, err := MixedModeFile(oldSource, newSource, oldDiffFile, newDiffFile)

			if err != nil {
				t.Errorf("Mixed mode for %q: got error %v; want error nil", tt.resultFile, err)
			}
			
			want := normalizeNewlines(correctResult)
			got := normalizeNewlines([]byte(currentResult))	
			if !cmp.Equal(want, got) {
				t.Errorf("File contents mismatch for %s (-want +got):\n%s",
					 tt.resultFile, cmp.Diff(want, got))
			}
		})
	}
}

func TestMixedModePath(t *testing.T) {
	for _, tt := range mixedModePathFileTests {
		t.Run(tt.resultFile, func(t *testing.T) {
			oldDiffFile, err := os.Open(tt.oldDiffFile)
			if err != nil {
				t.Errorf("Error opening oldDiffFile %q", tt.oldDiffFile)
			}

			newDiffFile, err := os.Open(tt.newDiffFile)
			if err != nil {
				t.Errorf("Error opening newDiffFile %q", tt.newDiffFile)
			}

			correctResult, err := ioutil.ReadFile(tt.resultFile)
			if err != nil {
				t.Errorf("Error reading resultFile %q", tt.resultFile)
			}

			currentResult, err := MixedModePath(tt.oldSource, tt.newSource, oldDiffFile, newDiffFile)

			if tt.wantErr && err == nil {
				t.Errorf("MixedModePath for %q: got error nil; want error non-nil", tt.resultFile)
			} else if !tt.wantErr {
				if err != nil {
					t.Errorf("MixedModePath for %q: got error %v; want error nil", tt.resultFile, err)
				}
				
				want := normalizeNewlines(correctResult)
				got := normalizeNewlines([]byte(currentResult))	
				if !cmp.Equal(want, got) {
					t.Errorf("File contents mismatch for %s (-want +got):\n%s",
						 tt.resultFile, cmp.Diff(want, got))
				}
			}
		})
	}
}

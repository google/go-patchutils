package patchutils

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sourcegraph/go-diff/diff"
)

var interDiffFileTests = []struct {
	diffAFile  string
	diffBFile  string
	resultFile string
}{
	{
		diffAFile:  "s1_a.diff",
		diffBFile:  "s1_b.diff",
		resultFile: "s1_a_b.diff",
	},
	{
		diffAFile:  "s2_a.diff",
		diffBFile:  "s2_b.diff",
		resultFile: "s2_a_b.diff",
	},
}

var applyDiffFileTests = []struct {
	sourceFile string
	diffFile   string
	resultFile string
	wantErr    bool
}{
	{
		sourceFile: "source_1/file_1.txt",
		diffFile:   "f1_a.diff",
		resultFile: "source_1_a/file_1.txt",
		wantErr:    false,
	},
	{
		sourceFile: "source_1_a/file_1.txt",
		diffFile:   "f1_a_c.diff",
		resultFile: "source_1_c/file_1.txt",
		wantErr:    false,
	},
	{
		sourceFile: "source_1/file_1.txt",
		diffFile:   "f1_b.diff",
		resultFile: "source_1_b/file_1.txt",
		wantErr:    false,
	},
	{
		sourceFile: "source_1_b/file_1.txt",
		diffFile:   "f1_b_c.diff",
		resultFile: "source_1_c/file_1.txt",
		wantErr:    false,
	},
	{
		sourceFile: "source_1/file_2.txt",
		diffFile:   "f2_a.diff",
		resultFile: "source_1_a/file_2.txt",
		wantErr:    false,
	},
	{
		sourceFile: "source_1_a/file_2.txt",
		diffFile:   "f2_a_c.diff",
		resultFile: "source_1_c/file_2.txt",
		wantErr:    false,
	},
	{
		sourceFile: "source_1/file_2.txt",
		diffFile:   "f2_b.diff",
		resultFile: "source_1_b/file_2.txt",
		wantErr:    false,
	},
	{
		sourceFile: "source_1_b/file_2.txt",
		diffFile:   "f2_b_c.diff",
		resultFile: "source_1_c/file_2.txt",
		wantErr:    false,
	},
	// sourceFile and diffFile have different origin content.
	{
		sourceFile: "source_1/file_1.txt",
		diffFile:   "f1_a_wrong_origin.diff",
		resultFile: "source_1_a/file_1.txt",
		wantErr:    true,
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
		oldSourceFile: "source_1/file_1.txt",
		oldDiffFile:   "f1_a.diff",
		newSourceFile: "source_1_b/file_1.txt",
		newDiffFile:   "f1_b_c.diff",
		resultFile:    "f1_a_c.diff",
	},
	{
		oldSourceFile: "source_1/file_2.txt",
		oldDiffFile:   "f2_a.diff",
		newSourceFile: "source_1_b/file_2.txt",
		newDiffFile:   "f2_b_c.diff",
		resultFile:    "f2_a_c.diff",
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
		oldSource:   "source_1/file_1.txt",
		oldDiffFile: "f1_a.diff",
		newSource:   "source_1_b/file_1.txt",
		newDiffFile: "f1_b_c.diff",
		resultFile:  "f1_a_c.diff",
		wantErr:     false,
	},
	{
		oldSource:   "source_1/file_2.txt",
		oldDiffFile: "f2_a.diff",
		newSource:   "source_1_b/file_2.txt",
		newDiffFile: "f2_b_c.diff",
		resultFile:  "f2_a_c.diff",
		wantErr:     false,
	},
	// Directories
	{
		oldSource:   "source_1",
		oldDiffFile: "s1_a.diff",
		newSource:   "source_1_b",
		newDiffFile: "s1_b_c.diff",
		resultFile:  "s1_a_c.diff",
		wantErr:     false,
	},
	// Contains added and unchanged files
	// Sources have different modes (file & directory)
	{
		oldSource:   "source_1",
		oldDiffFile: "s1_a.diff",
		newSource:   "source_1_/file_1.txt",
		newDiffFile: "f1_a.diff",
		resultFile:  "s1_a_c.diff",
		wantErr:     true,
	},
	// TODO: uncomment this test case, when added/deleted in diff files won't be ignored
	//{"source_1", "s1_a.diff",
	//	"source_1_d", "s1_c_d.diff", "s1_a_d.diff"},
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
			var fileA, errA = os.Open(tt.diffAFile)
			var fileB, errB = os.Open(tt.diffBFile)

			if errA != nil {
				t.Errorf("Error in opening %s file.", tt.diffAFile)
			}

			if errB != nil {
				t.Errorf("Error in opening %s file.", tt.diffBFile)
			}

			correctResult, err := ioutil.ReadFile(tt.resultFile)

			if err != nil {
				t.Error(err)
			}

			var readerA io.Reader = fileA
			var readerB io.Reader = fileB

			currentResult, err := InterDiff(readerA, readerB)
			if err != nil {
				t.Error(err)
			}

			if !bytes.Equal(normalizeNewlines([]byte(currentResult)), normalizeNewlines(correctResult)) {
				t.Errorf("File contents mismatch for %s.\nExpected:\n%s\nGot:\n%s\n",
					tt.resultFile, correctResult, currentResult)
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

			currentResult, err := ApplyDiff(string(source), d)
			if tt.wantErr && err == nil {
				t.Errorf("Applying diff for %q: got error nil; want error non-nil", tt.resultFile)
			} else if !tt.wantErr {
				if err != nil {
					t.Errorf("Applying diff for %q: got error %v; want error nil", tt.resultFile, err)
				}

				if !bytes.Equal(normalizeNewlines([]byte(currentResult)), normalizeNewlines(correctResult)) {
					t.Errorf("File contents mismatch for %s.\nGot:\n%s\nWant:\n%s\n",
						tt.resultFile, currentResult, correctResult)
				}
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

			currentResult, err := MixedMode(oldSource, newSource, oldD, newD)

			if err != nil {
				t.Errorf("Mixed mode for %q: got error %v; want error nil", tt.resultFile, err)
			}

			if !bytes.Equal(normalizeNewlines([]byte(currentResult)), normalizeNewlines(correctResult)) {
				t.Errorf("File contents mismatch for %s.\nGot:\n%s\nWant:\n%s\n",
					tt.resultFile, currentResult, correctResult)
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

				if !bytes.Equal(normalizeNewlines([]byte(currentResult)), normalizeNewlines(correctResult)) {
					t.Errorf("File contents mismatch for %s.\nGot:\n%s\nWant:\n%s\n",
						tt.resultFile, currentResult, correctResult)
				}
			}
		})
	}
}

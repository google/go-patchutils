// Package patchutils provides tools to compute the diff between source and diff files.
package patchutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	dbd "github.com/kylelemons/godebug/diff"
	"github.com/sourcegraph/go-diff/diff"
	"golang.org/x/sync/errgroup"
)

// InterDiff computes the diff of a source file patched with oldDiff
// and the same source file patched with newDiff.
// oldDiff and newDiff should be in unified format.
func InterDiff(oldDiff, newDiff io.Reader) (string, error) {
	oldFileDiffs, err := diff.NewMultiFileDiffReader(oldDiff).ReadAllFiles()
	if err != nil {
		return "", fmt.Errorf("parsing oldDiff: %w", err)
	}
	if len(oldFileDiffs) == 0 {
		return "", fmt.Errorf("oldDiff: %w", ErrEmptyDiffFile)
	}

	newFileDiffs, err := diff.NewMultiFileDiffReader(newDiff).ReadAllFiles()
	if err != nil {
		return "", fmt.Errorf("parsing newDiff: %w", err)
	}
	if len(newFileDiffs) == 0 {
		return "", fmt.Errorf("newDiff: %w", ErrEmptyDiffFile)
	}

	resultFiles := make(map[string]string)
	sort.SliceStable(oldFileDiffs, func(i, j int) bool {
		return oldFileDiffs[i].OrigName < oldFileDiffs[j].OrigName
	})
	sort.SliceStable(newFileDiffs, func(i, j int) bool {
		return newFileDiffs[i].OrigName < newFileDiffs[j].OrigName
	})
	// Iterate over files in FileDiff arrays
	i, j := 0, 0
	eg, _ := errgroup.WithContext(context.Background())
Loop:
	for i < len(oldFileDiffs) && j < len(newFileDiffs) {
		switch {
		case oldFileDiffs[i].OrigName == newFileDiffs[j].OrigName:
			switch {
			case oldFileDiffs[i].NewName == "" && newFileDiffs[j].NewName == "":
				// In both versions file has been deleted
				i++
				j++
				continue Loop
			case oldFileDiffs[i].NewName == "":
				// File was deleted in old version
				resultFiles[newFileDiffs[j].OrigName] = fmt.Sprintf("Only in %s: %s\n", filepath.Dir(newFileDiffs[j].NewName),
					filepath.Base(newFileDiffs[j].NewName))
			case newFileDiffs[j].NewName == "":
				// File deleted in new version
				resultFiles[oldFileDiffs[i].OrigName] = fmt.Sprintf("Only in %s: %s\n", filepath.Dir(oldFileDiffs[i].NewName),
					filepath.Base(oldFileDiffs[i].NewName))
			default:
				// interdiff of two versions
				var mu sync.Mutex
				i, j := i, j
				eg.Go(func() error {
					interFileDiff, err := interFileDiff(oldFileDiffs[i], newFileDiffs[j])
					if err != nil {
						return fmt.Errorf("merging diffs for file %q: %w", oldFileDiffs[i].OrigName, err)
					}

					fileDiffContent, err := diff.PrintFileDiff(interFileDiff)
					if err != nil {
						return fmt.Errorf("printing merged diffs for file %q: %w", oldFileDiffs[i].OrigName, err)
					}

					mu.Lock()
					defer mu.Unlock()
					resultFiles[oldFileDiffs[i].OrigName] = string(fileDiffContent)
					return nil
				})
			}
			i++
			j++
		case oldFileDiffs[i].OrigName < newFileDiffs[j].OrigName:
			// current file is only mentioned in oldDiff
			// determine if file has been added or just changed in only one version
			revertHunks(oldFileDiffs[i])
			oldD, err := interPrintSingleFileDiff(oldFileDiffs[i])
			if err != nil {
				return "", fmt.Errorf("printing oldDiff: %w", err)
			}
			resultFiles[oldFileDiffs[i].OrigName] = oldD
			i++
		case oldFileDiffs[i].OrigName > newFileDiffs[j].OrigName:
			// current file is only mentioned in newDiff
			// determine if file has been added or just changed in only one version
			newD, err := interPrintSingleFileDiff(newFileDiffs[j])
			if err != nil {
				return "", fmt.Errorf("printing newDiff: %w", err)
			}
			resultFiles[newFileDiffs[j].OrigName] = newD
			j++
		}
	}

	// In case there are more oldFileDiffs, while newFileDiffs are run out
	for i < len(oldFileDiffs) {
		oldD, err := interPrintSingleFileDiff(oldFileDiffs[i])
		if err != nil {
			return "", fmt.Errorf("printing oldDiff: %w", err)
		}
		resultFiles[oldFileDiffs[i].OrigName] = oldD
		i++
	}

	// In case there are more newFileDiffs, while oldFileDiffs are run out
	for j < len(newFileDiffs) {
		newD, err := interPrintSingleFileDiff(newFileDiffs[j])
		if err != nil {
			return "", fmt.Errorf("printing newDiff: %w", err)
		}
		resultFiles[newFileDiffs[j].OrigName] = newD
		j++
	}

	if err := eg.Wait(); err != nil {
		return "", fmt.Errorf("wait all routines: %w", err)
	}

	// Add diff files to result in order
	var originalFilenames []string
	for f := range resultFiles {
		originalFilenames = append(originalFilenames, f)
	}
	sort.Strings(originalFilenames)
	var result string
	for _, k := range originalFilenames {
		result += resultFiles[k]
	}

	return result, nil
}

// mixedMode computes the diff of a oldSource file patched with oldDiff
// and the newSource file patched with newDiff.
func mixedMode(oldSource, newSource io.Reader, oldFileDiff, newFileDiff *diff.FileDiff) (*diff.FileDiff, error) {
	// Skip check if in some version the file has been added/deleted as this is already done in MixedModeFilePath,
	// before opening oldSource and newSource files
	oldSourceContent, err := readContent(oldSource)
	if err != nil {
		return nil, fmt.Errorf("reading content of OldSource: %w", err)
	}

	newSourceContent, err := readContent(newSource)
	if err != nil {
		return nil, fmt.Errorf("reading content of NewSource: %w", err)
	}

	updatedOldSource, err := applyDiff(oldSourceContent, oldFileDiff)
	if err != nil {
		return nil, fmt.Errorf("applying diff to OldSource: %w", err)
	}

	updatedNewSource, err := applyDiff(newSourceContent, newFileDiff)
	if err != nil {
		return nil, fmt.Errorf("applying diff to NewSource: %w", err)
	}

	ch := dbd.DiffChunks(strings.Split(strings.TrimSuffix(updatedOldSource, "\n"), "\n"),
		strings.Split(strings.TrimSuffix(updatedNewSource, "\n"), "\n"))

	// TODO: something with extended (extended header lines)
	resultFileDiff := &diff.FileDiff{
		Extended: []string{},
		Hunks:    []*diff.Hunk{},
	}
	if oldFileDiff != nil {
		resultFileDiff.OrigName = oldFileDiff.NewName
		resultFileDiff.OrigTime = oldFileDiff.NewTime
	}
	if newFileDiff != nil {
		resultFileDiff.NewName = newFileDiff.NewName
		resultFileDiff.NewTime = newFileDiff.NewTime
	}

	convertChunksIntoFileDiff(ch, resultFileDiff)

	return resultFileDiff, nil
}

// MixedModeFile computes the diff of an oldSource file patched with oldDiff and
// newSource file patched with newDiff.
func MixedModeFile(oldSource, newSource, oldDiff, newDiff io.Reader) (string, error) {
	oldD, err := diff.NewFileDiffReader(oldDiff).Read()
	if err != nil {
		return "", fmt.Errorf("parsing oldDiff: %w", err)
	}

	newD, err := diff.NewFileDiffReader(newDiff).Read()
	if err != nil {
		return "", fmt.Errorf("parsing newDiff: %w", err)
	}

	resultFileDiff, err := mixedMode(oldSource, newSource, oldD, newD)
	if err != nil {
		return "", fmt.Errorf("mixedMode: %w", err)
	}
	result, err := diff.PrintFileDiff(resultFileDiff)
	if err != nil {
		return "", fmt.Errorf("printing result diff for file %q: %w",
			resultFileDiff.OrigName, err)
	}

	return string(result), nil
}

// MixedModePath recursively computes the diff of an oldSource patched with oldDiff
// and the newSource patched with newDiff, recursively if OldSource and NewSource are directories.
func MixedModePath(oldSourcePath, newSourcePath string, oldDiff, newDiff io.Reader) (string, error) {
	// Get stats of sources
	oldSourceStat, err := os.Stat(oldSourcePath)
	if err != nil {
		return "", fmt.Errorf("get stat from oldSourcePath %q: %w",
			oldSourcePath, err)
	}

	newSourceStat, err := os.Stat(newSourcePath)
	if err != nil {
		return "", fmt.Errorf("get stat from newSourcePath %q: %w",
			newSourcePath, err)
	}

	// Check mode of sources
	switch {
	case !oldSourceStat.IsDir() && !newSourceStat.IsDir():
		// Both sources are files
		oldD, err := diff.NewFileDiffReader(oldDiff).Read()
		if err != nil {
			return "", fmt.Errorf("parsing oldDiff for %q: %w",
				oldSourcePath, err)
		}

		if oldSourcePath != oldD.OrigName {
			return "", fmt.Errorf("filenames mismatch for oldSourcePath: %q and oldDiff: %q",
				oldSourcePath, oldD.OrigName)
		}

		newD, err := diff.NewFileDiffReader(newDiff).Read()
		if err != nil {
			return "", fmt.Errorf("parsing newDiff for %q: %w",
				newSourcePath, err)
		}

		if newSourcePath != newD.OrigName {
			return "", fmt.Errorf("filenames mismatch for newSourcePath: %q and newDiff: %q",
				newSourcePath, newD.OrigName)
		}

		resultString, err := mixedModeFilePath(oldSourcePath, newSourcePath, oldD, newD)
		return resultString, err

	case oldSourceStat.IsDir() && newSourceStat.IsDir():
		// Both paths are directories
		resultString, err := mixedModeDirPath(oldSourcePath, newSourcePath, oldDiff, newDiff)
		if err != nil {
			return "", fmt.Errorf("compute diff for %q and %q: %w",
				oldSourcePath, newSourcePath, err)
		}

		return resultString, nil
	}

	return "", errors.New("sources should be both dirs or files")
}

// readContent returns content of source as string
func readContent(source io.Reader) (string, error) {
	buf := new(strings.Builder)
	_, err := io.Copy(buf, source)
	if err != nil {
		return "", fmt.Errorf("copying source: %w", err)
	}
	return buf.String(), nil
}

// applyDiff returns applied changes from diffFile to source
func applyDiff(source string, diffFile *diff.FileDiff) (string, error) {
	if diffFile == nil {
		return source, nil
	}
	sourceBody := strings.Split(source, "\n")

	// currentOrgSourceI = 1 -- In diff lines started counting from 1
	var currentOrgSourceI int32 = 1
	var newBody []string

	for _, hunk := range diffFile.Hunks {
		// Add untouched part of source
		newBody = append(newBody, sourceBody[currentOrgSourceI-1:hunk.OrigStartLine-1]...)
		currentOrgSourceI = hunk.OrigStartLine

		hunkBody := strings.Split(strings.TrimSuffix(string(hunk.Body), "\n"), "\n")

		for _, line := range hunkBody {
			if strings.HasPrefix(line, "+") {
				newBody = append(newBody, line[1:])
			} else {
				if line[1:] != sourceBody[currentOrgSourceI-1] {
					return "", fmt.Errorf(
						"line %d in source (%q) and diff (%q): %w",
						currentOrgSourceI, sourceBody[currentOrgSourceI-1], line[1:], ErrContentMismatch)
				}

				if strings.HasPrefix(line, " ") {
					newBody = append(newBody, sourceBody[currentOrgSourceI-1])
				}

				currentOrgSourceI++
			}
		}
	}

	newBody = append(newBody, sourceBody[currentOrgSourceI-1:]...)

	return strings.Join(newBody, "\n"), nil
}

// mixedModeFilePath computes the diff of a oldSourcePath file patched with oldFileDiff
// and the newSourcePath file patched with newFileDiff.
func mixedModeFilePath(oldSourcePath, newSourcePath string, oldFileDiff, newFileDiff *diff.FileDiff) (string, error) {
	if oldFileDiff != nil && oldFileDiff.NewName == "" && newFileDiff != nil && newFileDiff.NewName == "" {
		// In both updated version file has been deleted
		return "", nil
	}

	if oldFileDiff != nil && oldFileDiff.OrigName != "" && oldFileDiff.NewName == "" {
		// File has been deleted in updated old version
		if newFileDiff == nil {
			return fmt.Sprintf("Only in %s: %s\n", newSourcePath,
				filepath.Base(oldFileDiff.NewName)), nil
		}
		return fmt.Sprintf("Only in %s: %s\n", filepath.Dir(newFileDiff.NewName),
			filepath.Base(newFileDiff.NewName)), nil
	}

	if newFileDiff != nil && newFileDiff.OrigName != "" && newFileDiff.NewName == "" {
		// File has been deleted in updated new version
		if oldFileDiff == nil {
			return fmt.Sprintf("Only in %s: %s\n", oldSourcePath,
				filepath.Base(newFileDiff.NewName)), nil
		}
		return fmt.Sprintf("Only in %s: %s\n", filepath.Dir(oldFileDiff.NewName),
			filepath.Base(oldFileDiff.NewName)), nil
	}

	oldSourceFile, err := os.Open(oldSourcePath)
	if err != nil {
		return "", fmt.Errorf("opening oldSource file %q: %w",
			oldSourcePath, err)
	}

	newSourceFile, err := os.Open(newSourcePath)
	if err != nil {
		return "", fmt.Errorf("opening newSource file %q: %w",
			newSourcePath, err)
	}

	resultFileDiff, err := mixedMode(oldSourceFile, newSourceFile, oldFileDiff, newFileDiff)
	if err != nil {
		return "", fmt.Errorf("compute diff for %q and %q: %w",
			oldSourcePath, newSourcePath, err)
	}
	if oldFileDiff == nil {
		resultFileDiff.OrigName = oldSourcePath
	}
	if newFileDiff == nil {
		resultFileDiff.NewName = newSourcePath
	}
	result, err := diff.PrintFileDiff(resultFileDiff)
	if err != nil {
		return "", fmt.Errorf("printing result diff for file %q: %w",
			resultFileDiff.OrigName, err)
	}

	return string(result), nil
}

// mixedModeDirPath computes the diff of a oldSourcePath directory patched with oldDiff
// and the newSourcePath directory patched with newDiff.
func mixedModeDirPath(oldSourcePath, newSourcePath string, oldDiff, newDiff io.Reader) (string, error) {
	oldFileNames, err := getAllFileNamesInDir(oldSourcePath)
	if err != nil {
		return "", fmt.Errorf("get all filenames for oldSource: %w", err)
	}

	newFileNames, err := getAllFileNamesInDir(newSourcePath)
	if err != nil {
		return "", fmt.Errorf("get all filenames for newSourcePath: %w", err)
	}

	oldFileDiffReader := diff.NewMultiFileDiffReader(oldDiff)
	newFileDiffReader := diff.NewMultiFileDiffReader(newDiff)

	oldFileDiffArr, err := oldFileDiffReader.ReadAllFiles()
	if err != nil {
		return "", fmt.Errorf("parsing next FileDiff in oldDiff: %w", err)
	}
	oldFileDiffs := make(map[string]diff.FileDiff)
	for _, d := range oldFileDiffArr {
		oldFileDiffs[d.OrigName] = *d
	}

	newFileDiffArr, err := newFileDiffReader.ReadAllFiles()
	if err != nil {
		return "", fmt.Errorf("parsing next FileDiff in newDiff: %w", err)
	}
	newFileDiffs := make(map[string]diff.FileDiff)
	for _, d := range newFileDiffArr {
		newFileDiffs[d.OrigName] = *d
	}

	result := ""

	// Iterate over files in FileDiff arrays
	i, j := 0, 0
	var currentOldDiff, currentNewDiff *diff.FileDiff
	for i < len(oldFileNames) && j < len(newFileNames) {
		// Check if there is correspondent oldFileDiff to current olfFile
		if d, ok := oldFileDiffs[oldFileNames[i]]; ok && currentOldDiff == nil {
			currentOldDiff = &d
			delete(oldFileDiffs, oldFileNames[i])
		}
		// Check if there is correspondent newFileDiff to current newFile
		if d, ok := newFileDiffs[newFileNames[j]]; ok && currentNewDiff == nil {
			currentNewDiff = &d
			delete(newFileDiffs, newFileNames[j])
		}
		switch {
		// Comparing parts after oldSourcePath and newSourcePath
		case strings.TrimPrefix(oldFileNames[i], oldSourcePath) == strings.TrimPrefix(newFileNames[j], newSourcePath):
			currentResult, err := mixedModeFilePath(oldFileNames[i], newFileNames[j], currentOldDiff, currentNewDiff)
			if err != nil {
				return "", fmt.Errorf("mixedModeFilePath for oldFile: %q and newFile: %q: %w",
					oldFileNames[i], newFileNames[j], err)
			}
			result += currentResult
			i++
			j++
			currentOldDiff = nil
			currentNewDiff = nil
		case strings.TrimPrefix(oldFileNames[i], oldSourcePath) < strings.TrimPrefix(newFileNames[j], newSourcePath):
			if currentOldDiff != nil {
				if currentOldDiff.NewName != "" {
					// File wasn't deleted in oldDiff
					result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(currentOldDiff.NewName),
						filepath.Base(currentOldDiff.NewName))
				}
			} else {
				result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(oldFileNames[i]),
					filepath.Base(oldFileNames[i]))
			}
			i++
			currentOldDiff = nil
		default:
			if currentNewDiff != nil {
				if currentNewDiff.NewName != "" {
					// File wasn't deleted in newDiff
					result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(currentNewDiff.NewName),
						filepath.Base(currentNewDiff.NewName))
				}
			} else {
				result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(newFileNames[j]),
					filepath.Base(newFileNames[j]))
			}
			j++
			currentNewDiff = nil
		}
	}

	// Iterate over oldFiles, while we run out of newFiles
	for i < len(oldFileNames) {
		// Check if there is correspondent oldFileDiff to current olfFile
		if d, ok := oldFileDiffs[oldFileNames[i]]; ok {
			delete(oldFileDiffs, oldFileNames[i])
			if d.NewName != "" {
				// File hasn't been deleted
				result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(oldFileNames[i]),
					filepath.Base(oldFileNames[i]))
			}
		} else {
			result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(oldFileNames[i]),
				filepath.Base(oldFileNames[i]))
		}
		i++
	}

	// Iterate over newFiles, while we run out of oldFiles
	for j < len(newFileNames) {
		// Check if there is correspondent oldFileDiff to current olfFile
		if d, ok := newFileDiffs[newFileNames[j]]; ok {
			delete(newFileDiffs, newFileNames[j])
			if d.NewName != "" {
				// File hasn't been deleted
				result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(newFileNames[j]),
					filepath.Base(newFileNames[j]))
			}
		} else {
			result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(newFileNames[j]),
				filepath.Base(newFileNames[j]))
		}
		j++
	}

	// New files have been added to the old version
	for filename, _ := range oldFileDiffs {
		result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(filename),
			filepath.Base(filename))
	}

	// New files have been added to the new version
	for filename, _ := range oldFileDiffs {
		result += fmt.Sprintf("Only in %s: %s\n", filepath.Dir(filename),
			filepath.Base(filename))
	}

	return result, nil
}

// getAllFileNamesInDir returns array of paths to files in root recursively.
func getAllFileNamesInDir(root string) ([]string, error) {
	var allFiles []string
	err := filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("walk into %q: %w",
					path, err)
			}
			if !info.IsDir() {
				allFiles = append(allFiles, path)
			}
			return nil
		})

	return allFiles, err
}

const contextLines = 2

// convertChunksIntoFileDiff adds the given chunks to the fileDiff struct.
func convertChunksIntoFileDiff(chunks []dbd.Chunk, fileDiff *diff.FileDiff) {
	var currentOldI, currentNewI int32 = 1, 1
	currentHunk := &diff.Hunk{
		OrigStartLine: currentOldI,
		NewStartLine:  currentNewI,
	}
	// Delete empty chunks in the beginning
	for len(chunks) > 0 && len(chunks[0].Added) == 0 && len(chunks[0].Deleted) == 0 && len(chunks[0].Equal) == 0 {
		chunks = chunks[1:]
	}
	// Delete empty chunks in the end
	last := len(chunks) - 1
	for len(chunks) > 0 && len(chunks[last].Added) == 0 && len(chunks[last].Deleted) == 0 && len(chunks[last].Equal) == 0 {
		chunks = chunks[:last]
		last--
	}

	// If chunks contains only one element with only unchanged lines
	if len(chunks) == 1 && len(chunks[0].Added) == 0 && len(chunks[0].Deleted) == 0 {
		return
	}

	var currentHunkBody []string

	// If array of chunks is already empty
	if len(chunks) == 0 {
		return
	}

	// If first chunk contains only equal lines, we are adding last contextLines to currentHunk
	if len(chunks[0].Added) == 0 && len(chunks[0].Deleted) == 0 {
		currentOldI += int32(len(chunks[0].Equal))
		currentNewI += int32(len(chunks[0].Equal))
		if len(chunks[0].Equal) > contextLines {
			for _, line := range chunks[0].Equal[len(chunks[0].Equal)-contextLines:] {
				currentHunkBody = append(currentHunkBody, " "+line)
				currentHunk.OrigStartLine = currentOldI - contextLines
				currentHunk.NewStartLine = currentNewI - contextLines
			}
		} else {
			for _, line := range chunks[0].Equal {
				currentHunkBody = append(currentHunkBody, " "+line)
			}
		}
		// Removing processed first hunk
		chunks = chunks[1:]
	}

	var lastLines []string
	last = len(chunks) - 1
	// If last chunk contains equal lines, save first contextLines of equal lines for further processing
	if len(chunks[last].Equal) > 0 {
		if len(chunks[last].Equal) > contextLines {
			for _, line := range chunks[last].Equal[:contextLines] {
				lastLines = append(lastLines, " "+line)
			}
		} else {
			for _, line := range chunks[last].Equal {
				lastLines = append(lastLines, " "+line)
			}
		}
		// Removing processed equal lines from last chunk
		chunks[last].Equal = []string{}
	}

	for _, c := range chunks {
		// A chunk will not have both added and deleted lines.
		for _, line := range c.Added {
			currentHunkBody = append(currentHunkBody, "+"+line)
			currentNewI++
		}
		for _, line := range c.Deleted {
			currentHunkBody = append(currentHunkBody, "-"+line)
			currentOldI++
		}

		// Next piece of content contains too many unchanged lines.
		// Current hunk will be 'closed' and started new one.
		if len(c.Equal) > 2*contextLines+1 {
			if len(currentHunkBody) > 0 {
				for _, line := range c.Equal[:contextLines] {
					currentHunkBody = append(currentHunkBody, " "+line)
				}
				currentHunk.OrigLines = currentOldI + contextLines + 1 - currentHunk.OrigStartLine
				currentHunk.NewLines = currentNewI + contextLines + 1 - currentHunk.NewStartLine
				currentHunk.Body = []byte(strings.Join(currentHunkBody, "\n") + "\n")
				fileDiff.Hunks = append(fileDiff.Hunks, currentHunk)
			}

			currentOldI += int32(len(c.Equal))
			currentNewI += int32(len(c.Equal))

			currentHunk = &diff.Hunk{
				OrigStartLine: currentOldI - contextLines,
				NewStartLine:  currentNewI - contextLines,
			}

			// Clean currentHunkBody
			currentHunkBody = []string{}
			for _, line := range c.Equal[len(c.Equal)-contextLines-1:] {
				currentHunkBody = append(currentHunkBody, " "+line)
			}

		} else {
			for _, line := range c.Equal {
				currentHunkBody = append(currentHunkBody, " "+line)
				currentOldI++
				currentNewI++
			}
		}
	}

	// Add lastLines (equal) to last hunk
	for _, line := range lastLines {
		currentHunkBody = append(currentHunkBody, line)
		currentOldI++
		currentNewI++
	}

	// currentHunkBody contains some lines. It need to be 'closed' and added to fileDiff.Hunks
	currentHunk.OrigLines = currentOldI - currentHunk.OrigStartLine
	currentHunk.NewLines = currentNewI - currentHunk.NewStartLine
	currentHunk.Body = []byte(strings.Join(currentHunkBody, "\n") + "\n")
	fileDiff.Hunks = append(fileDiff.Hunks, currentHunk)
}

// interPrintSingleFileDiff returns printed version of diffFile, which was found only in one out of two versions.
func interPrintSingleFileDiff(diffFile *diff.FileDiff) (string, error) {
	if diffFile.NewName == "" {
		// File has been added in current version
		return fmt.Sprintf("Only in %s: %s\n", filepath.Dir(diffFile.OrigName),
			filepath.Base(diffFile.OrigName)), nil
	}

	// File has been changed in current version and left unchanged in other version
	oldD, err := diff.PrintFileDiff(diffFile)
	if err != nil {
		return "", fmt.Errorf("printing diff for file %q: %w",
			diffFile.NewName, err)
	}
	return string(oldD), nil
}

// interFileDiff returns a new diff.FileDiff that is a diff of a source file patched with oldFileDiff
// and the same source file patched with newFileDiff.
func interFileDiff(oldFileDiff, newFileDiff *diff.FileDiff) (*diff.FileDiff, error) {

	// Configuration of result FileDiff
	// TODO: something with extended (extended header lines)

	resultFileDiff := &diff.FileDiff{
		OrigName: oldFileDiff.NewName,
		OrigTime: oldFileDiff.NewTime,
		NewName:  newFileDiff.NewName,
		NewTime:  newFileDiff.NewTime,
		Extended: []string{},
		Hunks:    []*diff.Hunk{}}

	// Iterating over hunks in order they start in origin
	i, j := 0, 0
	for i < len(oldFileDiff.Hunks) && j < len(newFileDiff.Hunks) {
		switch {
		case oldFileDiff.Hunks[i].OrigStartLine+oldFileDiff.Hunks[i].OrigLines < newFileDiff.Hunks[j].OrigStartLine:
			// Whole oldHunk is before starting of newHunk
			resultFileDiff.Hunks = append(resultFileDiff.Hunks,
				revertedHunk(oldFileDiff.Hunks[i]))
			i++
		case newFileDiff.Hunks[j].OrigStartLine+newFileDiff.Hunks[j].OrigLines < oldFileDiff.Hunks[i].OrigStartLine:
			// Whole newHunk is before starting of oldHunk
			resultFileDiff.Hunks = append(resultFileDiff.Hunks, newFileDiff.Hunks[j])
			j++
		default:
			// oldHunk and newHunk are overlapping somehow
			// Collecting a whole set of overlapping hunks to produce one continuous hunk
			oldHunks, newHunks := findOverlappingHunkSet(oldFileDiff, newFileDiff, &i, &j)
			mergedOverlappingHunk, err := mergeOverlappingHunks(oldHunks, newHunks)

			if err != nil {
				return nil, fmt.Errorf("merging overlapping hunks: %w", err)
			}

			// In case opposite hunks aren't doing same changes.
			if mergedOverlappingHunk != nil {
				resultFileDiff.Hunks = append(resultFileDiff.Hunks, mergedOverlappingHunk)
			}
		}
	}

	// In case there are more hunks in oldFileDiff, while hunks of newFileDiff are run out
	for i < len(oldFileDiff.Hunks) {
		resultFileDiff.Hunks = append(resultFileDiff.Hunks,
			revertedHunk(oldFileDiff.Hunks[i]))
		i++
	}

	// In case there are more hunks in newFileDiff, while hunks of oldFileDiff are run out
	for j < len(newFileDiff.Hunks) {
		resultFileDiff.Hunks = append(resultFileDiff.Hunks, newFileDiff.Hunks[j])
		j++
	}

	return resultFileDiff, nil
}

// findOverlappingHunkSet finds next set (two arrays: oldHunks and newHunks) of
// overlapping hunks in oldFileDiff and newFileDiff, starting from position i, j relatively.
func findOverlappingHunkSet(oldFileDiff, newFileDiff *diff.FileDiff, i, j *int) (oldHunks, newHunks []*diff.Hunk) {
	// Collecting overlapped hunks into two arrays

	oldHunks = append(oldHunks, oldFileDiff.Hunks[*i])
	newHunks = append(newHunks, newFileDiff.Hunks[*j])
	*i++
	*j++

Loop:
	for {
		switch {
		// Starting line of oldHunk is in previous newHunk body (between start and last lines)
		case *i < len(oldFileDiff.Hunks) && oldFileDiff.Hunks[*i].OrigStartLine >= newFileDiff.Hunks[*j-1].OrigStartLine &&
			oldFileDiff.Hunks[*i].OrigStartLine < newFileDiff.Hunks[*j-1].OrigStartLine+newFileDiff.Hunks[*j-1].OrigLines:
			oldHunks = append(oldHunks, oldFileDiff.Hunks[*i])
			*i++
		// Starting line of newHunk is in previous oldHunk body (between start and last lines)
		case *j < len(newFileDiff.Hunks) && newFileDiff.Hunks[*j].OrigStartLine >= oldFileDiff.Hunks[*i-1].OrigStartLine &&
			newFileDiff.Hunks[*j].OrigStartLine < oldFileDiff.Hunks[*i-1].OrigStartLine+oldFileDiff.Hunks[*i-1].OrigLines:
			newHunks = append(newHunks, newFileDiff.Hunks[*j])
			*j++
		default:
			// No overlapping hunks left
			break Loop
		}
	}

	return oldHunks, newHunks
}

// mergeOverlappingHunks returns a new diff.Hunk that is a diff hunk between overlapping oldHunks and newHunks,
// related to the same source file.
func mergeOverlappingHunks(oldHunks, newHunks []*diff.Hunk) (*diff.Hunk, error) {
	resultHunk, currentOrgI, err := configureResultHunk(oldHunks, newHunks)

	if err != nil {
		return nil, fmt.Errorf("configuring result hunk: %w", err)
	}

	// Indexes of hunks
	currentOldHunkI, currentNewHunkJ := 0, 0
	// Indexes of lines in body hunks
	// if indexes == -1 -- we don't have relevant hunk, which contains changes nearby currentOrgI
	i, j := -1, -1

	// Body of hunks
	var newBody []string
	var oldHunkBody, newHunkBody []string

	// Iterating through the hunks in the order they're appearing in origin file.
	// Using number of line in origin (currentOrgI) as an anchor to process line by line.
	// By using currentOrgI as anchor it is easier to see how changes have been applied step by step.

	// Merge, while there are hunks to process
	for currentOldHunkI < len(oldHunks) || currentNewHunkJ < len(newHunks) {

		// Entering next hunk in oldHunks
		if currentOldHunkI < len(oldHunks) && i == -1 && currentOrgI == oldHunks[currentOldHunkI].OrigStartLine {
			i = 0
			oldHunkBody = strings.Split(strings.TrimSuffix(string(oldHunks[currentOldHunkI].Body), "\n"), "\n")
		}

		// Entering next hunk in newHunks
		if currentNewHunkJ < len(newHunks) && j == -1 && currentOrgI == newHunks[currentNewHunkJ].OrigStartLine {
			j = 0
			newHunkBody = strings.Split(strings.TrimSuffix(string(newHunks[currentNewHunkJ].Body), "\n"), "\n")
		}

		switch {
		case i == -1 && j == -1:
		case i >= 0 && j == -1:
			// Changes are only in oldHunk
			newBody = append(newBody, revertedLine(oldHunkBody[i]))
			// In case current line haven't been added, we have processed anchor line.
			if !strings.HasPrefix(oldHunkBody[i], "+") {
				// Updating index of anchor line.
				currentOrgI++
			}
			i++

		case i == -1 && j >= 0:
			// Changes are only in newHunk
			newBody = append(newBody, newHunkBody[j])
			// In case current line haven't been added, we have processed anchor line.
			if !strings.HasPrefix(newHunkBody[j], "+") {
				// Updating index of anchor line.
				currentOrgI++
			}
			j++

		default:
			// Changes are in old and new hunks.
			switch {
			// Firstly proceeding added lines,
			// because added lines are between previous currentOrgI and currentOrgI.
			case strings.HasPrefix(oldHunkBody[i], "+") || strings.HasPrefix(newHunkBody[j], "+"):
				newBody = append(newBody, interAddedLines(&i, &j, &oldHunkBody, &newHunkBody)...)
			default:
				// Checking if original content is the same
				if oldHunkBody[i][1:] != newHunkBody[j][1:] {
					return nil, fmt.Errorf(
						"line in original %d in oldDiff (%q) and newDiff (%q): %w",
						currentOrgI, oldHunkBody[i][1:], newHunkBody[j][1:], ErrContentMismatch)
				}
				switch {
				case strings.HasPrefix(oldHunkBody[i], " ") && strings.HasPrefix(newHunkBody[j], " "):
					newBody = append(newBody, oldHunkBody[i])
				case strings.HasPrefix(oldHunkBody[i], "-") && strings.HasPrefix(newHunkBody[j], " "):
					newBody = append(newBody, revertedLine(oldHunkBody[i]))
				case strings.HasPrefix(oldHunkBody[i], " ") && strings.HasPrefix(newHunkBody[j], "-"):
					newBody = append(newBody, newHunkBody[j])
					// If both have deleted same line, no need to append it to newBody
				}

				// Updating currentOrgI since we have processed anchor line.
				currentOrgI++
				i++
				j++
			}
		}

		if i >= len(oldHunkBody) {
			// Proceed whole oldHunkBody
			i = -1
			currentOldHunkI++
		}

		if j >= len(newHunkBody) {
			// Proceed whole newHunkBody
			j = -1
			currentNewHunkJ++
		}
	}

	resultHunk.Body = []byte(strings.Join(newBody, "\n") + "\n")

	for _, line := range newBody {
		if !strings.HasPrefix(line, " ") {
			// resultHunkBody contains some changes
			return resultHunk, nil
		}
	}

	return nil, nil
}

// interAddedLines finds interdiff between added lines in oldHunkBody (after i) and newHunkBody (after j)
func interAddedLines(i, j *int, oldHunkBody, newHunkBody *[]string) []string {
	var result, oldAddedLines, newAddedLines []string
	// Collect added lines in oldHunkBody
	for (*i < len(*oldHunkBody)) && (strings.HasPrefix((*oldHunkBody)[*i], "+")) {
		oldAddedLines = append(oldAddedLines, (*oldHunkBody)[*i][1:])
		*i++
	}
	// Collect added lines in newHunkBody
	for (*j < len(*newHunkBody)) && (strings.HasPrefix((*newHunkBody)[*j], "+")) {
		newAddedLines = append(newAddedLines, (*newHunkBody)[*j][1:])
		*j++
	}

	// Difference between collected added lines
	chunks := dbd.DiffChunks(oldAddedLines, newAddedLines)
	for _, c := range chunks {
		// A chunk will not have both added and deleted lines.
		for _, line := range c.Added {
			result = append(result, "+"+line)
		}
		for _, line := range c.Deleted {
			result = append(result, "-"+line)
		}
		for _, line := range c.Equal {
			result = append(result, " "+line)
		}
	}

	return result
}

// configureResultHunk returns a new diff.Hunk (with configured StartLines and NumberLines)
// and currentOrgI (number of anchor line) based on oldHunks and newHunks, for their further merge.
func configureResultHunk(oldHunks, newHunks []*diff.Hunk) (*diff.Hunk, int32, error) {
	if len(oldHunks) == 0 || len(newHunks) == 0 {
		return nil, 0, errors.New("one of the hunks array is empty")
	}

	var currentOrgI int32
	resultHunk := &diff.Hunk{
		// TODO: Concatenate sections
		Section: "",
		Body:    []byte{0},
	}

	firstOldHunk, firstNewHunk := oldHunks[0], newHunks[0]
	lastOldHunk, lastNewHunk := oldHunks[len(oldHunks)-1], newHunks[len(newHunks)-1]

	// Calculate StartLine for origin and new in result
	if firstOldHunk.OrigStartLine < firstNewHunk.OrigStartLine {
		// Started with old hunk
		currentOrgI = firstOldHunk.OrigStartLine
		// As we started with this old hunk, OrigStartLine will be same as start line of hunk in old source
		resultHunk.OrigStartLine = firstOldHunk.NewStartLine
		// StartLine in firstNewHunk - number of origin lines between start of firstNewHunk and start of resultHunk
		resultHunk.NewStartLine = currentOrgI +
			firstNewHunk.NewStartLine - firstNewHunk.OrigStartLine
	} else {
		// Started with new hunk
		currentOrgI = firstNewHunk.OrigStartLine
		// StartLine in firstOldHunk - number of origin lines between start of firstOldHunk and start of resultHunk
		resultHunk.OrigStartLine = currentOrgI +
			firstOldHunk.NewStartLine - firstOldHunk.OrigStartLine
		// As we started with this new hunk, NewStartLine will be same as start line of hunk in new source
		resultHunk.NewStartLine = firstNewHunk.NewStartLine
	}

	// Calculate NumberLines for origin and new in result
	if lastOldHunk.OrigStartLine+lastOldHunk.OrigLines >
		lastNewHunk.OrigStartLine+lastNewHunk.OrigLines {
		// Finished with old hunk
		// Last line of lastOldHunk - first line of origin in resultHunk
		resultHunk.OrigLines = lastOldHunk.NewStartLine + lastOldHunk.NewLines - resultHunk.OrigStartLine
		// Last line of new in resultHunk - first line of new in resultHunk
		// lastNewHunk.NewStartLine + lastNewHunk.NewLines = last line of lastNewHunk
		resultHunk.NewLines = lastNewHunk.NewStartLine + lastNewHunk.NewLines +
			// + number of origin lines between last line of lastNewHunk and lastOldHunk
			lastOldHunk.OrigStartLine + lastOldHunk.OrigLines -
			lastNewHunk.OrigStartLine - lastNewHunk.OrigLines -
			// - first line of new in resultHunk
			resultHunk.NewStartLine
	} else {
		// Finished with new hunk
		// Last line of old in resultHunk - first line of old in resultHunk
		// lastOldHunk.NewStartLine + lastOldHunk.NewLines = last line of lastOldHunk
		resultHunk.OrigLines = lastOldHunk.NewStartLine + lastOldHunk.NewLines +
			// + number of origin lines between last line of lastOldHunk and lastNewHunk
			lastNewHunk.OrigStartLine + lastNewHunk.OrigLines -
			lastOldHunk.OrigStartLine - lastOldHunk.OrigLines -
			// - first line of old in resultHunk
			resultHunk.OrigStartLine
		// Last line of lastNewHunk - first line of new in resultHunk
		resultHunk.NewLines = lastNewHunk.NewStartLine + lastNewHunk.NewLines - resultHunk.NewStartLine
	}

	resultHunk.OrigNoNewlineAt = 0
	resultHunk.StartPosition = firstOldHunk.StartPosition

	return resultHunk, currentOrgI, nil
}

// revertHunks reverts each hunk body and swaps values of start and number of lines in hunks in diffFile
func revertHunks(diffFile *diff.FileDiff) {
	for i, h := range diffFile.Hunks {
		diffFile.Hunks[i] = revertedHunk(h)
	}
}

// revertedHunk returns a copy of hunk with reverted lines of Body.
func revertedHunk(hunk *diff.Hunk) *diff.Hunk {
	var newBody []string

	lines := strings.Split(string(hunk.Body), "\n")

	for _, line := range lines {
		newBody = append(newBody, revertedLine(line))
	}

	revertedHunk := &diff.Hunk{
		OrigStartLine:   hunk.NewStartLine,
		OrigLines:       hunk.NewLines,
		OrigNoNewlineAt: hunk.OrigNoNewlineAt,
		NewStartLine:    hunk.OrigStartLine,
		NewLines:        hunk.OrigLines,
		Section:         hunk.Section,
		StartPosition:   hunk.StartPosition,
		Body:            []byte(strings.Join(newBody, "\n") + "\n"),
	}

	return revertedHunk
}

// revertedLine returns a reverted line.
// `+` added lines are marked as `-` deleted and vise versa.
// ` ` unchanged lines are left as unchanged.
func revertedLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+"):
		return "-" + line[1:]
	case strings.HasPrefix(line, "-"):
		return "+" + line[1:]
	default:
		return line
	}
}

// ErrContentMismatch indicates that compared content is not same.
var ErrContentMismatch = errors.New("content mismatch")

// ErrEmptyDiffFile indicates that provided file doesn't contain any information about changes.
var ErrEmptyDiffFile = errors.New("empty diff file")

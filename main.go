/*
# MTGA Binary Patch Utility Documentation

-	Overview
  - This utility implements a binary file differencing and patching system. It creates, writes, and applies patches between two binary files using a custom patch format identified by the "MTGADIFF" magic number. The system ensures data integrity through SHA-256 checksums and supports files of different sizes.

- File Format Specification

  - Header Structure
    Contains:

  - Magic Identifier: "MTGADIFF" (8 bytes)

  - Version: 2 bytes

  - Major Version: 0x01

  - Minor Version: 0x00

  - Original File Information:

  - Length: uint32 (4 bytes, big-endian)

  - SHA-256 Checksum: 32 bytes

  - Patched File Information:

  - Length: uint32 (4 bytes, big-endian)

  - SHA-256 Checksum: 32 bytes

  - Patch Items Count: uint32 (4 bytes, big-endian)

  - Patch Item Structure
    Each patch item contains:

  - Offset: uint32 (4 bytes, big-endian)

  - Content Length: uint32 (4 bytes, big-endian)

  - Content: variable-length byte array

The utility includes comprehensive error checking for:
  - File format validation
  - Version compatibility
  - File length mismatches
  - Checksum verification
  - I/O operations
  - Buffer operations

- Usage Examples

Creating a Patch:

	original, err := os.ReadFile("original.dll")
	modified, err := os.ReadFile("modified.dll")
	patch, err := generatePatch(original, modified)
	patchFile, err := os.Create("patch.mtgadiff")
	err = writePatchFile(patch, patchFile)

Applying a Patch:

	patchFile, err := os.Open("patch.mtgadiff")
	readPatch, err := readPatchFile(patchFile)
	result, err := applyPatch(original, readPatch)

- Progress Reporting
The utility includes progress reporting during:
  - Patch generation
  - Patch writing
  - Patch reading
  - Patch application

Progress is displayed using console output with current/total item counts.
*/
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"mtgapatcher/helper"
	"os"
	"time"

	"github.com/Make-Tarkov-Great-Again/flog/v4/flog"
)

const (
	IDENTIFIER    = "MTGADIFF"
	VERSION_MAJOR = 0x01
	VERSION_MINOR = 0x00
)

type PatchItem struct {
	Offset  uint32 // Position in the file where the patch should be applied | uint32 (4 bytes, big-endian)
	Content []byte // The actual patch data | variable-length byte array
}

type PatchFile struct {
	OriginalLength   uint32      // Length of the original file | uint32 (4 bytes, big-endian)
	OriginalChecksum [32]byte    // SHA-256 hash of original file
	PatchedLength    uint32      // Length of the resulting patched file
	PatchedChecksum  [32]byte    // SHA-256 hash of patched file
	PatchItems       []PatchItem // List of patches to apply
}

/*
Generates a patch by comparing two binary files byte by byte.
Key features:

 1. Validates input files are not empty
 2. Returns early if files are identical
 3. Handles files of different sizes
 4. Creates patches for different sections
 5. Includes additional data if modified file is longer
*/
func generatePatch(original, modified []byte) (*PatchFile, error) {
	if len(original) == 0 || len(modified) == 0 {
		return nil, errors.New("empty input files")
	}

	patch := &PatchFile{
		OriginalLength:   uint32(len(original)),
		OriginalChecksum: sha256.Sum256(original),
		PatchedLength:    uint32(len(modified)),
		PatchedChecksum:  sha256.Sum256(modified),
		PatchItems:       []PatchItem{},
	}

	// If files are identical, return early
	if patch.OriginalLength == patch.PatchedLength && bytes.Equal(patch.OriginalChecksum[:], patch.PatchedChecksum[:]) {
		return patch, nil
	}

	minLength := helper.MinInt(len(original), len(modified)) //int(math.Min(float64(len(original)), float64(len(modified))))
	var currentData []byte
	diffOffsetStart := 0

	// Compare byte by byte up to the minimum length
	for i := 0; i < minLength; i++ {
		//fmt.Printf("\rOn Generating patch file: %d/%d (\"Max\" is estimation)", i, minLength)

		if original[i] != modified[i] {
			if len(currentData) == 0 {
				diffOffsetStart = i
			}
			currentData = append(currentData, modified[i])
		} else {
			if len(currentData) > 0 {
				patch.PatchItems = append(patch.PatchItems, PatchItem{
					Offset:  uint32(diffOffsetStart),
					Content: currentData,
				})
				currentData = nil
			}
		}
	}

	// Add any remaining diff data
	if len(currentData) > 0 {
		patch.PatchItems = append(patch.PatchItems, PatchItem{
			Offset:  uint32(diffOffsetStart),
			Content: currentData,
		})
	}

	// Handle case where patched file is longer
	if len(modified) > len(original) {
		extraData := make([]byte, len(modified)-len(original))
		copy(extraData, modified[len(original):])
		patch.PatchItems = append(patch.PatchItems, PatchItem{
			Offset:  uint32(len(original)),
			Content: extraData,
		})
	}

	return patch, nil
}

/*
Writes a patch to a file in the specified format.
Writing sequence:

 1. Magic identifier
 2. Version information
 3. Original file metadata
 4. Patched file metadata
 5. Number of patch items
 6. Individual patch items
*/
func writePatchFile(patch *PatchFile, writer io.Writer) error {
	// Write magic identifier
	if _, err := writer.Write([]byte(IDENTIFIER)); err != nil {
		return err
	}

	// Write version
	if _, err := writer.Write([]byte{VERSION_MAJOR, VERSION_MINOR}); err != nil {
		return err
	}

	// Write original file info
	if err := binary.Write(writer, binary.BigEndian, patch.OriginalLength); err != nil {
		return err
	}
	if _, err := writer.Write(patch.OriginalChecksum[:]); err != nil {
		return err
	}

	// Write patched file info
	if err := binary.Write(writer, binary.BigEndian, patch.PatchedLength); err != nil {
		return err
	}
	if _, err := writer.Write(patch.PatchedChecksum[:]); err != nil {
		return err
	}

	// Write patch items count
	itemCount := uint32(len(patch.PatchItems))
	if err := binary.Write(writer, binary.BigEndian, itemCount); err != nil {
		return err
	}

	// Write patch items
	for _, item := range patch.PatchItems {
		//fmt.Printf("\rOn Writing patch file: %d/%d", i, len(patch.PatchItems)+1)

		if err := binary.Write(writer, binary.BigEndian, item.Offset); err != nil {
			return err
		}
		if err := binary.Write(writer, binary.BigEndian, uint32(len(item.Content))); err != nil {
			return err
		}
		if _, err := writer.Write(item.Content); err != nil {
			return err
		}
	}

	return nil
}

func writePatchFilev2(patch *PatchFile, bufWriter *bufio.Writer) error {
	// Write magic identifier
	if _, err := bufWriter.Write([]byte(IDENTIFIER)); err != nil {
		return err
	}

	// Write version
	if _, err := bufWriter.Write([]byte{VERSION_MAJOR, VERSION_MINOR}); err != nil {
		return err
	}

	// Write original file info
	if err := binary.Write(bufWriter, binary.BigEndian, patch.OriginalLength); err != nil {
		return err
	}
	if _, err := bufWriter.Write(patch.OriginalChecksum[:]); err != nil {
		return err
	}

	// Write patched file info
	if err := binary.Write(bufWriter, binary.BigEndian, patch.PatchedLength); err != nil {
		return err
	}
	if _, err := bufWriter.Write(patch.PatchedChecksum[:]); err != nil {
		return err
	}

	// Write patch items count
	itemCount := uint32(len(patch.PatchItems))
	if err := binary.Write(bufWriter, binary.BigEndian, itemCount); err != nil {
		return err
	}

	// Write patch items
	for _, item := range patch.PatchItems {
		if err := binary.Write(bufWriter, binary.BigEndian, item.Offset); err != nil {
			return err
		}
		if err := binary.Write(bufWriter, binary.BigEndian, uint32(len(item.Content))); err != nil {
			return err
		}
		if _, err := bufWriter.Write(item.Content); err != nil {
			return err
		}
	}

	// Flush the buffered writer
	if err := bufWriter.Flush(); err != nil {
		return err
	}

	return nil
}

/*
Reads and validates a patch file.

Validation steps:

-	1. Verifies magic identifier

- 	2. Checks version compatibility

- 	3. Reads file metadata

- 	4. Loads patch items
*/
func readPatchFile(reader io.Reader) (*PatchFile, error) {
	// Read and verify magic identifier
	magic := make([]byte, len(IDENTIFIER))
	if _, err := reader.Read(magic); err != nil {
		return nil, err
	}
	if string(magic) != IDENTIFIER {
		return nil, errors.New("invalid patch file format")
	}

	// Read and verify version
	version := make([]byte, 2)
	if _, err := reader.Read(version); err != nil {
		return nil, err
	}
	if version[0] != VERSION_MAJOR || version[1] != VERSION_MINOR {
		return nil, errors.New("unsupported patch version")
	}

	patch := &PatchFile{}

	// Read original file info
	if err := binary.Read(reader, binary.BigEndian, &patch.OriginalLength); err != nil {
		return nil, err
	}
	if _, err := reader.Read(patch.OriginalChecksum[:]); err != nil {
		return nil, err
	}

	// Read patched file info
	if err := binary.Read(reader, binary.BigEndian, &patch.PatchedLength); err != nil {
		return nil, err
	}
	if _, err := reader.Read(patch.PatchedChecksum[:]); err != nil {
		return nil, err
	}

	// Read patch items
	var itemCount uint32
	if err := binary.Read(reader, binary.BigEndian, &itemCount); err != nil {
		return nil, err
	}

	patch.PatchItems = make([]PatchItem, itemCount)
	for i := uint32(0); i < itemCount; i++ {
		//fmt.Printf("\rOn Reading patch file: %d/%d", i, itemCount)

		var offset, length uint32
		if err := binary.Read(reader, binary.BigEndian, &offset); err != nil {
			return nil, err
		}
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			return nil, err
		}

		content := make([]byte, length)
		if _, err := reader.Read(content); err != nil {
			return nil, err
		}

		patch.PatchItems[i] = PatchItem{
			Offset:  offset,
			Content: content,
		}
	}

	return patch, nil
}

func readPatchFilev2(bufReader *bufio.Reader) (*PatchFile, error) {
	// Read and verify magic identifier
	magic := make([]byte, len(IDENTIFIER))
	if _, err := io.ReadFull(bufReader, magic); err != nil {
		return nil, err
	}
	if string(magic) != IDENTIFIER {
		return nil, errors.New("invalid patch file format")
	}

	// Read and verify version
	version := make([]byte, 2)
	if _, err := io.ReadFull(bufReader, version); err != nil {
		return nil, err
	}
	if version[0] != VERSION_MAJOR || version[1] != VERSION_MINOR {
		return nil, errors.New("unsupported patch version")
	}

	patch := &PatchFile{}

	// Read original file info
	if err := binary.Read(bufReader, binary.BigEndian, &patch.OriginalLength); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(bufReader, patch.OriginalChecksum[:]); err != nil {
		return nil, err
	}

	// Read patched file info
	if err := binary.Read(bufReader, binary.BigEndian, &patch.PatchedLength); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(bufReader, patch.PatchedChecksum[:]); err != nil {
		return nil, err
	}

	// Read patch items
	var itemCount uint32
	if err := binary.Read(bufReader, binary.BigEndian, &itemCount); err != nil {
		return nil, err
	}

	patch.PatchItems = make([]PatchItem, itemCount)
	for i := uint32(0); i < itemCount; i++ {
		var offset, length uint32
		if err := binary.Read(bufReader, binary.BigEndian, &offset); err != nil {
			return nil, err
		}
		if err := binary.Read(bufReader, binary.BigEndian, &length); err != nil {
			return nil, err
		}

		content := make([]byte, length)
		if _, err := io.ReadFull(bufReader, content); err != nil {
			return nil, err
		}

		patch.PatchItems[i] = PatchItem{
			Offset:  offset,
			Content: content,
		}
	}

	return patch, nil
}

/*
# Applies a patch to an original file to create the modified version.

Safety features:
  - Validates original file length
  - Verifies original file checksum
  - Ensures correct patched file length
  - Validates final checksum
  - Handles dynamic buffer resizing
*/
func applyPatch(original []byte, patch *PatchFile) ([]byte, error) {
	// Verify original file
	if uint32(len(original)) != patch.OriginalLength {
		return nil, errors.New("original file length mismatch")
	}
	if actualChecksum := sha256.Sum256(original); actualChecksum != patch.OriginalChecksum {
		return nil, errors.New("original file checksum mismatch")
	}

	// Create modified file buffer
	modified := make([]byte, patch.PatchedLength)
	if len(original) < len(modified) {
		copy(modified, original)
	} else {
		copy(modified, original[:len(modified)])
	}

	// Apply patches
	for _, item := range patch.PatchItems {
		//fmt.Printf("\rOn Patching File: %d/%d", i, len(patch.PatchItems)+1)

		num := int(item.Offset + uint32(len(item.Content)))
		if num > len(modified) {
			modified = append(modified, make([]byte, num-len(modified))...)
		}
		copy(modified[item.Offset:], item.Content)
	}

	// Verify result
	if uint32(len(modified)) != patch.PatchedLength {
		flog.Info(uint32(len(modified)), patch.PatchedLength)
		return nil, errors.New("patched file length mismatch")
	}
	if actualChecksum := sha256.Sum256(modified); actualChecksum != patch.PatchedChecksum {
		return nil, errors.New("patched file checksum mismatch")
	}

	return modified, nil
}

func runv2(original, modified []byte) error {
	start := time.Now()
	patch, err := generatePatch(original, modified)
	if err != nil {
		return fmt.Errorf("Error generating patch: %w", err)
	}

	// Write patch to file
	patchFile, err := os.Create("patchv2.mtgadiff")
	if err != nil {
		return fmt.Errorf("Error creating patch file: %v", err)
	}
	defer patchFile.Close()

	// Wrap the file with bufio.Writer
	bufWriter := bufio.NewWriter(patchFile)

	start = time.Now()
	if err := writePatchFilev2(patch, bufWriter); err != nil {
		return fmt.Errorf("error writing patch file: %v", err)
	}

	// Read patch from file
	var bufReader *bufio.Reader
	if patchFile, err := os.Open("patchv2.mtgadiff"); err != nil {
		return fmt.Errorf("Error opening patch file: %v", err)
	} else {
		bufReader = bufio.NewReader(patchFile)
	}

	start = time.Now()
	readPatch, err := readPatchFilev2(bufReader)
	if err != nil {
		return fmt.Errorf("Error reading patch:", err)
	}

	// Apply patch
	start = time.Now()
	result, err := applyPatch(original, readPatch)
	if err != nil {
		return fmt.Errorf("Error applying patch: %v", err)
	}

	// flog.Info("Original:", original)
	// flog.Info("Modified:", modified)
	// flog.Info("Result:", result)
	flog.Info("Patch successful:", bytes.Equal(modified, result))
	if err := os.WriteFile("outputv2.dll", result, 0644); err != nil {
		return err
	}

	elapsed := time.Since(start)
	fmt.Printf("\nrunv2 took %s\n", elapsed)
	return nil
}

func run(original, modified []byte) error {
	// Generate patch
	start := time.Now()
	patch, err := generatePatch(original, modified)
	if err != nil {
		return err
	}

	// Write patch to file
	patchFile, err := os.Create("patch.mtgadiff")
	if err != nil {
		return err
	}
	defer patchFile.Close()

	if err := writePatchFile(patch, patchFile); err != nil {
		return err
	}

	//Read patch from file
	patchFile, err = os.Open("patch.mtgadiff")
	if err != nil {
		return err
	}

	//patchFile.Seek(0, 0)
	readPatch, err := readPatchFile(patchFile)
	if err != nil {
		return err
	}

	// Apply patch
	result, err := applyPatch(original, readPatch)
	if err != nil {
		return err
	}

	// flog.Info("Original:", original)
	// flog.Info("Modified:", modified)
	// flog.Info("Result:", result)
	flog.Info("Patch successful:", bytes.Equal(modified, result))
	if err := os.WriteFile("output.dll", result, 0644); err != nil {
		return err
	}

	elapsed := time.Since(start)
	fmt.Printf("\nrun took %s\n", elapsed)
	return nil
}

func main() {

	// Example usage
	original, err := os.ReadFile("Assembly-CSharp.dll.spt")
	if err != nil {
		flog.Error("Oringal file does not exist", err)
		return
	}
	modified, err := os.ReadFile("Assembly-CSharp.dll")
	if err != nil {
		flog.Error("new file does not exist:", err)
		return
	}

	if err := run(original, modified); err != nil {
		flog.Error("Error running patch:", err)
		return
	}

	if err := runv2(original, modified); err != nil {
		flog.Error(err)
		return
	}

	//Just patching this time
}

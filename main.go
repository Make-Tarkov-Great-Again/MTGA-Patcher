package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/Make-Tarkov-Great-Again/flog/v4/flog"
)

const (
	IDENTIFIER    = "MTGADIFF"
	VERSION_MAJOR = 0x01
	VERSION_MINOR = 0x00
)

type PatchItem struct {
	Offset  uint32
	Content []byte
}

type PatchFile struct {
	OriginalLength   uint32
	OriginalChecksum [32]byte
	PatchedLength    uint32
	PatchedChecksum  [32]byte
	PatchItems       []PatchItem
}

// Generate a patch by comparing two files
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

	minLength := int(math.Min(float64(len(original)), float64(len(modified))))
	var currentData []byte
	diffOffsetStart := 0

	// Compare byte by byte up to the minimum length
	for i := 0; i < minLength; i++ {
		fmt.Printf("\rOn Generating patch file: %d/%d (\"Max\" is estimation)", i, minLength)

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

// Write patch to file in specified format
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
	for i, item := range patch.PatchItems {
		fmt.Printf("\rOn Writing patch file: %d/%d", i, len(patch.PatchItems)+1)

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

// Read patch from file
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
		fmt.Printf("\rOn Reading patch file: %d/%d", i, itemCount)

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

// Apply patch to original file
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
	for i, item := range patch.PatchItems {
		fmt.Printf("\rOn Patching File: %d/%d", i, len(patch.PatchItems)+1)

		if int(item.Offset+uint32(len(item.Content))) > len(modified) {
			modified = append(modified, make([]byte, int(item.Offset+uint32(len(item.Content)))-len(modified))...)
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

	// // Generate patch
	// patch, err := generatePatch(original, modified)
	// if err != nil {
	// 	flog.Error("Error generating patch:", err)
	// 	return
	// }

	// // Write patch to file
	// patchFile, err := os.Create("patch.mtgadiff")
	// if err != nil {
	// 	flog.Error("Error creating patch file:", err)
	// 	return
	// }
	// defer patchFile.Close()

	// if err := writePatchFile(patch, patchFile); err != nil {
	// 	flog.Error("Error writing patch:", err)
	// 	return
	// }

	// Read patch from file

	patchFile, _ := os.Open("patch.mtgadiff")
	patchFile.Seek(0, 0)
	readPatch, err := readPatchFile(patchFile)
	if err != nil {
		flog.Error("Error reading patch:", err)
		return
	}

	// Apply patch
	result, err := applyPatch(original, readPatch)
	if err != nil {
		flog.Error("Error applying patch:", err)
		return
	}

	// flog.Info("Original:", original)
	// flog.Info("Modified:", modified)
	// flog.Info("Result:", result)
	flog.Info("Patch successful:", bytes.Equal(modified, result))
	os.WriteFile("output.dll", result, 0644)
	//Just patching this time
}

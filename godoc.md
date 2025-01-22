# MTGA Binary Patch Utility Documentation

## Overview
This utility implements a binary file differencing and patching system. It creates, writes, and applies patches between two binary files using a custom patch format identified by the "MTGADIFF" magic number. The system ensures data integrity through SHA-256 checksums and supports files of different sizes.

## File Format Specification

### Header Structure
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

### Patch Item Structure
Each patch item contains:
- Offset: uint32 (4 bytes, big-endian)
- Content Length: uint32 (4 bytes, big-endian)
- Content: variable-length byte array

## Core Components

### PatchItem Structure
```go
type PatchItem struct {
    Offset  uint32   // Position in the file where the patch should be applied
    Content []byte   // The actual patch data
}
```

### PatchFile Structure
```go
type PatchFile struct {
    OriginalLength   uint32     // Length of the original file
    OriginalChecksum [32]byte   // SHA-256 hash of original file
    PatchedLength    uint32     // Length of the resulting patched file
    PatchedChecksum  [32]byte   // SHA-256 hash of patched file
    PatchItems       []PatchItem // List of patches to apply
}
```

## Key Functions

### generatePatch(original, modified []byte) (*PatchFile, error)
Generates a patch by comparing two binary files byte by byte.

Key features:
- Validates input files are not empty
- Returns early if files are identical
- Handles files of different sizes
- Creates patches for different sections
- Includes additional data if modified file is longer

### writePatchFile(patch *PatchFile, writer io.Writer) error
Writes a patch to a file in the specified format.

Writing sequence:
1. Magic identifier
2. Version information
3. Original file metadata
4. Patched file metadata
5. Number of patch items
6. Individual patch items

### readPatchFile(reader io.Reader) (*PatchFile, error)
Reads and validates a patch file.

Validation steps:
1. Verifies magic identifier
2. Checks version compatibility
3. Reads file metadata
4. Loads patch items

### applyPatch(original []byte, patch *PatchFile) ([]byte, error)
Applies a patch to an original file to create the modified version.

Safety features:
- Validates original file length
- Verifies original file checksum
- Ensures correct patched file length
- Validates final checksum
- Handles dynamic buffer resizing

## Error Handling
The utility includes comprehensive error checking for:
- File format validation
- Version compatibility
- File length mismatches
- Checksum verification
- I/O operations
- Buffer operations

## Usage Examples

### Creating a Patch
```go
original, err := os.ReadFile("original.dll")
modified, err := os.ReadFile("modified.dll")
patch, err := generatePatch(original, modified)
patchFile, err := os.Create("patch.mtgadiff")
err = writePatchFile(patch, patchFile)
```

### Applying a Patch
```go
patchFile, err := os.Open("patch.mtgadiff")
readPatch, err := readPatchFile(patchFile)
result, err := applyPatch(original, readPatch)
```

## Progress Reporting
The utility includes progress reporting during:
- Patch generation
- Patch writing
- Patch reading
- Patch application

Progress is displayed using console output with current/total item counts.

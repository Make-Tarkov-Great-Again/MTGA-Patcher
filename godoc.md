# MTGA Binary Patch Utility Documentation  
**Version 1.0**  

---

## Table of Contents  
1. [Overview](#overview)  
2. [File Format Specification](#file-format-specification)  
   - [Header Structure](#header-structure)  
   - [Patch Item Structure](#patch-item-structure)  
3. [Core Functionality](#core-functionality)  
   - [Patch Generation](#patch-generation)  
   - [Patch Application](#patch-application)  
4. [Error Handling & Validation](#error-handling--validation)  
5. [API Reference](#api-reference)  
6. [Usage Examples](#usage-examples)  
7. [Performance Considerations](#performance-considerations)  
8. [Integration with MTGA Ecosystem](#integration-with-mtga-ecosystem)  
9. [Troubleshooting](#troubleshooting)  
10. [Contributing](#contributing)  

---

## Overview  
The **MTGA Binary Patch Utility** is a high-performance tool for creating and applying binary diffs/patches between file versions. Designed for the *Escape From Tarkov* modding ecosystem, it enables:  

- **Efficient Version Control**: Create minimal patches between binary files (e.g., game DLLs)  
- **Safe Mod Distribution**: Verify file integrity via SHA-256 checksums  
- **Cross-Version Compatibility**: Handle files of different sizes seamlessly  

Key Features:  
- Custom binary patch format (`*.mtgadiff`)  
- Versioned file headers for backward compatibility  
- Buffered I/O operations for large files  
- Progress reporting during critical operations  

---

## File Format Specification  
### Header Structure  
| Field               | Type           | Size (Bytes) | Description                          |  
|---------------------|----------------|--------------|--------------------------------------|  
| Magic Identifier    | ASCII String   | 8            | `MTGADIFF` (file format signature)   |  
| Version             | uint16         | 2            | Major (0x01) + Minor (0x00) version  |  
| Original Length     | uint32 (BE)    | 4            | Original file size                   |  
| Original SHA-256    | byte[32]       | 32           | Original file checksum               |  
| Patched Length      | uint32 (BE)    | 4            | Patched file size                    |  
| Patched SHA-256     | byte[32]       | 32           | Patched file checksum                |  
| Patch Items Count   | uint32 (BE)    | 4            | Number of patch items                |  

### Patch Item Structure  
| Field          | Type           | Size (Bytes) | Description                          |  
|----------------|----------------|--------------|--------------------------------------|  
| Offset         | uint32 (BE)    | 4            | File position to apply patch        |  
| Content Length | uint32 (BE)    | 4            | Length of patch data                |  
| Content        | byte[]         | Variable     | Raw bytes to write at offset        |  

---

## Core Functionality  
### Patch Generation (`generatePatch`)  
**Algorithm**:  
1. Validate input files are non-empty  
2. Compare original/modified files byte-by-byte  
3. Identify contiguous different regions  
4. Handle size mismatches via append operations  

**Optimizations**:  
- Early exit if files are identical  
- Buffer reuse for memory efficiency  
- Batched write operations  

### Patch Application (`applyPatch`)  
**Safety Measures**:  
1. Verify original file length/checksum  
2. Validate final patched file checksum  
3. Dynamic buffer expansion for oversized patches  

**Edge Case Handling**:  
- Original file shorter than patched: Append new data  
- Original file longer: Truncate excess data  

---

## Error Handling & Validation  
### Error Types  
| Error Code               | Description                          | Recovery Strategy               |  
|--------------------------|--------------------------------------|----------------------------------|  
| `ERR_EMPTY_INPUT`        | Empty input file                     | Validate files before processing |  
| `ERR_CHECKSUM_MISMATCH`  | SHA-256 verification failed          | Re-download source files         |  
| `ERR_VERSION_MISMATCH`   | Unsupported patch version            | Upgrade utility                  |  
| `ERR_IO_OPERATION`       | File read/write failure              | Check permissions/disk space     |  

### Validation Workflow  
```plaintext
1. Verify Magic Header → 2. Check Version → 3. Validate Original File → 4. Apply Patches → 5. Verify Result
```

---

## API Reference  
### Key Functions  
#### `func generatePatch(original, modified []byte) (*PatchFile, error)`  
- **Parameters**:  
  - `original`: Byte slice of original file  
  - `modified`: Byte slice of modified file  
- **Returns**:  
  - `PatchFile` struct or error  

#### `func applyPatch(original []byte, patch *PatchFile) ([]byte, error)`  
- **Preconditions**:  
  - Original file matches `patch.OriginalChecksum`  
- **Postconditions**:  
  - Output matches `patch.PatchedChecksum`  

---

## Usage Examples  

### Programmatic Usage  
```go
// Create patch
original, _ := os.ReadFile("v1.dll")
modified, _ := os.ReadFile("v2.dll")
patch, _ := generatePatch(original, modified)

// Save to file
f, _ := os.Create("update.mtgadiff")
writePatchFile(patch, f)

// Apply later
patchFile, _ := os.Open("update.mtgadiff")
loadedPatch, _ := readPatchFile(patchFile)
result, _ := applyPatch(original, loadedPatch)
os.WriteFile("v2_patched.dll", result, 0644)
```

---

## Performance Considerations  
### Buffered I/O (v2 Functions)  
- Uses `bufio.Reader`/`Writer` for batch operations  
- Reduces system calls by 80-90% for large files  
- Recommended for files >10MB  

### Memory Management  
- Avoids full-file loading with stream processing (future roadmap)  
- Reusable byte buffers in critical paths  

---

## Integration with MTGA Ecosystem  
### FLog Integration  
```go
import "github.com/Make-Tarkov-Great-Again/flog/v4/flog"

// Example usage
flog.Info("Patch applied successfully")
flog.Error("Checksum mismatch", err)
```

### Launcher Compatibility  
Designed to work with:  
- [MTGA-Launcher](https://github.com/Make-Tarkov-Great-Again/MTGO-Launcher)  
- [Event Horizon Lite](https://github.com/EFHDev/Event-Horizion-Lite)  

---

## Troubleshooting  
### Common Issues  
| Symptom                     | Likely Cause               | Solution                          |  
|-----------------------------|----------------------------|-----------------------------------|  
| "Invalid patch format"      | Corrupted header           | Verify file integrity             |  
| "Checksum mismatch"         | Modified original file     | Use exact original from patch     |  
| Slow patch generation       | Large file size            | Use `v2` functions with buffering |  

---

## Contributing  
### Development Guidelines  
1. Follow Go idiomatic style  
2. Add tests for new features in `*_test.go`  
3. Update checksum logic if format changes  

### Roadmap  
- Delta compression algorithms (BSDiff integration)  (Maybe)
- Multithreaded patch generation  (If it will even help, we will see)
- Streaming API for large files  

---

### Version History  
| Version | Date       | Changes                     |  
|---------|------------|-----------------------------|  
| v1.0    | 2023-11-01 | Initial release             |  
| v1.1    | 2024-01-15 | Buffered I/O optimizations  |  

---

**License**: BSD 2-Clause License (modified)  
**Support**: [MTGA Discord](https://discord.gg/xvJUAjewFw)

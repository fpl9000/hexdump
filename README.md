# Hex Dump Utility

A graphical hex dump tool built with Go and the Fyne GUI toolkit. This application displays the content of files in hexadecimal format with various viewing options.

## Features

### Hex Display Options
- **Byte Grouping**: Display bytes in groups of 1, 2, 4, 8, or 16 bytes
- **Address Column**: Shows file offsets in hexadecimal format
- **Configurable Layout**: 16 bytes per line by default

### Character Display Options
- **ISO Latin-1**: Single-byte character encoding
- **UTF-8**: Unicode encoding (variable-length)
- **UTF-16LE**: Unicode encoding (little-endian, 2 bytes per character)
- **GB 18030**: Chinese character encoding

### GUI Features
- **Resizable Interface**: Fully resizable window with proper scaling
- **Split View**: Hex display on the left, character display on the right
- **File Operations**: Open files through file dialog or menu
- **Status Bar**: Shows current file name and size
- **Synchronized Display**: Character count matches hex data on each line

## Usage

### Running the Application
```bash
# Run without arguments (opens empty window)
./hexdump.exe

# Run with a file argument (loads file immediately)
./hexdump.exe filename.txt
```

### Opening a File
1. Click the "Open File" button in the toolbar, or
2. Use the File menu → Open
3. Select any file from the file dialog

### Changing Display Options
- **Byte Grouping**: Use the dropdown in the toolbar to select 1, 2, 4, 8, or 16 bytes per group
- **Character Encoding**: Use the encoding dropdown to select how bytes are interpreted as characters

### Interface Layout
```
┌─────────────────────────────────────────────────────────────┐
│ Menu Bar: File | Options                                    │
├─────────────────────────────────────────────────────────────┤
│ Toolbar: [Open File] [Byte Grouping: ▼] [Encoding: ▼]     │
├─────────────────────────────────────────────────────────────┤
│ ┌─────────────────────────┬─────────────────────────────────┐ │
│ │ Hex Display Panel       │ Character Display Panel        │ │
│ │                         │                                 │ │
│ │ 00000000: 48 65 6C 6C   │ Hell                           │ │
│ │ 00000004: 6F 20 57 6F   │ o Wo                           │ │
│ │ 00000008: 72 6C 64 21   │ rld!                           │ │
│ │                         │                                 │ │
│ └─────────────────────────┴─────────────────────────────────┘ │
│ Status Bar: File: example.txt | Size: 12 bytes             │
└─────────────────────────────────────────────────────────────┘
```

## Building from Source

### Prerequisites
- Go 1.24.2 or later
- MinGW-w64 (for Windows builds)
- Fyne v2.6.1 (automatically installed via go mod)

### Build Commands
```bash
# Standard build
PATH="/cygdrive/c/apps/msys64/mingw64/bin:$PATH" go build -o hexdump.exe

# Build without console window (recommended for GUI)
PATH="/cygdrive/c/apps/msys64/mingw64/bin:$PATH" go build -ldflags "-H windowsgui" -o hexdump.exe
```

### Dependencies
The application uses the following Go modules:
- `fyne.io/fyne/v2` - GUI toolkit
- `golang.org/x/text` - Text encoding support

## Character Encoding Details

### ISO Latin-1
- Single-byte encoding
- Supports ASCII (0-127) and extended Latin characters (160-255)
- Non-printable characters displayed as dots (.)

### UTF-8
- Variable-length Unicode encoding
- Handles multi-byte characters correctly
- Invalid sequences displayed as dots (.)

### UTF-16LE
- Little-endian 16-bit Unicode encoding
- Reads bytes in pairs (little-endian order)
- Odd number of bytes at end displayed as dots (.)

### GB 18030
- Chinese character encoding
- Supports simplified Chinese characters
- Invalid sequences displayed as dots (.)

## File Support
The application can open and display any file type:
- Text files (.txt, .md, .log, etc.)
- Binary files (.exe, .dll, .bin, etc.)
- Image files (.jpg, .png, .gif, etc.)
- Any other file format

## Technical Details
- **Memory Efficient**: Handles large files by loading them entirely into memory
- **Cross-Platform**: Built with Fyne for cross-platform compatibility
- **Monospace Display**: Uses monospace fonts for proper alignment
- **Error Handling**: Graceful handling of file read errors and encoding issues

## License
This project is licensed under the terms specified in the LICENSE file.

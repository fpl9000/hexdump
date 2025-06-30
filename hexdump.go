package main

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	nativedialog "github.com/sqweek/dialog"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Package-scope variable to cache the debug setting at startup.
var debugEnabled bool

// init function runs once when the package is initialized.
func init() {
	debugEnabled = os.Getenv("HEXDUMP_DEBUG") != ""

	if debugEnabled {
		fmt.Println("DEBUG: HexDumpApp initialized with debug mode enabled")
	}
}

// HexDumpApp represents the main application structure
type HexDumpApp struct {
	app    fyne.App
	window fyne.Window

	// File data
	fileData []byte
	fileName string

	// GUI components
	// hexDisplay      *widget.Label // Removed
	// charDisplay     *widget.Label // Removed
	byteGroupSelect *widget.Select
	encodingSelect  *widget.Select
	statusLabel     *widget.Label
	// scrollContainer *container.Scroll // Removed
	dataList *widget.List // Added

	// Settings
	bytesPerGroup int
	encoding      string
	bytesPerLine  int

	// Display metrics
	totalLines int
}

// NewHexDumpApp creates a new hex dump application instance
func NewHexDumpApp(app fyne.App, window fyne.Window) *HexDumpApp {
	return &HexDumpApp{
		app:           app,
		window:        window,
		bytesPerGroup: 1,
		encoding:      "ISO Latin-1",
		bytesPerLine:  16,
	}
}

// setupGUI initializes and sets up the GUI components
func (h *HexDumpApp) setupGUI() {
	// Create menu
	h.createMenu()

	// Create main content area first (this initializes the display widgets)
	content := h.createMainContent()

	// Create toolbar (this can now safely set default values)
	toolbar := h.createToolbar()

	// Create status bar
	statusBar := h.createStatusBar()

	// Combine all components
	mainContainer := container.NewBorder(
		container.NewVBox(toolbar),
		statusBar,
		nil,
		nil,
		content,
	)

	h.window.SetContent(mainContainer)
}

// createMenu creates the application menu
func (h *HexDumpApp) createMenu() {
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Open file...", h.openFile),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			h.app.Quit()
		}),
	)

	optionsMenu := fyne.NewMenu("Options",
		fyne.NewMenuItem("About", h.showAbout),
	)

	mainMenu := fyne.NewMainMenu(fileMenu, optionsMenu)
	h.window.SetMainMenu(mainMenu)
}

// createToolbar creates the toolbar with controls
func (h *HexDumpApp) createToolbar() *fyne.Container {
	// Open file button
	openBtn := widget.NewButton("Open File...", h.openFile)

	// Byte grouping selector
	h.byteGroupSelect = widget.NewSelect(
		[]string{"1 byte", "2 bytes", "4 bytes", "8 bytes", "16 bytes"},
		h.onByteGroupChanged,
	)
	h.byteGroupSelect.SetSelected("1 byte")

	// Encoding selector
	h.encodingSelect = widget.NewSelect(
		[]string{"ISO Latin-1", "UTF-8", "UTF-16LE", "GB 18030"},
		h.onEncodingChanged,
	)
	h.encodingSelect.SetSelected("ISO Latin-1")

	// Create toolbar content
	toolbarContent := container.NewHBox(
		openBtn,
		widget.NewSeparator(),
		widget.NewLabel("Byte Grouping:"),
		h.byteGroupSelect,
		widget.NewSeparator(),
		widget.NewLabel("Encoding:"),
		h.encodingSelect,
	)

	// Create light background for toolbar
	lightGray := color.RGBA{R: 45, G: 45, B: 45, A: 255}
	background := canvas.NewRectangle(lightGray)

	return container.NewStack(background, toolbarContent)
}

// createMainContent creates the main content area using widget.List.
func (h *HexDumpApp) createMainContent() fyne.CanvasObject {
	h.dataList = widget.NewList(
		h.listLength,
		h.listCreateItem,
		h.listUpdateItem,
	)
	// Hide separators to eliminate space between line rectangles
	h.dataList.HideSeparators = true
	return h.dataList
}

// createStatusBar creates the status bar
func (h *HexDumpApp) createStatusBar() *fyne.Container {
	h.statusLabel = widget.NewLabel("Ready")

	// Create status bar content
	statusContent := container.NewHBox(h.statusLabel)

	// Create light background for status bar
	lightGray := color.RGBA{R: 45, G: 45, B: 45, A: 255}
	background := canvas.NewRectangle(lightGray)

	return container.NewStack(background, statusContent)
}

// synchronizeScrolling function REMOVED

// openFile opens a native Windows file dialog and loads the selected file
func (h *HexDumpApp) openFile() {
	filename, err := nativedialog.File().Filter("All Files", "*").Load()
	if err != nil {
		// Check if user cancelled the dialog
		if err.Error() != "Cancelled" {
			dialog.ShowError(err, h.window)
		}
		return
	}

	h.loadFileFromPath(filename)
}

// loadFileFromPath loads a file from the given file path
func (h *HexDumpApp) loadFileFromPath(filePath string) {
	// Read the entire file at once
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		dialog.ShowError(err, h.window)
		return
	}

	// Set file data and name
	h.fileData = fileData
	h.fileName = filePath

	// Update display and status
	h.updateDisplay()
	h.updateStatus()
}

// onByteGroupChanged handles byte grouping selection changes
func (h *HexDumpApp) onByteGroupChanged(value string) {
	switch value {
	case "1 byte":
		h.bytesPerGroup = 1
	case "2 bytes":
		h.bytesPerGroup = 2
	case "4 bytes":
		h.bytesPerGroup = 4
	case "8 bytes":
		h.bytesPerGroup = 8
	case "16 bytes":
		h.bytesPerGroup = 16
	}
	h.updateDisplay()
}

// onEncodingChanged handles encoding selection changes
func (h *HexDumpApp) onEncodingChanged(value string) {
	h.encoding = value
	h.updateDisplay()
}

// updateDisplay updates the dataList
func (h *HexDumpApp) updateDisplay() {
	if h.dataList == nil { // Check if dataList is initialized
		return
	}

	if len(h.fileData) == 0 {
		h.totalLines = 0
		h.dataList.Refresh()
		return
	}

	// Calculate total lines needed
	h.totalLines = (len(h.fileData) + h.bytesPerLine - 1) / h.bytesPerLine

	// The actual updating of list items will be handled by widget.List's
	// UpdateItem callback, which will use generateHexLine and generateCharLine.
	// For now, just refresh the list.
	h.dataList.Refresh()
}

// listLength returns the number of items in the list (number of lines).
func (h *HexDumpApp) listLength() int {
	if h.fileData == nil || h.bytesPerLine == 0 {
		return 0
	}
	return (len(h.fileData) + h.bytesPerLine - 1) / h.bytesPerLine
}

// listCreateItem creates a new template item for the list.
func (h *HexDumpApp) listCreateItem() fyne.CanvasObject {
	// Use canvas.Text for better control over text positioning and size
	hexText := canvas.NewText("HEX_PLACEHOLDER", color.White)
	hexText.TextStyle.Monospace = true
	hexText.TextSize = 12 // Smaller font size to fit in reduced height

	charText := canvas.NewText("CHAR_PLACEHOLDER", color.White)
	charText.TextStyle.Monospace = true
	charText.TextSize = 12 // Smaller font size to fit in reduced height

	// Create a spacer to separate hex data from character data for better readability
	spacer := canvas.NewText("          ", color.Transparent) // Invisible spacer text
	spacer.TextStyle.Monospace = true
	// Set the same font size as hex and char text for alignment
	spacer.TextSize = 12

	// Use HBox with spacer between hex and character data
	return container.NewHBox(hexText, spacer, charText)
}

// listUpdateItem updates the content of a list item.
func (h *HexDumpApp) listUpdateItem(id widget.ListItemID, item fyne.CanvasObject) {
	if h.fileData == nil {
		return // No data to display
	}
	// The item is now an HBox container with hex, spacer, and char text objects
	hbox := item.(*fyne.Container)
	hexText := hbox.Objects[0].(*canvas.Text)
	// spacer is at index 1, skip it
	charText := hbox.Objects[2].(*canvas.Text)

	offset := id * h.bytesPerLine
	if offset >= len(h.fileData) {
		// This case should ideally not be reached if listLength is correct
		hexText.Text = ""
		charText.Text = ""
		hexText.Refresh()
		charText.Refresh()
		return
	}

	hexAndAddrStr := h.generateHexLine(offset) // This includes address
	charStr := h.generateCharLine(offset)

	hexText.Text = strings.TrimSpace(hexAndAddrStr)

    // Maybe pad hexText.Text with spaces to align the character text with the previous line.  Here,
    // 10 is the width of the hex address column (including the trailing space), 32 is the maximum
    // number of hex digits in one line, and `(16/h.bytesPerGroup) - 1` is the maximum number of
    // spaces between the hex digits in one line.
    hexAndAddrColumns := 10 + 32 + (16/h.bytesPerGroup) - 1
    hexTextLength := len(hexText.Text)
    paddingLength := hexAndAddrColumns - hexTextLength

    if paddingLength > 0 {
        hexText.Text += strings.Repeat(" ", paddingLength)
    }

	charText.Text = strings.TrimSpace(charStr)
	hexText.Refresh()
	charText.Refresh()

	// Set a custom height for this list item to reduce vertical padding
	// Use 18 pixels to accommodate the smaller 12pt font with minimal padding
	h.dataList.SetItemHeight(id, 18) // Slightly increased to prevent text clipping
}

// generateHexLine generates a single hex line
func (h *HexDumpApp) generateHexLine(offset int) string {
	var builder strings.Builder
	dataLen := len(h.fileData)

	// Write address
	builder.WriteString(fmt.Sprintf("%08X: ", offset))

	// Write hex bytes
	lineEnd := offset + h.bytesPerLine
	if lineEnd > dataLen {
		lineEnd = dataLen
	}

	for index := offset; index < lineEnd; index += h.bytesPerGroup {
		groupEnd := index + h.bytesPerGroup
		if groupEnd > lineEnd {
			groupEnd = lineEnd
		}

		// Write bytes in group
		for byteIndex := index; byteIndex < groupEnd; byteIndex++ {
			builder.WriteString(fmt.Sprintf("%02X", h.fileData[byteIndex]))
		}

		// Add space after group (except for last group on line)
		if groupEnd < lineEnd {
			builder.WriteString(" ")
		}
	}

	// Pad line if necessary
	bytesOnLine := lineEnd - offset
	if bytesOnLine < h.bytesPerLine {
		// Calculate padding needed
		groupsOnLine := (bytesOnLine + h.bytesPerGroup - 1) / h.bytesPerGroup
		totalGroups := (h.bytesPerLine + h.bytesPerGroup - 1) / h.bytesPerGroup
		missingGroups := totalGroups - groupsOnLine

		// Add padding for missing bytes
		for index := bytesOnLine; index < h.bytesPerLine; index++ {
			builder.WriteString("  ")
		}
		// Add padding for missing group separators
		for index := 0; index < missingGroups; index++ {
			builder.WriteString(" ")
		}
	}

	builder.WriteString("\n")                         // Newline might not be needed for List items
	return strings.TrimRight(builder.String(), "\n ") // Trim trailing space/newline for list display
}

// generateHexDisplay generates the hexadecimal display content (legacy method for compatibility)
// This will likely be removed or adapted when widget.List is fully integrated.
func (h *HexDumpApp) generateHexDisplay() string {
	var builder strings.Builder
	dataLen := len(h.fileData)

	for offset := 0; offset < dataLen; offset += h.bytesPerLine {
		builder.WriteString(h.generateHexLine(offset))
	}

	return builder.String()
}

// generateCharLine generates a single character line
func (h *HexDumpApp) generateCharLine(offset int) string {
	dataLen := len(h.fileData)
	lineEnd := offset + h.bytesPerLine
	if lineEnd > dataLen {
		lineEnd = dataLen
	}

	lineData := h.fileData[offset:lineEnd]
	chars := h.bytesToChars(lineData)

//	// Pad the character string with spaces to align the last line.
//	numRunes := utf8.RuneCountInString(chars)
//	if numRunes < h.bytesPerLine {
//		padding := strings.Repeat(" ", h.bytesPerLine-numRunes)
//		chars += padding
//	}

	return chars // Newline might not be needed for List items
}

// generateCharDisplay generates the character display content (legacy method for compatibility)
// This will likely be removed or adapted when widget.List is fully integrated.
func (h *HexDumpApp) generateCharDisplay() string {
	var builder strings.Builder
	dataLen := len(h.fileData)

	for offset := 0; offset < dataLen; offset += h.bytesPerLine {
		builder.WriteString(h.generateCharLine(offset))
		builder.WriteString("\n") // Add newline if generating full display text
	}
	return builder.String()
}

// bytesToChars converts bytes to characters based on the selected encoding
func (h *HexDumpApp) bytesToChars(data []byte) string {
	switch h.encoding {
	case "ISO Latin-1":
		return h.bytesToLatin1(data)
	case "UTF-8":
		return h.bytesToUTF8(data)
	case "UTF-16LE":
		return h.bytesToUTF16LE(data)
	case "GB 18030":
		return h.bytesToGB18030(data)
	default:
		return h.bytesToLatin1(data)
	}
}

// bytesToLatin1 converts bytes to ISO Latin-1 characters
func (h *HexDumpApp) bytesToLatin1(data []byte) string {
	var builder strings.Builder
	for _, b := range data {
		if b >= 32 && b <= 126 {
			// Printable ASCII
			builder.WriteByte(b)
		} else if b >= 160 && b <= 255 {
			// Extended Latin-1
			builder.WriteRune(rune(b))
		} else {
			// Non-printable
			builder.WriteString(".")
		}
	}
	return builder.String()
}

// bytesToUTF8 converts bytes to UTF-8 characters
func (h *HexDumpApp) bytesToUTF8(data []byte) string {
	var builder strings.Builder
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8
			builder.WriteString(".")
			data = data[1:]
		} else {
			if unicode.IsPrint(r) {
				builder.WriteRune(r)
			} else {
				builder.WriteString(".")
			}
			data = data[size:]
		}
	}
	return builder.String()
}

// bytesToUTF16LE converts bytes to UTF-16LE characters
func (h *HexDumpApp) bytesToUTF16LE(data []byte) string {
	var builder strings.Builder

	// Ensure we have pairs of bytes
	for index := 0; index < len(data); index += 2 {
		if index+1 >= len(data) {
			// Odd number of bytes, show as dot
			builder.WriteString(".")
			break
		}

		// Read UTF-16LE (little-endian)
		low := uint16(data[index])
		high := uint16(data[index+1])
		codeUnit := low | (high << 8)

		// Convert to rune
		runes := utf16.Decode([]uint16{codeUnit})
		if len(runes) > 0 && unicode.IsPrint(runes[0]) {
			builder.WriteRune(runes[0])
		} else {
			builder.WriteString(".")
		}
	}
	return builder.String()
}

// bytesToGB18030 converts bytes to GB 18030 characters
func (h *HexDumpApp) bytesToGB18030(data []byte) string {
	decoder := simplifiedchinese.GB18030.NewDecoder()
	result, _, err := transform.Bytes(decoder, data)
	if err != nil {
		// Fallback to showing dots for invalid sequences
		var builder strings.Builder
		for range data {
			builder.WriteString(".")
		}
		return builder.String()
	}

	// Filter out non-printable characters
	var builder strings.Builder
	for _, r := range string(result) {
		if unicode.IsPrint(r) {
			builder.WriteRune(r)
		} else {
			builder.WriteString(".")
		}
	}
	return builder.String()
}

// updateStatus updates the status bar
func (h *HexDumpApp) updateStatus() {
	if h.fileName == "" {
		h.statusLabel.SetText("Ready")
	} else {
		h.statusLabel.SetText(fmt.Sprintf("File: %s | Size: %d bytes", h.fileName, len(h.fileData)))
	}
}

// showAbout shows the about dialog
func (h *HexDumpApp) showAbout() {
	dialog.ShowInformation("About", "Hex Dump Utility\n\nA graphical hex dump tool built with Fyne.\nSupports multiple byte groupings and character encodings.", h.window)
}

package main

import (
	"fmt"
	"image/color"
	"io"
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

// HexDumpApp represents the main application structure
type HexDumpApp struct {
	app    fyne.App
	window fyne.Window

	// File data
	fileData []byte
	fileName string

	// GUI components
	hexDisplay      *widget.Entry
	charDisplay     *widget.Entry
	byteGroupSelect *widget.Select
	encodingSelect  *widget.Select
	statusLabel     *widget.Label
	scrollContainer *container.Scroll

	// Settings
	bytesPerGroup int
	encoding      string
	bytesPerLine  int

	// Virtual scrolling
	visibleLines    int
	currentTopLine  int
	totalLines      int
	lineHeight      float32
	cachedHexLines  map[int]string
	cachedCharLines map[int]string
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

// createMainContent creates the main content area with hex and character displays
func (h *HexDumpApp) createMainContent() *container.Split {
	// Initialize virtual scrolling parameters
	h.visibleLines = 30 // Show ~30 lines at a time
	h.currentTopLine = 0
	h.cachedHexLines = make(map[int]string)
	h.cachedCharLines = make(map[int]string)

	// Create hex display - using Entry for better performance
	h.hexDisplay = widget.NewMultiLineEntry()
	h.hexDisplay.Wrapping = fyne.TextWrapOff
	h.hexDisplay.Disable() // Make it read-only

	// Create character display
	h.charDisplay = widget.NewMultiLineEntry()
	h.charDisplay.Wrapping = fyne.TextWrapOff
	h.charDisplay.Disable() // Make it read-only

	// Create scroll containers
	hexScroll := container.NewScroll(h.hexDisplay)
	charScroll := container.NewScroll(h.charDisplay)

	// Synchronize scrolling between the two panels
	h.synchronizeScrolling(hexScroll, charScroll)

	// Create split container
	split := container.NewHSplit(hexScroll, charScroll)
	split.SetOffset(0.7) // 70% for hex, 30% for characters

	return split
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

// synchronizeScrolling synchronizes scrolling between hex and character displays
func (h *HexDumpApp) synchronizeScrolling(hexScroll, charScroll *container.Scroll) {
	// Store reference to the scroll containers for potential future use
	h.scrollContainer = hexScroll

	// Create a flag to prevent infinite recursion during synchronization
	var syncing bool

	// Synchronize hex scroll to character scroll
	hexScroll.OnScrolled = func(position fyne.Position) {
		if syncing {
			return
		}
		syncing = true
		charScroll.Offset = position
		charScroll.Refresh()

		// Update virtual scrolling based on scroll position
		h.updateVirtualScroll(position.Y)
		syncing = false
	}

	// Synchronize character scroll to hex scroll
	charScroll.OnScrolled = func(position fyne.Position) {
		if syncing {
			return
		}
		syncing = true
		hexScroll.Offset = position
		hexScroll.Refresh()

		// Update virtual scrolling based on scroll position
		h.updateVirtualScroll(position.Y)
		syncing = false
	}
}

// updateVirtualScroll updates the virtual scrolling based on scroll position
func (h *HexDumpApp) updateVirtualScroll(scrollY float32) {
	if len(h.fileData) == 0 {
		return
	}

	// Estimate line height (this is approximate)
	if h.lineHeight == 0 {
		h.lineHeight = 20 // Default line height in pixels
	}

	// Calculate which line should be at the top
	newTopLine := int(scrollY / h.lineHeight)
	if newTopLine < 0 {
		newTopLine = 0
	}
	if newTopLine >= h.totalLines {
		newTopLine = h.totalLines - 1
	}

	// Only update if the top line changed significantly
	if abs(newTopLine-h.currentTopLine) > 5 {
		h.currentTopLine = newTopLine
		h.refreshVisibleContent()
	}
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// refreshVisibleContent refreshes only the visible content
func (h *HexDumpApp) refreshVisibleContent() {
	if h.hexDisplay == nil || h.charDisplay == nil {
		return
	}

	hexContent := h.generateVisibleHexDisplay()
	charContent := h.generateVisibleCharDisplay()

	h.hexDisplay.SetText(hexContent)
	h.charDisplay.SetText(charContent)
}

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
	// Open and read file data
	file, err := os.Open(filePath)
	if err != nil {
		dialog.ShowError(err, h.window)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		dialog.ShowError(err, h.window)
		return
	}

	h.fileData = data
	h.fileName = filePath
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

// updateDisplay updates both hex and character displays using virtual scrolling
func (h *HexDumpApp) updateDisplay() {
	// Safety check: ensure widgets are initialized
	if h.hexDisplay == nil || h.charDisplay == nil {
		return
	}

	if len(h.fileData) == 0 {
		h.hexDisplay.SetText("")
		h.charDisplay.SetText("")
		return
	}

	// Calculate total lines needed
	h.totalLines = (len(h.fileData) + h.bytesPerLine - 1) / h.bytesPerLine

	// Clear cache when settings change
	h.cachedHexLines = make(map[int]string)
	h.cachedCharLines = make(map[int]string)

	// Generate visible content only
	hexContent := h.generateVisibleHexDisplay()
	charContent := h.generateVisibleCharDisplay()

	h.hexDisplay.SetText(hexContent)
	h.charDisplay.SetText(charContent)
}

// generateVisibleHexDisplay generates only the visible portion of hex display
func (h *HexDumpApp) generateVisibleHexDisplay() string {
	var builder strings.Builder
	dataLen := len(h.fileData)

	// Calculate which lines to show
	endLine := h.currentTopLine + h.visibleLines
	if endLine > h.totalLines {
		endLine = h.totalLines
	}

	for lineNum := h.currentTopLine; lineNum < endLine; lineNum++ {
		// Check cache first
		if cachedLine, exists := h.cachedHexLines[lineNum]; exists {
			builder.WriteString(cachedLine)
			continue
		}

		// Generate line
		offset := lineNum * h.bytesPerLine
		if offset >= dataLen {
			break
		}

		lineContent := h.generateHexLine(offset)
		h.cachedHexLines[lineNum] = lineContent
		builder.WriteString(lineContent)
	}

	return builder.String()
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

	builder.WriteString("\n")
	return builder.String()
}

// generateHexDisplay generates the hexadecimal display content (legacy method for compatibility)
func (h *HexDumpApp) generateHexDisplay() string {
	var builder strings.Builder
	dataLen := len(h.fileData)

	for offset := 0; offset < dataLen; offset += h.bytesPerLine {
		builder.WriteString(h.generateHexLine(offset))
	}

	return builder.String()
}

// generateVisibleCharDisplay generates only the visible portion of character display
func (h *HexDumpApp) generateVisibleCharDisplay() string {
	var builder strings.Builder
	dataLen := len(h.fileData)

	// Calculate which lines to show
	endLine := h.currentTopLine + h.visibleLines
	if endLine > h.totalLines {
		endLine = h.totalLines
	}

	for lineNum := h.currentTopLine; lineNum < endLine; lineNum++ {
		// Check cache first
		if cachedLine, exists := h.cachedCharLines[lineNum]; exists {
			builder.WriteString(cachedLine)
			continue
		}

		// Generate line
		offset := lineNum * h.bytesPerLine
		if offset >= dataLen {
			break
		}

		lineContent := h.generateCharLine(offset)
		h.cachedCharLines[lineNum] = lineContent
		builder.WriteString(lineContent)
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
	return chars + "\n"
}

// generateCharDisplay generates the character display content (legacy method for compatibility)
func (h *HexDumpApp) generateCharDisplay() string {
	var builder strings.Builder
	dataLen := len(h.fileData)

	for offset := 0; offset < dataLen; offset += h.bytesPerLine {
		builder.WriteString(h.generateCharLine(offset))
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

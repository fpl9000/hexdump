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
}

// HexDumpApp represents the main application structure
type HexDumpApp struct {
	app    fyne.App
	window fyne.Window

	// File data
	fileData []byte
	fileName string

	// Progressive loading
	file          *os.File
	fileSize      int64
	loadedBytes   int64
	chunkSize     int
	loadMoreBtn   *widget.Button
	progressLabel *widget.Label
	isLoading     bool

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
		chunkSize:     1024 * 1024, // Default 1MB chunks
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

	// Load More button (initially hidden)
	h.loadMoreBtn = widget.NewButton("Load More", h.loadNextChunk)
	h.loadMoreBtn.Hide()

	// Progress label (initially hidden)
	h.progressLabel = widget.NewLabel("")
	h.progressLabel.Hide()

	// Create toolbar content
	toolbarContent := container.NewHBox(
		openBtn,
		widget.NewSeparator(),
		widget.NewLabel("Byte Grouping:"),
		h.byteGroupSelect,
		widget.NewSeparator(),
		widget.NewLabel("Encoding:"),
		h.encodingSelect,
		widget.NewSeparator(),
		h.loadMoreBtn,
		h.progressLabel,
	)

	// Create light background for toolbar
	lightGray := color.RGBA{R: 45, G: 45, B: 45, A: 255}
	background := canvas.NewRectangle(lightGray)

	return container.NewStack(background, toolbarContent)
}

// createMainContent creates the main content area with hex and character displays
func (h *HexDumpApp) createMainContent() *container.Split {
	// Create hex display - using widget.Entry for better performance
	h.hexDisplay = widget.NewMultiLineEntry()
	h.hexDisplay.Wrapping = fyne.TextWrapOff
	h.hexDisplay.TextStyle.Monospace = true
	h.hexDisplay.Disable() // Make read-only

	// Create character display
	h.charDisplay = widget.NewMultiLineEntry()
	h.charDisplay.Wrapping = fyne.TextWrapOff
	h.charDisplay.TextStyle.Monospace = true
	h.charDisplay.Disable() // Make read-only

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

	// Debug: Print when scroll synchronization is set up
	if debugEnabled {
		fmt.Println("DEBUG: Setting up scroll synchronization")
	}

	// Synchronize hex scroll to character scroll
	hexScroll.OnScrolled = func(position fyne.Position) {
		if debugEnabled {
			fmt.Printf("DEBUG: Hex scroll event - Position: X=%.2f, Y=%.2f, Syncing=%t\n", position.X, position.Y, syncing)
		}
		if syncing {
			if debugEnabled {
				fmt.Println("DEBUG: Hex scroll - skipping due to syncing flag")
			}
			return
		}
		syncing = true
		if debugEnabled {
			fmt.Printf("DEBUG: Hex scroll - setting char scroll offset to X=%.2f, Y=%.2f\n", position.X, position.Y)
		}
		charScroll.Offset = position
		charScroll.Refresh()
		syncing = false
		if debugEnabled {
			fmt.Println("DEBUG: Hex scroll - synchronization complete")
		}

		// Check for auto-load on scroll (Phase 3)
		h.checkAutoLoad(hexScroll)
	}

	// Synchronize character scroll to hex scroll
	charScroll.OnScrolled = func(position fyne.Position) {
		if debugEnabled {
			fmt.Printf("DEBUG: Char scroll event - Position: X=%.2f, Y=%.2f, Syncing=%t\n", position.X, position.Y, syncing)
		}
		if syncing {
			if debugEnabled {
				fmt.Println("DEBUG: Char scroll - skipping due to syncing flag")
			}
			return
		}
		syncing = true
		if debugEnabled {
			fmt.Printf("DEBUG: Char scroll - setting hex scroll offset to X=%.2f, Y=%.2f\n", position.X, position.Y)
		}
		hexScroll.Offset = position
		hexScroll.Refresh()
		syncing = false
		if debugEnabled {
			fmt.Println("DEBUG: Char scroll - synchronization complete")
		}

		// Check for auto-load on scroll (Phase 3)
		h.checkAutoLoad(hexScroll)
	}

	if debugEnabled {
		fmt.Println("DEBUG: Scroll synchronization setup complete")
	}
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

// loadFileFromPath loads a file from the given file path using progressive loading
func (h *HexDumpApp) loadFileFromPath(filePath string) {
	// Close any previously opened file
	if h.file != nil {
		h.file.Close()
		h.file = nil
	}

	// Open file for progressive reading
	file, err := os.Open(filePath)
	if err != nil {
		dialog.ShowError(err, h.window)
		return
	}

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		dialog.ShowError(err, h.window)
		return
	}

	// Initialize progressive loading state
	h.file = file
	h.fileName = filePath
	h.fileSize = fileInfo.Size()
	h.loadedBytes = 0
	h.fileData = nil // Clear existing data
	h.isLoading = false

	// Load first chunk
	h.loadNextChunk()
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

// updateDisplay updates both hex and character displays
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

	// Generate complete file content
	hexContent := h.generateHexDisplay()
	charContent := h.generateCharDisplay()

	// Set text directly - Entry widgets use monospace font by default
	h.hexDisplay.SetText(hexContent)
	h.charDisplay.SetText(charContent)
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

// loadNextChunk loads the next chunk of data from the file
func (h *HexDumpApp) loadNextChunk() {
	if h.file == nil || h.isLoading {
		return
	}

	// Check if we've reached the end of the file
	if h.loadedBytes >= h.fileSize {
		h.updateLoadMoreButton()
		return
	}

	h.isLoading = true
	h.updateLoadMoreButton()

	// Calculate how much to read
	remainingBytes := h.fileSize - h.loadedBytes
	chunkSize := int64(h.chunkSize)
	if remainingBytes < chunkSize {
		chunkSize = remainingBytes
	}

	// Read the chunk
	chunkData := make([]byte, chunkSize)
	n, err := h.file.Read(chunkData)
	if err != nil && err.Error() != "EOF" {
		dialog.ShowError(err, h.window)
		h.isLoading = false
		h.updateLoadMoreButton()
		return
	}

	if n > 0 {
		// Append to existing data
		h.fileData = append(h.fileData, chunkData[:n]...)
		h.loadedBytes += int64(n)

		// Update display
		h.updateDisplay()
		h.updateStatus()
	}

	h.isLoading = false
	h.updateLoadMoreButton()
}

// updateLoadMoreButton updates the Load More button and progress label visibility and state
func (h *HexDumpApp) updateLoadMoreButton() {
	if h.file == nil {
		// No file loaded
		h.loadMoreBtn.Hide()
		h.progressLabel.Hide()
		return
	}

	// Show progress
	progressText := fmt.Sprintf("Loaded %d of %d MB", h.loadedBytes/(1024*1024), h.fileSize/(1024*1024))
	h.progressLabel.SetText(progressText)
	h.progressLabel.Show()

	if h.loadedBytes >= h.fileSize {
		// File fully loaded
		h.loadMoreBtn.Hide()
		h.progressLabel.SetText(fmt.Sprintf("Complete: %d MB", h.fileSize/(1024*1024)))
	} else if h.isLoading {
		// Currently loading
		h.loadMoreBtn.SetText("Loading...")
		h.loadMoreBtn.Disable()
		h.loadMoreBtn.Show()
	} else {
		// More data available
		h.loadMoreBtn.SetText("Load More")
		h.loadMoreBtn.Enable()
		h.loadMoreBtn.Show()
	}
}

// checkAutoLoad checks if we should automatically load more data based on scroll position (Phase 3)
func (h *HexDumpApp) checkAutoLoad(scroll *container.Scroll) {
	// Only auto-load if we have more data to load and we're not already loading
	if h.file == nil || h.isLoading || h.loadedBytes >= h.fileSize {
		return
	}

	// Get the content size and viewport size
	contentSize := h.hexDisplay.Size()
	viewportSize := scroll.Size()

	// Calculate scroll percentage
	// We need to check if we're near the bottom (90% threshold)
	if contentSize.Height > 0 && viewportSize.Height > 0 {
		maxScrollY := contentSize.Height - viewportSize.Height
		if maxScrollY > 0 {
			scrollPercentage := scroll.Offset.Y / maxScrollY

			if debugEnabled {
				fmt.Printf("DEBUG: Auto-load check - ScrollY: %.2f, MaxScrollY: %.2f, Percentage: %.2f%%\n",
					scroll.Offset.Y, maxScrollY, scrollPercentage*100)
			}

			// If we're at 90% or more, trigger auto-load
			if scrollPercentage >= 0.9 {
				if debugEnabled {
					fmt.Println("DEBUG: Auto-load triggered - loading next chunk")
				}
				go h.loadNextChunk() // Load in background to avoid blocking UI
			}
		}
	}
}

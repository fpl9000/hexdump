package main

import (
	"image/color"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"
)

// CustomTheme creates a theme with white text
type CustomTheme struct {
	fyne.Theme
}

// NewCustomTheme creates a new custom theme
func NewCustomTheme() fyne.Theme {
	return &CustomTheme{Theme: theme.DefaultTheme()}
}

// Color returns the color for the given theme color name
func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameForeground:
		return color.RGBA{R: 255, G: 255, B: 255, A: 255} // White text
	case theme.ColorNameDisabled:
		return color.RGBA{R: 255, G: 255, B: 255, A: 255} // White text for disabled widgets
	default:
		return t.Theme.Color(name, variant)
	}
}

func main() {
	// Create the application
	myApp := app.New()
	myApp.Settings().SetTheme(NewCustomTheme())
	myApp.SetIcon(nil)

	// Create the main window
	myWindow := myApp.NewWindow("Hex Dump Utility")
	myWindow.Resize(fyne.NewSize(1000, 600))

	// Create the hex dump application instance
	hexApp := NewHexDumpApp(myApp, myWindow)

	// Set up the GUI
	hexApp.setupGUI()

	// Check for command-line arguments to load a file
	if len(os.Args) > 1 {
		filename := os.Args[1]
		hexApp.loadFileFromPath(filename)
	}

	// Show the window and run the application
	myWindow.ShowAndRun()
}

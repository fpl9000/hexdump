package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	// Create the application
	myApp := app.New()
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

package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
)

var wg sync.WaitGroup //Create the waitgroup object

// Takes in the file path of an image and returns the width and height
// Used to determine whether an image is portrait or landscape orientation
// imagePath is an absolute path to the image in question
func getImageDimension(imagePath string) (int, int) {

	// Open the file
	file, err := os.Open(imagePath)
	if err != nil {
		log.Fatal(err)
	}

	// Decode the image into an image object
	image, _, err := image.DecodeConfig(file)
	if err != nil {
		log.Fatal(err)
	}

	// Return the width and height of the image from the image object
	return image.Width, image.Height
}

// Generates a thumbnail of a JPG file. Takes in the src path of the file and then saves it to the dst path
// src is an absolute path of the JPG to generate a thumbnail from
// dst is the absolute path to save the thumbnail to
// height is the desired height for the jpg to be resized to
// width is the desired width for the jpg to be resized to
// quality is the percentage of quality the jpg should be taken down to. Should be between 1 and 99. Example: 80
func createThumbnail(src string, dst string, height int, width int, quality int) {

	defer wg.Done() //Schedule with the waitgroup

	// Holds the resized jpg
	var thumbnail image.Image

	// Tells whether the image is portrait or landscape orientation
	var orientation string

	// Determine orientation, annotate it in the orientation variable
	width1, height1 := getImageDimension(src)
	if width1 > height1 {
		orientation = "landscape"
	} else if height1 > width1 {
		orientation = "portrait"
	}

	// Open the image
	orig, err := imaging.Open(src, imaging.AutoOrientation(true))
	if err != nil {
		log.Fatalf("Failed to open image: %v", err)
	}

	// Resize the image
	if orientation == "landscape" {
		thumbnail = imaging.Resize(orig, height, width, imaging.Lanczos)
	} else if orientation == "portrait" {
		thumbnail = imaging.Resize(orig, width, height, imaging.Lanczos)
	}

	// Save the resulting image as JPEG.
	err = imaging.Save(thumbnail, dst, imaging.JPEGQuality(quality))
	if err != nil {
		log.Fatalf("Failed to save image: %v", err)
	}

}

// Creates thumbnails of all of the jpg files in a directory
// Saves them as <filename>_thumb.jpg
// dir is the directory to target
// height is the desired height for the jpg to be resized to
// width is the desired width for the jpg to be resized to
// quality is the percentage of quality the jpg should be taken down to. Should be between 1 and 99. Example: 80
// maxroutines is the max number of concurrent goroutines you would like at a time. Higher = higher CPU and Memory usage
func thumbnailDir(dir string, height int, width int, quality int, maxroutines int) {

	routines := 0            // Number of goroutines
	start_time := time.Now() // Timer start time

	// Get all files in the provided directory
	// Store it in variable photos
	photos, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	// Iterate through the photos
	// If there is not a thumbnail already existing, create one
	for _, photo := range photos {

		// If the file is not a thumbnail
		if !(strings.Contains(photo.Name(), "_thumb")) {

			// Generate certain variables to be used on each photo
			photoPath := fmt.Sprintf("%v/%v", dir, photo.Name())                      // Absolute filepath to the photo
			noSuffixName := strings.TrimSuffix(photo.Name(), filepath.Ext(photoPath)) // Name of the photo without the file extension
			thumbnailName := fmt.Sprintf("%v/%v_thumb.jpg", dir, noSuffixName)        // Absolute filepath for the thumbnail file. Used for save path

			// Check to see if thumbnail of the file already exists
			_, err := os.OpenFile(thumbnailName, os.O_RDWR, 0644)
			if errors.Is(err, os.ErrNotExist) {

				// Only execute on files that are jpg
				if strings.ToLower(filepath.Ext(photoPath)) == ".jpg" || strings.ToLower(filepath.Ext(photoPath)) == ".jpeg" {

					wg.Add(1)                                                            // Add a Go routine to the waitlist
					routines += 1                                                        // Add one to the number of active goroutines
					go createThumbnail(photoPath, thumbnailName, height, width, quality) // Start goroutine to create a thumbnail of the jpg

					// If the number of active goroutines reaches the max desired concurrent goroutines, wait for them to finish
					// Reset counter to zero then continue
					if routines >= maxroutines {
						wg.Wait()
						routines = 0
					}
				}
			}
		}
	}
	wg.Wait()                          // Wait for all the goroutines to finish
	duration := time.Since(start_time) // Calculate execution duration
	fmt.Println(duration)              // Print execution duration
}

func main() {

	args := os.Args
	if len(args) < 6 {
		log.Fatal("Usage: dir, height, width, quality, maxroutines")
	}
	height, _ := strconv.Atoi(args[2])
	width, _ := strconv.Atoi(args[3])
	quality, _ := strconv.Atoi(args[4])
	maxroutines, _ := strconv.Atoi(args[5])
	thumbnailDir(args[1], height, width, quality, maxroutines)
}

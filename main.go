package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	nocache "github.com/alexander-melentyev/gin-nocache"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
)

var wg sync.WaitGroup //Create the waitgroup object

// Lists all the objects in an S3 Bucket prefix
// client is the S3 client object. Used to connect to the S3 service
// region is a string annotating the AWS region to be used. Example: "us-east-2"
// bucket is a string annotating the S3 bucket to be used. Example "ryans-test-bucket675"
// prefix is a string annotating the prefix within the bucket to be targeting
// maxkeys is an int64 to set the max number of objects to return
func getObjects(client *s3.S3, region string, bucket string, prefix string, maxkeys int64) []string {

	var final []string // Holds the final value for the return

	// List objects in the bucket + prefix
	objects, err := client.ListObjects(&s3.ListObjectsInput{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxkeys,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Append the object keys to a slice to return
	for _, key := range objects.Contents {
		if *key.Size > 0 {
			final = append(final, *key.Key)
		}
	}
	return final
}

// Takes list of objects in the S3 prefix and creates presigned urls for them
// Returns a string slice containing the urls
// client is the S3 client object. Used to connect to the S3 service
// bucket is a string annotating the S3 bucket to be used. Example "ryans-test-bucket675"
// keys is a slice of the object keys in an S3 bucket prefix
// minutes is the number of minutes the presigned urls should be good for
func createUrls(client *s3.S3, bucket string, keys []string, minutes int64) map[string]string {

	final := make(map[string]string)

	// iterate through objects keys from the bucket + prefix
	for _, key := range keys {

		// Create the request object using the key + bucket
		req, _ := client.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})

		// Generate presigned url for x minutes using the request object
		urlStr, err := req.Presign(time.Duration(minutes) * time.Minute)
		if err != nil {
			log.Println("Failed to sign request", err)
		}

		// Append the url to final for return
		key = key[strings.LastIndex(key, "/")+1 : strings.LastIndex(key, "_thumb")]
		final[key] = urlStr
		fmt.Println(key)
	}

	return final
}

// Takes in a string slice of presigned urls and generates the html page to send to the user
// Returns the HTML as a string
// keys is a slice of the presigned urls to be used in the gallery
func createHTML(keys map[string]string) string {

	// Holds final value for return
	// In this case it is a string that holds the HTML to send to the user
	var final string

	// Add first part of the HTML to the string
	// Yes I am sure there are much better ways to do this, it is on the to-do list
	final += `<!doctype html>
	<html lang="en">
	<head>
	<link rel="stylesheet" href="gallery.css">
	<meta charset="utf-8">
	
	<title>Image Gallery</title>
	<meta name="description" content="Responsive Image Gallery">
	<meta name="author" content="Ryan Strom">
	
	<style type="text/css">
	</style>
	</head>
	<body>
	<div id="gallery">
	`

	// Iterate through the slice of urls and add them as images to the HTML
	for key, url := range keys {
		final += fmt.Sprintf("<a id=\"%v\" href=\"%v\" target=\"_blank\"><img src=\"%v\"></a>\n", key, url, url)
	}

	// Add the final part of the HTML
	final += `</div></body></html>`

	return final
}

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

	port := ""   // Port to listen on
	region := "" // AWS region to be used
	bucket := ""
	prefix := ""
	var minutes int64 = 20   // Number of minutes the the presigned urls will be good for
	var maxkeys int64 = 1000 // Max number of objects to get from the S3 prefix

	// Create S3 service client based on the configuration
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		log.Fatal(err)
	}
	client := s3.New(sess)

	// Initialize Gin
	gin.SetMode(gin.ReleaseMode)                // Turn off debugging mode
	r := gin.Default()                          // Initialize Gin
	r.Use(nocache.NoCache())                    // Sets gin to disable browser caching
	r.StaticFile("/gallery.css", "gallery.css") // Tells Gin to send the gallery.css file when requested

	//Route for health check
	r.GET("/ping", func(c *gin.Context) {

		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})

	})

	// Route to request the image gallery
	r.GET("/", func(c *gin.Context) {
		objects := getObjects(client, region, bucket, prefix, maxkeys)   // Get the prefix objects
		urls := createUrls(client, bucket, objects, minutes)             // Generate the presigned urls
		html := createHTML(urls)                                         // Generate the HTML
		c.Data(http.StatusOK, "text/html; charaset-utf-8", []byte(html)) // Send the HTML to the client
	})

	fmt.Printf("Listening on port %v...", port) //Notifies that server is running on X port
	r.Run(port)                                 //Start running the Gin server
}

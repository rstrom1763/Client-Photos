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

func getObjects(client *s3.S3, region string, bucket string, prefix string, maxkeys int64) []string {

	var final []string

	objects, err := client.ListObjects(&s3.ListObjectsInput{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxkeys,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, key := range objects.Contents {
		if *key.Size > 0 {
			final = append(final, *key.Key)
		}
	}
	return final
}

func createUrls(client *s3.S3, bucket string, keys []string, minutes int64) []string {

	var final []string

	for _, key := range keys {
		req, _ := client.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		urlStr, err := req.Presign(time.Duration(minutes) * time.Minute)
		if err != nil {
			log.Println("Failed to sign request", err)
		}
		final = append(final, urlStr)
	}

	return final
}

func createHTML2(keys []string) string {

	var final string

	final += `<!doctype html>
	<html lang="en">
	
	<head><link rel="stylesheet" href="test.css"><style type="text/css"></style></head><br><br><br><br><br><br><br><br><br><br>
	
	<body style="background-color:rgb(168, 168, 168);">
		<div id="gallery">
	`

	for _, key := range keys {
		final += fmt.Sprintf("<img src=\"%v\" class=\"thumbnail\">\n", key)
	}

	final += "</div></body></html>"
	return final
}

func createHTML(keys []string) string {

	var final string

	final += `<!doctype html>
	<html lang="en">
	 <head>
	 <link rel="stylesheet" href="test.css">
	  <meta charset="utf-8">
	  
	  <title>Image Gallery</title>
	  <meta name="description" content="Responsive Image Gallery">
	  <meta name="author" content="Tim Wells">
	  
	  <style type="text/css">
	  </style>
	</head>
	<body>
	<div id="gallery">
	`

	for _, key := range keys {
		final += fmt.Sprintf("<a href=\"%v\" target=\"_blank\"><img src=\"%v\"></a>\n", key, key)
	}

	final += `</div>
 
	</body>
   </html>`
	return final
}

func getImageDimension(imagePath string) (int, int) {
	file, err := os.Open(imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	image, _, err := image.DecodeConfig(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", imagePath, err)
	}
	return image.Width, image.Height
}

func createThumbnail(src string, dst string, height int, width int, quality int) {
	defer wg.Done() //Schedule with the waitgroup

	var thumbnail image.Image

	var orientation string
	width1, height1 := getImageDimension(src)
	if width1 > height1 {
		orientation = "landscape"
	} else if height1 > width1 {
		orientation = "portrait"
	}

	orig, err := imaging.Open(src)
	if err != nil {
		log.Fatalf("Failed to open image: %v", err)
	}

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

func thumbnailDir(dir string, height int, width int, quality int) {

	routines := 0
	start_time := time.Now()
	photos, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, photo := range photos {
		if !(strings.Contains(photo.Name(), "_thumb")) {
			photoPath := fmt.Sprintf("%v/%v", dir, photo.Name())
			noSuffixName := strings.TrimSuffix(photo.Name(), filepath.Ext(photoPath))
			thumbnailName := fmt.Sprintf("%v/%v_thumb.jpg", dir, noSuffixName)
			_, err := os.OpenFile(thumbnailName, os.O_RDWR, 0644)
			if errors.Is(err, os.ErrNotExist) {
				if strings.ToLower(filepath.Ext(photoPath)) == ".jpg" {
					wg.Add(1) //Add the Go routine to the waitlist
					routines += 1
					go createThumbnail(photoPath, thumbnailName, height, width, quality)
					if routines >= 8 {
						wg.Wait()
						routines = 0
					}
				}
			}
		}
	}
	wg.Wait() //Wait for all the Go routines to finish
	duration := time.Since(start_time)
	fmt.Println(duration)
}

func main() {
	port := ":8081"              //Port to listen on
	gin.SetMode(gin.ReleaseMode) //Turn off debugging mode
	r := gin.Default()           //Initialize Gin
	r.Use(nocache.NoCache())

	//Route for testing functionality
	r.GET("/ping", func(c *gin.Context) {

		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})

	})

	region := "us-east-2" //AWS region to be used
	bucket := "ryans-test-bucket423"
	prefix := "image_host/"
	//key := "image_host/2D3A4246.jpg"
	var minutes int64 = 20
	var maxkeys int64 = 5000

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create S3 service client
	client := s3.New(sess)

	objects := getObjects(client, region, bucket, prefix, maxkeys)
	urls := createUrls(client, bucket, objects, minutes)
	os.WriteFile("./test2.html", []byte(createHTML(urls)), 0644)

	r.StaticFile("/test.css", "test.css")
	r.GET("/", func(c *gin.Context) {
		html := createHTML(urls)
		c.Data(http.StatusOK, "text/html; charaset-utf-8", []byte(html))
	})

	fmt.Printf("Listening on port %v...", port) //Notifies that server is running on X port
	r.Run(port)                                 //Start running the Gin server
}

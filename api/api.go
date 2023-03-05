package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	nocache "github.com/alexander-melentyev/gin-nocache"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// Get key from the env file
func env(key string) string {

	// load .env file
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

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
	<script src="js.js"></script>
	<meta charset="utf-8">
	
	<title>Image Gallery</title>
	<meta name="description" content="Responsive Image Gallery">
	
	<style type="text/css">
	</style>
	</head>
	<body>
	<div id="gallery">
	`

	// Iterate through the slice of urls and add them as images to the HTML
	for key, url := range keys {
		final += fmt.Sprintf("<a id=\"%v\" onclick=\"markImage(this.id)\" alt=\"0\"><img src=\"%v\" ></a>\n", key, url)
	}

	// Add the final part of the HTML
	final += `</div></body></html>`

	return final
}

func main() {
	port := env("PORT")     // Port to listen on
	region := env("REGION") // AWS region to be used
	bucket := env("BUCKET")
	prefix := env("PREFIX")
	var minutes int64
	minutes, _ = strconv.ParseInt(env("MINUTES"), 10, 64) // Number of minutes the the presigned urls will be good for
	var maxkeys int64
	maxkeys, _ = strconv.ParseInt(env("MAXKEYS"), 10, 64) // Max number of objects to get from the S3 prefix

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
	r.StaticFile("js.js", "js.js")

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

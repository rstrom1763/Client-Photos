package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	nocache "github.com/alexander-melentyev/gin-nocache"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username   string `json:"username"`
	First_name string `json:"first_name"`
	Last_name  string `json:"last_name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	City       string `json:"city"`
	State      string `json:"state"`
	Password   string `json:"password"`
	Zip        string `json:"zip"`
}

// Get key from the env file
func env(key string) string {

	// load .env file
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

// This function renews the dynamoDB client on a 4 minute interval
// This prevents security token expiration errors
// Gets put into a goroutine to run in the background
func autoRenewDynamoCreds(svc **dynamodb.DynamoDB) {

	for {

		time.Sleep(time.Minute * 4)

		// snippet-start:[dynamodb.go.create_item.session]
		// Initialize a session that the SDK will use to load
		// credentials from the shared credentials file ~/.aws/credentials
		// and region from the shared configuration file ~/.aws/config.
		dynamoSess := session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))

		// Create DynamoDB client
		*svc = dynamodb.New(dynamoSess)

	}

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
	</head>
	<body>
	<div id="loading-screen">
    <div class="loader"></div>
  </div>
	<div class="navbar" onClick="submit()">
	<a>Submit</a>
	</div>
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

func getUser(tablename string, username string, svc *dynamodb.DynamoDB) (map[string]string, error) {
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tablename),
		Key: map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(username),
			},
		},
	})
	if err != nil {
		log.Fatalf("Got error calling GetItem: %s", err)
	}
	final := make(map[string]string)
	err = dynamodbattribute.UnmarshalMap(result.Item, &final)
	if err != nil {
		log.Fatal(err)
	}

	if final["username"] == "" {
		return nil, errors.New("User does not exist")
	}

	return final, nil
}

func createUser(tablename string, user User, svc *dynamodb.DynamoDB) error {

	var userMap map[string]string
	userJson, err := json.Marshal(user)
	if err != nil {
		errorMessage := "Could not marshal user struct"
		log.Println(errorMessage)
		return errors.New(errorMessage)
	}

	err = json.Unmarshal(userJson, &userMap)

	userMap["username"] = strings.ToLower(userMap["username"]) //Ensure username is all lowercase

	_, err = getUser(tablename, userMap["username"], svc)
	if err == nil {
		return errors.New("User already exists")
	}

	av, err := dynamodbattribute.MarshalMap(userMap)
	if err != nil {
		return errors.New(fmt.Sprintf("Got error marshalling new user item: %s", err))
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tablename),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		return errors.New(fmt.Sprintf("Got error calling PutItem: %s", err))
	}

	return nil

}

// Converts string map to json string
func map2json(object map[string]string) string {
	json, err := json.Marshal(object)
	if err != nil {
		log.Fatal(err)
	}
	return string(json)
}

// Returns error code and ends handler function for gin routes
func abortWithError(statusCode int, err error, c *gin.Context) {

	c.AbortWithError(statusCode, err)
	c.JSON(statusCode, gin.H{"status": fmt.Sprint(err)})

}

func generateSSL() {

	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal("Error generating private key:", err)
		return
	}

	// Generate a self-signed certificate
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		log.Fatal("Error creating certificate:", err)
		return
	}

	// Write the private key and certificate to files
	keyOut, err := os.Create("./private.key")
	if err != nil {
		log.Fatal("Error creating private key file:", err)
		return
	}
	defer keyOut.Close()

	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	certOut, err := os.Create("./cert.pem")
	if err != nil {
		log.Fatal("Error creating certificate file:", err)
		return
	}
	defer certOut.Close()

	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	fmt.Println("TLS certificate and private key generated successfully.")
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true // File exists
	}
	if os.IsNotExist(err) {
		return false // File does not exist
	}
	return false // Error occurred (e.g., permission denied)
}

func main() {
	port := env("PORT")           // Port to listen on
	region := env("REGION")       // AWS region to be used
	bucket := env("BUCKET")       // S3 bucket to be referenced
	prefix := env("PREFIX")       // Bucket prefix to use
	tablename := env("TABLENAME") // DynamoDB table to use
	protocol := strings.ToLower(env("PROTOCOL"))
	var minutes int64
	minutes, _ = strconv.ParseInt(env("MINUTES"), 10, 64) // Number of minutes the the presigned urls will be good for
	var maxkeys int64
	maxkeys, _ = strconv.ParseInt(env("MAXKEYS"), 10, 64) // Max number of objects to get from the S3 prefix

	//Ensure valid protocol env entry
	if protocol != "http" && protocol != "https" {
		log.Fatal("Invalid protocol. Must be HTTP or HTTPS")
	}

	//Generate TLS keys if they do not already exist
	if !(fileExists("./cert.pem") && fileExists("./private.key")) && protocol == "https" {
		generateSSL()
	}

	// Create S3 service client based on the configuration
	s3sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		log.Fatal(err)
	}
	client := s3.New(s3sess)

	// Initialize a session that the SDK will use to load
	// credentials from the shared credentials file ~/.aws/credentials
	// and region from the shared configuration file ~/.aws/config.
	dynamoSess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	// Create DynamoDB client session
	svc := dynamodb.New(dynamoSess)
	go autoRenewDynamoCreds(&svc) //Renew client session every 4 minutes to prevent token expiry

	// Initialize Gin
	gin.SetMode(gin.ReleaseMode)                // Turn off debugging mode
	r := gin.Default()                          // Initialize Gin
	r.Use(nocache.NoCache())                    // Sets gin to disable browser caching
	r.StaticFile("/gallery.css", "gallery.css") // Tells Gin to send the gallery.css file when requested
	r.StaticFile("js.js", "js.js")
	r.StaticFile("/favicon.ico", "favicon.ico")

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

	r.POST("/submit", func(c *gin.Context) {

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Fatal(err)
		}

		err = os.WriteFile("./picks.json", body, 0644)
		if err != nil {
			log.Fatal(err)
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
		})

	})

	r.GET("/getSelections", func(c *gin.Context) {

		picks, err := os.ReadFile("./picks.json")
		if err != nil {
			log.Fatal(err)
		}

		var results map[string]any
		json.Unmarshal([]byte(picks), &results)

		c.IndentedJSON(http.StatusOK, results)

	})

	// Get a user from the DB
	r.GET("/user/:username", func(c *gin.Context) {
		username := c.Param("username")
		result, err := getUser(tablename, username, svc)
		if err != nil {
			abortWithError(404, err, c)
			return
		}
		c.JSON(http.StatusOK, result)
	})

	r.POST("/createUser", func(c *gin.Context) {

		// Read the request body into body variable
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			abortWithError(http.StatusInternalServerError, err, c)
			return
		}
		if string(body) == "" {
			err = errors.New("body is empty")
			abortWithError(http.StatusBadRequest, err, c)
			return
		}

		// Unmarshal the body json into a user struct
		var user User
		json.Unmarshal(body, &user)

		//Convert the password from the request body into a salted hash using bcrypt
		var hash []byte
		hash, err = bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			err = errors.New(fmt.Sprintf("Could not hash the password ", err))
			abortWithError(http.StatusInternalServerError, err, c)
			return
		}
		user.Password = string(hash)

		// Create the user in DynamoDB
		err = createUser(tablename, user, svc)
		if err != nil {
			abortWithError(http.StatusInternalServerError, err, c)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
		})
	})

	fmt.Printf("Listening on port %v...\n", port) //Notifies that server is running on X port
	if protocol == "http" {                       //Start running the Gin server
		err = r.Run(":" + port)
		if err != nil {
			fmt.Println(err)
		}
	} else if protocol == "https" {
		err = r.RunTLS(":"+port, "./cert.pem", "./private.key")
		if err != nil {
			fmt.Println(err)
		}
	} else {
		log.Fatal("Something went wrong starting the Gin server")
	}

}

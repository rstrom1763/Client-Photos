package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
	"github.com/things-go/gin-contrib/nocache"
	"golang.org/x/crypto/bcrypt"
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

// This function renews the dynamoDB client on a 4-minute interval
// This prevents security token expiration errors
// Gets put into a goroutine to run in the background
func autoRenewDynamoCredentials(svc **dynamodb.DynamoDB) {

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
		// Nice
	}
}

// Lists all the objects in an S3 Bucket prefix
// client is the S3 client object. Used to connect to the S3 service
// region is a string annotating the AWS region to be used. Example: "us-east-2"
// bucket is a string annotating the S3 bucket to be used. Example "ryans-test-bucket675"
// prefix is a string annotating the prefix within the bucket to be targeting
// maxKeys is an int64 to set the max number of objects to return
func getObjects(client *s3.S3, bucket string, prefix string, page int, pageSize int) ([]string, error) {

	var final []string // Holds the final value for the return
	lowerBound := page * pageSize
	upperBound := (page * pageSize) + pageSize

	// List objects in the bucket + prefix
	objects, err := client.ListObjects(&s3.ListObjectsInput{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: aws.Int64(10000),
	})
	if err != nil {
		return []string{}, err
	}

	// Append the object keys to a slice to return
	for i, key := range objects.Contents {
		if *key.Size > 0 {
			if i >= lowerBound && i < upperBound {
				final = append(final, *key.Key)
			}
		}
	}
	return final, nil
}

// Used to get all of a user's shoots for use in the home page
func getShoots(tableName string, username string, svc *dynamodb.DynamoDB) (string, error) {

	key := map[string]*dynamodb.AttributeValue{
		"username": {
			S: aws.String(username),
		},
	}

	attributeToGet := "shoots"

	input := &dynamodb.GetItemInput{
		TableName:            aws.String(tableName),
		Key:                  key,
		ProjectionExpression: aws.String(attributeToGet),
	}

	result, err := svc.GetItem(input)
	if err != nil {
		return "", errors.New("there was an error getting the shoots")
	}

	var final []Shoot
	var shoots map[string]Shoot
	err = dynamodbattribute.UnmarshalMap(result.Item, &final)

	if result.Item != nil {
		// Unmarshal the DynamoDB item into the Item struct
		err := dynamodbattribute.UnmarshalMap(result.Item["shoots"].M, &shoots)
		if err != nil {
			fmt.Println("Error unmarshalling item:", err)
			return "", errors.New("error unmarshalling item")
		}
	} else {
		fmt.Println("Item not found")
	}

	shootsJSON, _ := json.Marshal(shoots)

	return string(shootsJSON), nil

}

// Takes list of objects in the S3 prefix and creates pre-signed urls for them
// Returns a string slice containing the urls
// client is the S3 client object. Used to connect to the S3 service
// bucket is a string annotating the S3 bucket to be used. Example "ryans-test-bucket675"
// keys is a slice of the object keys in an S3 bucket prefix
// minutes is the number of minutes the pre-signed urls should be good for
func createUrls(client *s3.S3, bucket string, keys []string, minutes int64) ([]Thumbnail, error) {

	var final []Thumbnail

	// iterate through objects keys from the bucket + prefix
	for _, key := range keys {

		urlStr, err := createS3Presigned(bucket, key, minutes, client)
		if err != nil {
			return []Thumbnail{}, nil
		}

		// Append the url to final for return
		key = key[strings.LastIndex(key, "/")+1 : strings.LastIndex(key, "_thumb")]
		final = append(final, Thumbnail{Key: key, Url: urlStr})

	}

	return final, nil
}

// Create the request object using the key + bucket
func createS3Presigned(bucket string, key string, minutes int64, client *s3.S3) (string, error) {
	// Create the request object using the key + bucket
	req, _ := client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Generate pre-signed url for x minutes using the request object
	urlStr, err := req.Presign(time.Duration(minutes) * time.Minute)
	if err != nil {
		return "", err
	}

	return urlStr, nil

}

func generateTiles(inputMAP map[string]Shoot, bucket string, client *s3.S3) ([]HomePageTile, error) {

	var final []HomePageTile

	for key, value := range inputMAP {

		thumbnail, err := createS3Presigned(bucket, value.Thumbnail, 30, client)
		if err != nil {
			return make([]HomePageTile, 0), errors.New("could not generate thumbnail url")
		}

		final = append(final, HomePageTile{Name: key, Thumbnail: thumbnail})
	}

	return final, nil

}

// Takes in a string slice of pre-signed urls and generates the html page to send to the user
// Returns the HTML as a string
// keys is a slice of the pre-signed urls to be used in the gallery
func createHTML(keys []Thumbnail) (string, error) {

	tmpl, err := template.ParseFiles("./static/html/gallery.html")
	if err != nil {
		log.Printf("Could not parse gallery.html")
		return "", err
	}

	var final bytes.Buffer
	err = tmpl.Execute(&final, keys)
	if err != nil {
		log.Printf("Could not execute html template: %v", err)
		return "", err
	}

	return final.String(), nil
}

func generateSalt(length int) (string, error) {
	salt := make([]byte, length)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}

func generateSessionToken() (string, error) {
	token, err := generateSalt(32)
	if err != nil {
		return "", fmt.Errorf("could not generate token: %v", err)
	}
	return token, nil
}

func getUser(tableName string, username string, svc *dynamodb.DynamoDB) (User, error) {
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(username),
			},
		},
	})
	if err != nil {
		return User{}, err
	}

	var final User
	err = dynamodbattribute.UnmarshalMap(result.Item, &final)
	if err != nil {
		return User{}, err
	}

	if final.Username == "" {
		return User{}, errors.New("user does not exist")
	}

	return final, nil
}

func createUser(tableName string, user User, svc *dynamodb.DynamoDB) error {

	user.Username = strings.ToLower(user.Username) //Ensure username is all lowercase
	user.Shoots = make(map[string]Shoot)           //Make sure the property is initialized
	user.Shoots["placeholder"] = Shoot{}

	_, err := getUser(tableName, user.Username, svc)
	if err == nil {
		return errors.New("user already exists")
	}

	av, err := dynamodbattribute.MarshalMap(user)
	if err != nil {
		return fmt.Errorf("got error marshalling new user item: %s", err)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		return fmt.Errorf("got error calling PutItem: %s", err)
	}

	return nil
}

func setToken(r *redis.Client, username string, token string) {
	err := r.Set(username, token, time.Minute*30).Err()
	if err != nil {
		log.Printf("there was a problem setting the token in redis: %v", err)
	}
}

// Returns error code and ends handler function for gin routes
func abortWithError(statusCode int, err error, c *gin.Context) {

	_ = c.AbortWithError(statusCode, err)
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
	certTemplate := x509.Certificate{
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

	derBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &privateKey.PublicKey, privateKey)
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

	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err != nil {
		log.Fatal("Error creating certificate file: ", err)
		return
	}

	certOut, err := os.Create("./cert.pem")
	if err != nil {
		log.Fatal("Error creating certificate file: ", err)
		return
	}
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		log.Fatal("Error creating certificate file: ", err)
		return
	}

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

func resetTokenTimeout(redClient *redis.Client, username string, redisTimeout int) {
	_ = redClient.Expire(username, time.Minute*time.Duration(redisTimeout))
}

func verifyPassword(hashedPassword string, inputPassword string, salt string) bool {
	// Compare the hashed password with the input password
	inputPassword = inputPassword + salt
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(inputPassword))
	return err == nil
}

func checkToken(c *gin.Context, redClient *redis.Client) (bool, string) {

	cookie, err := c.Cookie("authToken")
	if err != nil {
		log.Printf("could not get cookie value: %v", err)
		return false, ""
	}

	var cookieValue map[string]string
	err = json.Unmarshal([]byte(cookie), &cookieValue)
	if err != nil {
		log.Printf("could not unmarshal cookie value: %v", err)
		return false, ""
	}

	val, err := redClient.Get(cookieValue["username"]).Result()
	if err != nil {
		return false, ""
	} else {
		if val == cookieValue["token"] {
			resetTokenTimeout(redClient, cookieValue["username"], 30)
			return true, cookieValue["username"]
		} else {
			return false, ""
		}
	}
}

func updatePicks(tableName string, username string, shootName string, newValue Picks, svc *dynamodb.DynamoDB) error {

	// Define the key to identify the item you want to update
	key := map[string]*dynamodb.AttributeValue{
		"username": {
			S: aws.String(username),
		},
	}

	// Define the update expression to set the new property value
	updateExpression := "SET shoots." + shootName + ".picks = :newValue"

	newPicksMap, _ := dynamodbattribute.MarshalMap(newValue)

	// Define expression attribute values
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{
		":newValue": {
			M: newPicksMap,
		},
	}

	// Configure the update input
	updateInput := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(tableName),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: expressionAttributeValues,
		ReturnValues:              aws.String("UPDATED_NEW"), // If you want to return the updated item
	}

	// Perform the update operation
	_, err := svc.UpdateItem(updateInput)

	return err
}

// Used to load the static files into memory
func cacheStaticFiles() map[string][]byte {

	staticFiles := make(map[string][]byte)

	cssDirHandle, _ := os.Open("./static/css")
	defer cssDirHandle.Close()

	jsDirHandle, _ := os.Open("./static/js")
	defer jsDirHandle.Close()

	cssFiles, _ := cssDirHandle.Readdirnames(0)
	jsFiles, _ := jsDirHandle.Readdirnames(0)

	everythingList := append(jsFiles, cssFiles...)

	for _, fileName := range everythingList {
		var data []byte
		if strings.Contains(fileName, ".css") {
			data, _ = os.ReadFile("./static/css/" + fileName)
		} else if strings.Contains(fileName, ".js") {
			data, _ = os.ReadFile("./static/js/" + fileName)
		}
		staticFiles[fileName] = data
	}

	favicon, _ := os.ReadFile("./static/favicon.ico")
	staticFiles["favicon.ico"] = favicon

	homeButton, _ := os.ReadFile("./static/home.png")
	staticFiles["home.png"] = homeButton

	return staticFiles
}

// This middleware looks at the file being requested regardless of the route
// If the file is one of the available static js or css files, it sends it to the client
func StaticHandler(staticFiles map[string][]byte) gin.HandlerFunc {
	return func(c *gin.Context) {

		url := fmt.Sprint(c.Request.URL)
		urlArr := strings.Split(url, "/")
		file := urlArr[len(urlArr)-1]

		// Allow browser to cache for up to one hour
		c.Header("Cache-Control", "max-age=3600")

		if strings.Contains(file, ".") { // Check to see if the url potentially has a file
			data, exists := staticFiles[file] // See if the file is cached, if so get its bytes from the map
			if exists {
				if strings.Contains(file, ".css") {
					c.Data(http.StatusOK, "text/css", data)
				} else if strings.Contains(file, ".js") {
					c.Data(http.StatusOK, "text/plain", data)
				} else if file == "favicon.ico" {
					c.Data(http.StatusOK, "image/x-icon", data)
				}
				c.Abort()
			}

		}

		// If all else fails, move to the next middleware/handler
		c.Next()
	}

}

func deletePlaceHolder(svc *dynamodb.DynamoDB, username string, tableName string) error {

	// Define the key to identify the item you want to update
	key := map[string]*dynamodb.AttributeValue{
		"username": {
			S: aws.String(username),
		},
	}

	// Define the update expression to set the new property value
	updateExpression := "REMOVE shoots.placeholder"

	// Configure the update input
	updateInput := &dynamodb.UpdateItemInput{
		TableName:        aws.String(tableName),
		Key:              key,
		UpdateExpression: aws.String(updateExpression),
		ReturnValues:     aws.String("UPDATED_NEW"), // If you want to return the updated item
	}

	// Perform the update operation
	_, err := svc.UpdateItem(updateInput)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	port := env("PORT")           // Port to listen on
	region := env("REGION")       // AWS region to be used
	bucket := env("BUCKET")       // S3 bucket to be referenced
	tableName := env("TABLENAME") // DynamoDB table to use
	protocol := strings.ToLower(env("PROTOCOL"))
	debug := strings.ToLower(env("DEBUG"))
	scyllaUrl := env("SCYLLA_URL")
	var minutes int64
	minutes, _ = strconv.ParseInt(env("MINUTES"), 10, 64) // Number of minutes the pre-signed urls will be good for
	staticFiles := cacheStaticFiles()
	maxPics, _ := strconv.Atoi(env("MAXPICS"))

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
		log.Fatalf("Error creating S3 Client: %v", err)
	}
	client := s3.New(s3sess)

	// If the Scylla url is not used, use AWS
	// Otherwise connect to Scylla
	var svc *dynamodb.DynamoDB
	if scyllaUrl == "" {

		// Initialize a session that the SDK will use to load
		// credentials from the shared credentials file ~/.aws/credentials
		// and region from the shared configuration file ~/.aws/config.
		dynamoSess := session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
		// Create DynamoDB client session
		svc = dynamodb.New(dynamoSess)
		go autoRenewDynamoCredentials(&svc) // Renew client session every 4 minutes to prevent token expiry

	} else {
		scyllaCredentials := credentials.NewStaticCredentials("cassandra", "cassandra", "None") //Auth not yet actually working
		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String("None"),
			Endpoint:    aws.String(scyllaUrl),
			Credentials: scyllaCredentials,
		})
		if err != nil {
			log.Println(err)
		}
		svc = dynamodb.New(sess)
	}

	//deleteInput := &dynamodb.DeleteTableInput{TableName: &tableName}
	//svc.DeleteTable(deleteInput)

	createInput := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("username"),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("username"),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
		TableName: aws.String(tableName),
	}

	_, err = svc.CreateTable(createInput)
	if err == nil {
		fmt.Printf("Created the DB table: %v\n", tableName)
	} else if !(strings.Contains(err.Error(), "ResourceInUseException: Table")) {
		log.Fatalf("Could not create DB: %v", err)
	}

	// Create the Redis client
	redClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Test the Redis client connection
	// Exit program if Redis is unavailable
	err = redClient.Ping().Err()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	// Initialize Gin
	gin.SetMode(gin.ReleaseMode)      // Turn off debugging mode
	r := gin.Default()                // Initialize Gin
	r.Use(StaticHandler(staticFiles)) // Cache and serve static files
	if debug == "true" {
		r.Use(nocache.NoCache()) // Sets gin to disable browser caching
	}

	//Route for health check
	r.GET("/ping", func(c *gin.Context) {

		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})

	})

	// Route to request either login or home page for the user
	r.GET("/", func(c *gin.Context) {

		auth, _ := checkToken(c, redClient)

		if !auth {
			c.Redirect(302, "/login")
			return
		} else {
			c.Redirect(302, "/home")
		}

	})

	r.GET("/home", func(c *gin.Context) {

		auth, userName := checkToken(c, redClient)
		if !auth {
			c.Redirect(302, "/login")
			return
		}

		shoots, err := getShoots(tableName, userName, svc)
		if err != nil {
			abortWithError(http.StatusInternalServerError, err, c)
			return
		}

		var shootsMap map[string]Shoot
		_ = json.Unmarshal([]byte(shoots), &shootsMap)

		tiles, err := generateTiles(shootsMap, bucket, client)
		if err != nil {
			abortWithError(http.StatusInternalServerError, err, c)
			return
		}

		tmpl, err := template.ParseFiles("./static/html/home.html")
		if err != nil {
			log.Printf("Could not parse home.html")
			c.Data(http.StatusInternalServerError, "text/plain", []byte("Could not parse template"))
			return
		}

		var final bytes.Buffer
		err = tmpl.Execute(&final, tiles)
		if err != nil {
			log.Printf("Could not execute html template: %v", err)
			c.Data(http.StatusInternalServerError, "text/plain", []byte("Could not parse template"))
			return
		}

		c.Data(http.StatusOK, "text/html", []byte(final.String()))

	})

	r.GET("/login", func(c *gin.Context) {

		auth, _ := checkToken(c, redClient)

		if auth {
			c.Redirect(302, "/home")
			return
		}

		html, _ := os.ReadFile("./static/html/login.html")
		c.Data(http.StatusOK, "text/html", html)
	})

	r.GET("/shoot/:shoot/:page", func(c *gin.Context) {

		auth, username := checkToken(c, redClient)
		if !auth {
			c.Redirect(302, "/login")
			return
		}

		page, _ := strconv.Atoi(c.Param("page"))
		shoot := c.Param("shoot")

		data, err := getUser(tableName, username, svc)
		if err != nil {
			log.Printf("could not get shoot data: %v", err)
			abortWithError(http.StatusNotFound, err, c)
		}

		prefix := data.Shoots[shoot].Prefix
		if prefix == "" {
			log.Printf("shoot did not exist")
			c.Data(http.StatusNotFound, "text/plain", []byte("Shoot not found"))
			return
		}

		objects, err := getObjects(client, bucket, prefix, page, maxPics) // Get the prefix objects
		if err != nil {
			log.Print(err.Error())
			abortWithError(http.StatusBadRequest, err, c)
		}
		urls, err := createUrls(client, bucket, objects, minutes) // Generate the pre-signed urls
		if err != nil {
			log.Print(err.Error())
			abortWithError(http.StatusBadRequest, err, c)
		}
		html, err := createHTML(urls) // Generate the HTML
		if err != nil {
			log.Print(err.Error())
			abortWithError(http.StatusBadRequest, err, c)
		}
		c.Data(http.StatusOK, "text/html; charset-utf-8", []byte(html)) // Send the HTML to the client
	})

	r.GET("/shoot/:shoot", func(c *gin.Context) {
		shoot := c.Param("shoot")
		c.Redirect(http.StatusFound, "/shoot/"+shoot+"/0")
	})

	r.GET("/shoot/:shoot/getPicks", func(c *gin.Context) {

		shootName := c.Param("shoot")

		auth, username := checkToken(c, redClient)
		if !auth {
			c.Redirect(302, "/login")
			return
		}

		// Create an Item struct to hold the retrieved data
		var picks Picks

		key := map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(username),
			},
		}

		// Define which attribute(s) you want to retrieve
		projectionExpression := fmt.Sprintf("shoots.%v.picks", shootName)

		// Create a GetItemInput object
		input := &dynamodb.GetItemInput{
			TableName:            aws.String(tableName),
			Key:                  key,
			ProjectionExpression: aws.String(projectionExpression),
		}

		// Perform the GetItem operation
		result, err := svc.GetItem(input)
		if err != nil {
			fmt.Println("Error getting item:", err)
			return
		}

		// Check if the item was found
		if result.Item != nil {
			// Unmarshal the DynamoDB item into the Item struct
			err := dynamodbattribute.UnmarshalMap(result.Item["shoots"].M[shootName].M["picks"].M, &picks)
			if err != nil {
				fmt.Println("Error unmarshalling item:", err)
				return
			}
		} else {
			fmt.Println("Item not found")
		}

		picksJSON, _ := json.Marshal(picks)

		c.Data(http.StatusOK, "application/json", picksJSON)
	})

	r.GET("/shoot/:shoot/updatePicksCookie", func(c *gin.Context) {

		shootName := c.Param("shoot")

		auth, username := checkToken(c, redClient)
		if !auth {
			c.Redirect(302, "/login")
			return
		}

		// Create an Item struct to hold the retrieved data
		var picks Picks

		key := map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(username),
			},
		}

		// Define which attribute(s) you want to retrieve
		projectionExpression := fmt.Sprintf("shoots.%v.picks", shootName)

		// Create a GetItemInput object
		input := &dynamodb.GetItemInput{
			TableName:            aws.String(tableName),
			Key:                  key,
			ProjectionExpression: aws.String(projectionExpression),
		}

		// Perform the GetItem operation
		result, err := svc.GetItem(input)
		if err != nil {
			fmt.Println("Error getting item:", err)
			return
		}

		// Check if the item was found
		if result.Item != nil {
			// Unmarshal the DynamoDB item into the Item struct
			err := dynamodbattribute.UnmarshalMap(result.Item["shoots"].M[shootName].M["picks"].M, &picks)
			if err != nil {
				fmt.Println("Error unmarshalling item:", err)
				return
			}
		} else {
			fmt.Println("Item not found")
		}

		picksJSON, _ := json.Marshal(picks)

		// Create a new cookie
		picksCookie := &http.Cookie{
			Name:     "picks",
			Value:    string(picksJSON),
			Secure:   true,
			HttpOnly: false,
		}

		weekInSeconds := 604800
		c.SetCookie(picksCookie.Name, picksCookie.Value, weekInSeconds, "/", c.Request.Host, picksCookie.Secure, picksCookie.HttpOnly)

		c.Data(http.StatusOK, "application/json", picksJSON)
	})

	r.POST("/shoot/add/:shootName", func(c *gin.Context) {

		auth, username := checkToken(c, redClient)
		if !auth {
			c.Redirect(302, "/login")
			return
		}

		shootName := c.Param("shootName")
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("could not read request body: %v", err)
			abortWithError(http.StatusBadRequest, err, c)
		}

		// Define the key to identify the item you want to update
		key := map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(username),
			},
		}

		// Define the update expression to set the new property value
		updateExpression := "SET shoots." + shootName + " = :newValue"

		var shoot Shoot
		_ = json.Unmarshal(body, &shoot)
		newShoot, err := dynamodbattribute.MarshalMap(shoot)
		if err != nil {
			log.Printf("Could not marshal json: %v", err)
			abortWithError(http.StatusBadRequest, err, c)
		}

		// Define expression attribute values
		expressionAttributeValues := map[string]*dynamodb.AttributeValue{
			":newValue": {
				M: newShoot,
			},
		}

		// Configure the update input
		updateInput := &dynamodb.UpdateItemInput{
			TableName:                 aws.String(tableName),
			Key:                       key,
			UpdateExpression:          aws.String(updateExpression),
			ExpressionAttributeValues: expressionAttributeValues,
			ReturnValues:              aws.String("UPDATED_NEW"), // If you want to return the updated item
		}

		// Perform the update operation
		_, err = svc.UpdateItem(updateInput)
		if err != nil {
			log.Printf("Could not add shoot: %v", err)
			abortWithError(http.StatusBadRequest, err, c)
		}
		err = deletePlaceHolder(svc, username, tableName)
		if err != nil {
			log.Println(err)
		}

	})

	// Called when the user sends their shoot picks in via the front end
	r.POST("/shoot/:shoot/:page/savePicks", func(c *gin.Context) {

		auth, username := checkToken(c, redClient)
		if !auth {
			c.Redirect(302, "/login")
			return
		}

		shoot := c.Param("shoot")

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("could not read request body: %v", err)
			abortWithError(http.StatusBadRequest, err, c)
		}

		user, err := getUser(tableName, username, svc)
		if err != nil {
			log.Printf("could not get user: %v : %v", username, err)
			abortWithError(http.StatusNotFound, err, c)
		}

		var picks Picks
		err = json.Unmarshal(body, &picks)
		if err != nil {
			fmt.Printf("could not unmarshal json: %v", err)
			abortWithError(http.StatusInternalServerError, err, c)
		}

		modifiedShoot := user.Shoots[shoot]
		modifiedShoot.Picks = picks
		user.Shoots[shoot] = modifiedShoot

		err = updatePicks(tableName, username, shoot, picks, svc)
		if err != nil {
			fmt.Printf("could not edit picks: %v", err)
			abortWithError(http.StatusInternalServerError, err, c)
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
		})

	})

	// Used when a user is creating an account
	r.GET("/signup", func(c *gin.Context) {
		html, _ := os.ReadFile("./static/html/signup.html")
		c.Data(http.StatusOK, "text/html", html)
	})

	// Returns the picks from a user's shoot
	r.GET("/getSelections/:shoot", func(c *gin.Context) {

		shoot := c.Param("shoot")
		shoot = strings.ToLower(shoot)

		auth, username := checkToken(c, redClient)

		if !auth {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		data, err := getUser(tableName, username, svc)
		if err != nil {
			log.Printf("could not get shoot data: %v", err)
			abortWithError(http.StatusNotFound, err, c)
			return
		}

		results, err := json.Marshal(data.Shoots[shoot].Picks)
		if err != nil {
			log.Printf("could not marshal shoot json data: %v", err)
			abortWithError(http.StatusNotFound, err, c)
			return
		}

		c.Data(http.StatusOK, "application/json", results)

	})

	// Get a user from the DB
	// Only works in debug mode
	if debug == "true" {
		r.GET("/user/:username", func(c *gin.Context) {
			username := c.Param("username")
			result, err := getUser(tableName, username, svc)
			if err != nil {
				abortWithError(404, err, c)
				return
			}
			c.JSON(http.StatusOK, result)
		})
	}

	// Creates a new user in the database
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
		_ = json.Unmarshal(body, &user)
		user.Salt, _ = generateSalt(32)
		user.Password = user.Password + user.Salt

		//Convert the password from the request body into a salted hash using bcrypt
		var hash []byte
		hash, err = bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			err = fmt.Errorf("could not hash the password: %v", err)
			abortWithError(http.StatusInternalServerError, err, c)
			return

		}
		user.Password = string(hash)

		// Create the user in DynamoDB
		err = createUser(tableName, user, svc)
		if err != nil {
			abortWithError(http.StatusInternalServerError, err, c)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
		})
	})

	// User sign in. Sends an auth token cookie to the front end
	// Also sends a json with the auth token
	r.POST("/signin", func(c *gin.Context) {

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

		var providedCredentials map[string]string

		_ = json.Unmarshal(body, &providedCredentials)
		providedCredentials["username"] = strings.ToLower(providedCredentials["username"])
		user, err := getUser(tableName, providedCredentials["username"], svc)
		if err != nil {
			log.Printf("There was a problem fetching a user from the DB: %v", err)
			abortWithError(http.StatusNotFound, err, c)
			return
		}

		authBool := verifyPassword(user.Password, providedCredentials["password"], user.Salt)

		if authBool {
			token, err := generateSessionToken()
			if err != nil {
				log.Printf("Could not generate token for %v: %v", providedCredentials["username"], err)
			}
			setToken(redClient, providedCredentials["username"], token)

			authJson := map[string]string{"username": user.Username, "token": token}
			authJsonBytes, err := json.Marshal(authJson)
			if err != nil {
				log.Printf("could not marshal json: %v", err)
			}

			// Create a new cookie
			authCookie := &http.Cookie{
				Name:     "authToken",
				Value:    string(authJsonBytes),
				HttpOnly: true,
			}

			weekInSeconds := 604800
			c.SetCookie(authCookie.Name, authCookie.Value, weekInSeconds, "/", c.Request.Host, true, true)

			c.JSON(http.StatusAccepted, gin.H{"accepted": authBool, "token": token})
		} else if !authBool {
			c.Data(http.StatusNotFound, "text/plain", []byte("incorrect username or password"))
		}

	})

	fmt.Printf("Listening for %v on port %v...\n", protocol, port) //Notifies that server is running on X port
	if protocol == "http" {                                        //Start running the Gin server
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

package main

type User struct {
	Username   string `json:"username"`
	First_name string `json:"first"`
	Last_name  string `json:"last"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	City       string `json:"city"`
	State      string `json:"state"`
	Password   string `json:"password"`
	Salt       string `json:"salt"`
	Zip        string `json:"zip"`
}

type Thumbnail struct {
	Key string
	Url string
}

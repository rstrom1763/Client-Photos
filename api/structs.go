package main

type User struct {
	Username   string           `json:"username"`
	First_name string           `json:"first"`
	Last_name  string           `json:"last"`
	Email      string           `json:"email"`
	Phone      string           `json:"phone"`
	Address    string           `json:"address"`
	City       string           `json:"city"`
	State      string           `json:"state"`
	Password   string           `json:"password"`
	Salt       string           `json:"salt"`
	Shoots     map[string]Shoot `json:"shoots"`
	Zip        string           `json:"zip"`
}

type Thumbnail struct {
	Key string
	Url string
}

type Shoot struct {
	Picks     Picks  `json:"picks"`
	Prefix    string `json:"prefix"`
	Date      string `json:"date"`
	Thumbnail string `json:"thumbnail"`
}

type Picks struct {
	Count int      `json:"count"`
	Picks []string `json:"picks"`
}

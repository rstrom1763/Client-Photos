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
	Files     []string `json:"files"`
	Picks     Picks    `json:"picks"`
	Prefix    string   `json:"prefix"`
	Date      string   `json:"date"`
	Thumbnail string   `json:"thumbnail"`
}

type Picks struct {
	Count int      `json:"count"`
	Picks []string `json:"picks"`
}

type RequestLog struct {
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	RemoteIP  string `json:"remote_ip"`
	Body      string `json:"body"`
	Status    int    `json:"status"`
}

type HomePageTile struct {
	Name      string
	Thumbnail string
}

type Session struct {
	Username  string `json:"username"`
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

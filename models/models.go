package models

// Will be stored at mongodb
type ChatMessage struct {
	Time    string
	Name    string
	Message string
}

// Will be stored at Redis
type User struct {
	Login string
	Fname string
	Lname string
	Pass  []byte
	Role  string
}

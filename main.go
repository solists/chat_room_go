package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

// TODO: remove race condition
var tpl *template.Template
var dbUsers map[string]user
var dbSessions map[string]session
var dbSessionsCleaned time.Time

const (
	sessionLength int = 30
)

var dbMessages []chatMessage

func init() {
	tpl = template.Must(template.ParseGlob("./views/*.gohtml"))
	dbUsers = make(map[string]user)
	dbSessions = make(map[string]session)
	dbMessages = make([]chatMessage, 0)
}

type chatMessage struct {
	Time  time.Time
	Name  string
	Value string
}

type user struct {
	login string
	fname string
	lname string
	pass  []byte
	role  string
}

// Implement userInt interface
func (u user) getFName() string {
	return u.fname
}
func (u user) getLName() string {
	return u.lname
}
func (u user) getLogin() string {
	return u.login
}
func (u user) getPass() []byte {
	return u.pass
}
func (u user) getRole() string {
	return u.role
}

type userInt interface {
	getFName() string
	getLName() string
	getLogin() string
	getPass() []byte
	getRole() string
}

type session struct {
	un         string
	lastActive time.Time
}

func (s session) getUN() string {
	return s.un
}
func (s *session) setLastActive(t time.Time) {
	s.lastActive = t
}

type sessionInt interface {
	getUN() string
	getLastActive() string
}

func main() {
	http.HandleFunc("/login", loginHandle)
	http.Handle("/views/", http.StripPrefix("/views/", http.FileServer(http.Dir("views"))))
	http.HandleFunc("/main", mainHandle)
	http.HandleFunc("/signup", signupHandle)

	http.Handle("/", http.RedirectHandler("/main", http.StatusSeeOther))
	http.Handle("/favicon.ico", http.NotFoundHandler())

	http.ListenAndServe(":8080", nil)
}

func signupHandle(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/main", http.StatusSeeOther)
	}
	var u user
	if r.Method == http.MethodPost {
		// get form values
		un := r.FormValue("username")
		p := r.FormValue("password")
		f := r.FormValue("firstname")
		l := r.FormValue("lastname")
		role := r.FormValue("role")
		// username taken?
		if _, ok := dbUsers[un]; ok {
			http.Error(w, "Username already taken", http.StatusForbidden)
			return
		}
		// create session
		sID := uuid.NewV4()
		c := &http.Cookie{
			Name:  "session",
			Value: sID.String(),
		}
		c.MaxAge = sessionLength
		http.SetCookie(w, c)
		dbSessions[c.Value] = session{un, time.Now()}
		// store user in dbUsers
		bs, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.MinCost)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		u = user{un, f, l, bs, role}
		dbUsers[un] = u
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tpl.ExecuteTemplate(w, "signup.gohtml", u)
}

func loginHandle(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/main", http.StatusSeeOther)
	}

	if r.Method == http.MethodPost {
		un := r.FormValue("username")
		pswrd := r.FormValue("password")
		// is there a username?
		u, ok := dbUsers[un]
		if !ok {
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		// does the entered password match the stored password?
		err := bcrypt.CompareHashAndPassword(u.pass, []byte(pswrd))
		if err != nil {
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		// create session
		sID := uuid.NewV4()
		c := &http.Cookie{
			Name:  "session",
			Value: sID.String(),
		}
		c.MaxAge = sessionLength
		http.SetCookie(w, c)
		dbSessions[c.Value] = session{un, time.Now()}
		http.Redirect(w, r, "/main", http.StatusSeeOther)
		return
	}
	err := tpl.ExecuteTemplate(w, "login.gohtml", "Sam")
	if err != nil {
		log.Fatalln(err)
	}

}

func mainHandle(w http.ResponseWriter, r *http.Request) {
	u, err := getUser(w, r)
	if err != nil {
		log.Fatalln(err)
	}
	// User not found
	if u.login == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	if r.Method == http.MethodGet {
		lout := r.FormValue("logout")
		if lout == "true" {
			if !alreadyLoggedIn(w, r) {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			c, _ := r.Cookie("session")
			// delete the session
			delete(dbSessions, c.Value)
			// remove the cookie
			c = &http.Cookie{
				Name:   "session",
				Value:  "",
				MaxAge: -1,
			}
			http.SetCookie(w, c)

			// clean up dbSessions
			if time.Now().Sub(dbSessionsCleaned) > (time.Second * 30) {
				go cleanSessions()
			}

			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	} else if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			log.Fatalln(err)
		}
		sButton := r.PostForm.Get("submitmsg")
		sMess := r.PostForm.Get("usermsg")

		if sButton == "Send" && sMess != "" {
			m := chatMessage{Time: time.Now(), Name: u.login, Value: sMess}
			dbMessages = append(dbMessages, m)
			data := fmt.Sprintf("<div class='msgln'><span class='chat-time'>%v</span> <b class='user-name'>%s</b>%s<br></div>", m.Time, m.Name, m.Value)
			fmt.Println(data)
			f, err := os.OpenFile("./views/message.gohtml", os.O_RDWR|os.O_APPEND, 0660)
			if err != nil {
				http.Error(w, "Error during processing messages", http.StatusInternalServerError)
				log.Panicln(err)
			}
			defer f.Close()
			_, err = f.WriteString(data)
			if err != nil {
				http.Error(w, "Error during writing file", http.StatusInternalServerError)
				log.Panicln(err)
			}
		}
	}
	err = tpl.ExecuteTemplate(w, "index.gohtml", dbMessages)
	if err != nil {
		http.Error(w, "Error during processing template", http.StatusInternalServerError)
		log.Panicln(err)
	}
}

func getUser(w http.ResponseWriter, r *http.Request) (*user, error) {
	// get cookie
	c, err := r.Cookie("session")
	if err != nil {
		sID := uuid.NewV4()
		c = &http.Cookie{
			Name:  "session",
			Value: sID.String(),
		}
	}
	c.MaxAge = sessionLength
	http.SetCookie(w, c)

	// if the user exists already, get user
	var u user
	if s, ok := dbSessions[c.Value]; ok {
		s.lastActive = time.Now()
		dbSessions[c.Value] = s
		u = dbUsers[s.un]
	}
	return &u, nil
}

func alreadyLoggedIn(w http.ResponseWriter, req *http.Request) bool {
	c, err := req.Cookie("session")
	if err != nil {
		return false
	}
	s, ok := dbSessions[c.Value]
	if ok {
		s.lastActive = time.Now()
		dbSessions[c.Value] = s
	}
	_, ok = dbUsers[s.un]
	// refresh session
	c.MaxAge = sessionLength
	http.SetCookie(w, c)
	return ok
}

func cleanSessions() {
	fmt.Println("BEFORE CLEAN") // for demonstration purposes
	showSessions()              // for demonstration purposes
	for k, v := range dbSessions {
		if time.Now().Sub(v.lastActive) > (time.Second * 30) {
			delete(dbSessions, k)
		}
	}
	dbSessionsCleaned = time.Now()
	fmt.Println("AFTER CLEAN") // for demonstration purposes
	showSessions()             // for demonstration purposes
}

// for demonstration purposes
func showSessions() {
	fmt.Println("********")
	for k, v := range dbSessions {
		fmt.Println(k, v.un)
	}
	fmt.Println("")
}

func checkUserName(w http.ResponseWriter, req *http.Request) {
	sampleUsers := map[string]bool{
		"test@example.com": true,
		"jame@bond.com":    true,
		"moneyp@uk.gov":    true,
	}

	bs, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Println(err)
	}

	sbs := string(bs)
	fmt.Println("USERNAME: ", sbs)

	fmt.Fprint(w, sampleUsers[sbs])
}

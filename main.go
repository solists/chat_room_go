package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
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

	bs, _ := bcrypt.GenerateFromPassword([]byte("123"), bcrypt.MinCost)
	dbUsers["tyt@tyt"] = user{login: "tyt@tyt", pass: bs}
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

type session struct {
	un         string
	lastActive time.Time
}

func main() {
	// http.HandleFunc("/login", loginHandle)
	// http.Handle("/views/", http.StripPrefix("/views/", http.FileServer(http.Dir("views"))))
	// http.HandleFunc("/main", mainHandle)
	// http.HandleFunc("/signup", signupHandle)
	// http.HandleFunc("/messages", getMessagesHandle)

	// http.Handle("/", http.RedirectHandler("/main", http.StatusSeeOther))
	// http.Handle("/favicon.ico", http.NotFoundHandler())

	// http.ListenAndServe("localhost:8080", nil)

	url := "tt"
	logger, _ := zap.NewProduction()
	defer logger.Sync() // flushes buffer, if any
	sugar := logger.Sugar()
	sugar.Infow("failed to fetch URL",
		// Structured context as loosely typed key-value pairs.
		"url", url,
		"attempt", 3,
		"backoff", time.Second,
	)
	sugar.Infof("Failed to fetch URL: %s", url)
}

func signupHandle(w http.ResponseWriter, r *http.Request) {
	c, cFound := getSessionCookie(w, r)
	if cFound && alreadyLoggedIn(w, c) {
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

func getMessagesHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET methods allowed", http.StatusMethodNotAllowed)
		return
	}
	c, cFound := getSessionCookie(w, r)
	if !cFound {
		http.Error(w, "Not logged in", http.StatusForbidden)
		return
	}
	_, uFound := getUser(c)
	if !uFound {
		http.Error(w, "Not logged in", http.StatusForbidden)
		return
	}
	numChatMessages := 5
	numOfAllMess := len(dbMessages)
	var lastMessages = make([]chatMessage, 5)
	if numOfAllMess < numChatMessages {
		lastMessages = dbMessages
	} else {
		lastMessages = dbMessages[numOfAllMess-numChatMessages : numOfAllMess]
	}

	outputJSON, err := json.Marshal(lastMessages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(outputJSON)
}

func loginHandle(w http.ResponseWriter, r *http.Request) {
	c, cFound := getSessionCookie(w, r)
	if cFound && alreadyLoggedIn(w, c) {
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
	c, cFound := getSessionCookie(w, r)
	if !cFound {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	u, userFound := getUser(c)
	if userFound {
		updateSession(w, c)
	}

	if r.Method == http.MethodGet {
		if !userFound {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		lout := r.FormValue("logout")
		if lout == "true" {
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
			if time.Since(dbSessionsCleaned) > (time.Second * 30) {
				go cleanSessions()
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	} else if r.Method == http.MethodPost {
		if !userFound {
			w.Header().Add("redirect", "/login")
			w.WriteHeader(http.StatusOK)
			return
		}
		err := r.ParseForm()
		if err != nil {
			log.Fatalln(err)
		}

		sMess := r.PostForm.Get("usermsg")

		if sMess != "" {
			m := chatMessage{Time: time.Now(), Name: u.login, Value: sMess}
			dbMessages = append(dbMessages, m)
		}
	}
	if !userFound {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	err := tpl.ExecuteTemplate(w, "index.gohtml", nil) //dbMessages)
	if err != nil {
		http.Error(w, "Error during processing template", http.StatusInternalServerError)
		log.Panicln(err)
	}
}

func getUser(c *http.Cookie) (user, bool) {
	// if the user exists already, get user
	var u user
	if s, ok := dbSessions[c.Value]; ok {
		u = dbUsers[s.un]

		return u, true
	}
	return u, false
}

// Gets cookie "session" or adds new
func getSessionCookie(w http.ResponseWriter, r *http.Request) (*http.Cookie, bool) {
	c, err := r.Cookie("session")
	found := true
	if err != nil {
		sID := uuid.NewV4()
		c = &http.Cookie{
			Name:  "session",
			Value: sID.String(),
		}
		found = false
	}
	c.MaxAge = sessionLength
	http.SetCookie(w, c)

	return c, found
}

func updateSession(w http.ResponseWriter, c *http.Cookie) {
	s, ok := dbSessions[c.Value]
	if ok {
		s.lastActive = time.Now()
		dbSessions[c.Value] = s
	}
	// refresh session
	c.MaxAge = sessionLength
	http.SetCookie(w, c)
}

func alreadyLoggedIn(w http.ResponseWriter, c *http.Cookie) bool {
	_, ok := getUser(c)

	return ok
}

func cleanSessions() {
	fmt.Println("BEFORE CLEAN") // for demonstration purposes
	showSessions()              // for demonstration purposes
	for k, v := range dbSessions {
		if time.Since(v.lastActive) > (time.Second * 30) {
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

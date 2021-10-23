package main

import (
	"chat_room_go/models"
	"chat_room_go/utils/logs"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	mongoconnector "chat_room_go/microservices/mongodb/pb"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

var tpl *template.Template

// TODO: remove probable race condition
var dbSessions map[string]session
var dbSessionsCleaned time.Time

const (
	sessionLength int = 30
)

func init() {
	tpl = template.Must(template.ParseGlob("./views/*.gohtml"))
	dbSessions = make(map[string]session)
}

type session struct {
	un         string
	lastActive time.Time
}

func main() {
	defer Cleanup()
	http.HandleFunc("/login", loginHandle)
	http.Handle("/views/", http.StripPrefix("/views/", http.FileServer(http.Dir("views"))))
	http.HandleFunc("/main", mainHandle)
	http.HandleFunc("/signup", signupHandle)
	http.HandleFunc("/messages", getMessagesHandle)

	http.Handle("/", http.RedirectHandler("/main", http.StatusSeeOther))
	http.Handle("/favicon.ico", http.NotFoundHandler())

	http.ListenAndServe("localhost:8080", nil)

	logs.Logger.Infof("Started server")
	logs.Logger.Sync()

	bs, _ := bcrypt.GenerateFromPassword([]byte("123"), bcrypt.MinCost)

	_, err := RedisAdapter.Write(models.User{Login: "tyt@tyt", Pass: bs})
	if err != nil {
		logs.Logger.Panic(err)
	}
}

// Closes grpc connections
func Cleanup() {
	defer logs.WL.GrpcConn.Close()
	defer MongoAdapter.grpcConn.Close()
	defer RedisAdapter.grpcConn.Close()
}

// Handles signup page TODO: rework front
func signupHandle(w http.ResponseWriter, r *http.Request) {
	logs.Logger.Info(r)
	c, cFound := getSessionCookie(w, r)
	if cFound && alreadyLoggedIn(w, c) {
		http.Redirect(w, r, "/main", http.StatusSeeOther)
	}
	var u models.User
	if r.Method == http.MethodPost {
		// get form values
		un := r.FormValue("username")
		p := r.FormValue("password")
		f := r.FormValue("firstname")
		l := r.FormValue("lastname")
		role := r.FormValue("role")
		// username taken?
		res, err := RedisAdapter.Read(un)
		if err != nil {
			logs.Logger.Panic(err)
		}
		if res != nil {
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
		u = models.User{
			Login: un,
			Fname: f,
			Lname: l,
			Pass:  bs,
			Role:  role}
		_, err = RedisAdapter.Write(u)
		if err != nil {
			logs.Logger.Panic(err)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tpl.ExecuteTemplate(w, "signup.gohtml", u)
}

// Returns messages to front
func getMessagesHandle(w http.ResponseWriter, r *http.Request) {
	logs.Logger.Info(r)
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
	dbMessages, err := MongoAdapter.Read()
	if err != nil {
		logs.Logger.Info(err)
	}
	numOfAllMess := len(dbMessages)
	var lastMessages = make([]*mongoconnector.MessageInfo, 0, 5)
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

// Handles login page
func loginHandle(w http.ResponseWriter, r *http.Request) {
	logs.Logger.Info(r)
	c, cFound := getSessionCookie(w, r)
	if cFound && alreadyLoggedIn(w, c) {
		http.Redirect(w, r, "/main", http.StatusSeeOther)
	}

	if r.Method == http.MethodPost {
		un := r.FormValue("username")
		pswrd := r.FormValue("password")
		// is there a username?
		u, err := RedisAdapter.Read(un)
		if err != nil {
			logs.Logger.Panic(err)
		}
		if u == nil {
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		// does the entered password match the stored password?
		err = bcrypt.CompareHashAndPassword([]byte(u.Pass), []byte(pswrd))
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
		logs.Logger.Panic(err)
	}

}

// Handles main page
func mainHandle(w http.ResponseWriter, r *http.Request) {
	logs.Logger.Info(r)
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
			logs.Logger.Panic(err)
		}

		sMess := r.PostForm.Get("usermsg")

		if sMess != "" {
			m := models.ChatMessage{Time: time.Now().Format("2006-01-02 15:04:05"), Name: u.Login, Message: sMess}
			_, err := MongoAdapter.Write(m.Message, m.Name, m.Time)
			if err != nil {
				logs.Logger.Info(err)
			}
		}
	}
	if !userFound {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	err := tpl.ExecuteTemplate(w, "index.gohtml", nil)
	if err != nil {
		http.Error(w, "Error during processing template", http.StatusInternalServerError)
		logs.Logger.Panic(err)
	}
}

// Gets user from cookie
func getUser(c *http.Cookie) (models.User, bool) {
	// if the user exists already, get user
	var u models.User
	if s, ok := dbSessions[c.Value]; ok {
		res, err := RedisAdapter.Read(s.un)
		if err != nil {
			logs.Logger.Panic(err)
		}
		u = *res

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

// Updates cookie
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

// Removes expired cookies, TODO: move session to redis
func cleanSessions() {
	for k, v := range dbSessions {
		if time.Since(v.lastActive) > (time.Second * 30) {
			delete(dbSessions, k)
		}
	}
	dbSessionsCleaned = time.Now()
}

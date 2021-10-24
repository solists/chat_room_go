// Main function, http listen and serve here

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

const (
	sessionLength int32 = 30
)

func init() {
	tpl = template.Must(template.ParseGlob("./views/*.gohtml"))
}

func main() {
	defer Cleanup()
	// Mux for logs and panic recovery
	techMux := http.NewServeMux()
	techHandler := panicMiddleware(accessLogMiddleware(techMux))

	// Mux for authentification required
	authMux := http.NewServeMux()
	authMux.HandleFunc("/main", mainHandle)
	authMux.HandleFunc("/messages", getMessagesHandle)
	siteHandler := authMiddleware(authMux)

	techMux.HandleFunc("/login", loginHandle)
	techMux.Handle("/views/", http.StripPrefix("/views/", http.FileServer(http.Dir("views"))))
	techMux.Handle("/main", siteHandler)
	techMux.HandleFunc("/signup", signupHandle)
	techMux.Handle("/messages", siteHandler)
	techMux.Handle("/", http.RedirectHandler("/main", http.StatusSeeOther))
	techMux.Handle("/favicon.ico", http.NotFoundHandler())

	http.ListenAndServe("localhost:8080", techHandler)

	logs.Logger.Infof("Started server")
	logs.Logger.Sync()
}

// Recover from panic logic here
func panicMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logs.Logger.Errorf("Recovered from panic %s", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Prints logs, response and processing time
func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logs.Logger.Info(r)
		start := time.Now()
		next.ServeHTTP(w, r)
		logs.Logger.Infof("[%s] Time elapsed: %s\n", r, time.Since(start))
	})
}

// Checks authentification via cookie
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isLoggedIn := isLoggedIn(r)
		if !isLoggedIn {
			destroySessionCookie(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// TODO: REMOVE
		updateSession(w, r)
		//http.Redirect(w, r, "/main", http.StatusSeeOther)
		next.ServeHTTP(w, r)
	})
}

// Closes grpc connections
func Cleanup() {
	defer logs.WL.GrpcConn.Close()
	defer MongoAdapter.grpcConn.Close()
	defer RedisAdapter.grpcConn.Close()
}

// Handles signup page TODO: rework front
func signupHandle(w http.ResponseWriter, r *http.Request) {
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
		err = setSessionCookie(w, r, un)
		if err != nil {
			logs.Logger.Panic("Error during session creation", err)
		}
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
		err = setSessionCookie(w, r, un)
		if err != nil {
			logs.Logger.Panic("Error during session creation", err)
		}
		http.Redirect(w, r, "/main", http.StatusSeeOther)
		return
	} else if r.Method == http.MethodGet {
		// If we are logged out at /messages, we asquire this, to redirect properly
		w.Header().Add("redirect", "/login")
		w.WriteHeader(http.StatusOK)
	}
	err := tpl.ExecuteTemplate(w, "login.gohtml", "Sam")
	if err != nil {
		logs.Logger.Panic(err)
	}
}

// Handles main page
func mainHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		lout := r.FormValue("logout")
		if lout == "true" {
			// delete the session
			_, isFound := getSessionCookie(w, r)
			if !isFound {
				// We should not get here without session (middleware should handle), then panic (middleware will handle)
				logs.Logger.Panic("Session not found")
			}
			destroySessionCookie(w, r)
			updateSession(w, r)

			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	} else if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			logs.Logger.Panic(err)
		}

		sMess := r.PostForm.Get("usermsg")
		u, isFound := getUser(w, r)
		if !isFound {
			logs.Logger.Panic("User not found")
		}
		if sMess != "" {
			m := models.ChatMessage{Time: time.Now().Format("2006-01-02 15:04:05"), Name: u.Login, Message: sMess}
			_, err := MongoAdapter.Write(m.Message, m.Name, m.Time)
			if err != nil {
				logs.Logger.Info(err)
			}
		}
	}
	err := tpl.ExecuteTemplate(w, "index.gohtml", nil)
	if err != nil {
		http.Error(w, "Error during processing template", http.StatusInternalServerError)
		logs.Logger.Panic(err)
	}
}

// Gets user from cookie
func getUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	// if the user exists already, get user
	un, isFound := getUsernameFromSession(w, r)
	if !isFound {
		return nil, false
	}
	res, err := RedisAdapter.Read(string(*un))
	if err != nil {
		logs.Logger.Panic(err)
	}

	return res, true
}

// Gets cookie "session" as a *string
func getUsernameFromSession(w http.ResponseWriter, r *http.Request) (*string, bool) {
	c, err := r.Cookie("session")
	if err == http.ErrNoCookie {
		return nil, false
	} else if err != nil {
		logs.Logger.Panic("Error while acquiring cookie")
	}

	record, err := RedisAdapter.GetSession(c.Value)
	if err != nil {
		logs.Logger.Panic("Error while retrieving value from cache, getUsernameFromSession: ", err)
	}
	if record == "" {
		return nil, false
	}

	return &record, true
}

// Gets cookie "session" as a *string
func getSessionCookie(w http.ResponseWriter, r *http.Request) (*string, bool) {
	c, err := r.Cookie("session")
	if err == http.ErrNoCookie {
		return nil, false
	} else if err != nil {
		logs.Logger.Panic("Error while acquiring cookie")
	}

	record, err := RedisAdapter.GetSession(c.Value)
	if err != nil {
		logs.Logger.Error("Error while retrieving value from cache, getSessionCookie: ", err)
	}
	if record == "" {
		return nil, false
	}

	return &c.Value, true
}

// Check if user logged in
func isLoggedIn(r *http.Request) bool {
	c, err := r.Cookie("session")
	if err == http.ErrNoCookie {
		return false
	} else if err != nil {
		logs.Logger.Panic("Error while acquiring cookie")
	}
	record, err := RedisAdapter.GetSession(c.Value)
	if err != nil {
		logs.Logger.Error("Error while retrieving value from cache, isLoggedIn: ", err)
	}
	if record == "" {
		return false
	}

	return true
}

// Check if user logged in
func setSessionCookie(w http.ResponseWriter, r *http.Request, login string) error {
	sID := uuid.NewV4()
	c := &http.Cookie{
		Name:   "session",
		Value:  sID.String(),
		MaxAge: int(sessionLength),
	}
	http.SetCookie(w, c)
	RedisAdapter.AddSession(sID.String(), login)

	return nil
}

// Updates cookie, and memcache record lifetime, if session is not in cache, then delete it
func updateSession(w http.ResponseWriter, r *http.Request) error {
	c, err := r.Cookie("session")
	if err != nil {
		return err
	}
	_, isFound := getSessionCookie(w, r)
	if !isFound {
		c = &http.Cookie{
			Name:   "session",
			Value:  "",
			MaxAge: -1,
		}
		http.SetCookie(w, c)
		return err
	} else if err != nil {
		return err
	}
	c.MaxAge = int(sessionLength)
	http.SetCookie(w, c)

	return nil
}

// TODO: new grpc method
func destroySessionCookie(w http.ResponseWriter, r *http.Request) error {
	c := &http.Cookie{
		Name:   "session",
		Value:  "",
		MaxAge: -1,
	}
	http.SetCookie(w, c)

	return nil
}

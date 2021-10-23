package main

import (
	"chat_room_go/models"
	"chat_room_go/utils/logs"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	grpcconnector "chat_room_go/microservices/mongodb/pb"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// TODO: remove race condition
var tpl *template.Template
var dbUsers map[string]models.User
var dbSessions map[string]session
var dbSessionsCleaned time.Time

const (
	sessionLength int = 30
)

//var dbMessages []models.ChatMessage
var wm mongoDBAdapter

func init() {
	tpl = template.Must(template.ParseGlob("./views/*.gohtml"))
	dbUsers = make(map[string]models.User)
	dbSessions = make(map[string]session)
	//dbMessages = make([]models.ChatMessage, 0)

	bs, _ := bcrypt.GenerateFromPassword([]byte("123"), bcrypt.MinCost)
	dbUsers["tyt@tyt"] = models.User{Login: "tyt@tyt", Pass: bs}

	wm = mongoDBAdapter{}
	wm.DbParms = MongoDBParms{DbName: "test", CollectionName: "messages"}
	wm.InitMongoAdapter()
}

type session struct {
	un         string
	lastActive time.Time
}

const (
	address     = "localhost:50051"
	defaultName = "world"
)

func main() {
	http.HandleFunc("/login", loginHandle)
	http.Handle("/views/", http.StripPrefix("/views/", http.FileServer(http.Dir("views"))))
	http.HandleFunc("/main", mainHandle)
	http.HandleFunc("/signup", signupHandle)
	http.HandleFunc("/messages", getMessagesHandle)

	http.Handle("/", http.RedirectHandler("/main", http.StatusSeeOther))
	http.Handle("/favicon.ico", http.NotFoundHandler())

	http.ListenAndServe("localhost:8080", nil)

	wl := logs.WriterToClickHouse{}
	wl.InitClickHouseLogger()
	wl.DbParms = logs.ClickHouseDBParms{DbName: "logs", TableName: "main"}
	defer wl.GrpcConn.Close()
	slc := wl.GetCLickHouseLogger()
	slc.Infof("cjcjc")
	slc.Sync()

	defer wm.GrpcConn.Close()
}

// Struct, that implements io.Writer, keeps all data for grpc request TODO: GrpcConn closel ogic move inside
type mongoDBAdapter struct {
	writerClient grpcconnector.WriterClient
	readerClient grpcconnector.ReaderClient
	ctx          context.Context
	GrpcConn     *grpc.ClientConn
	DbParms      MongoDBParms
}

type MongoDBParms struct {
	DbName         string
	CollectionName string
}

func (w *mongoDBAdapter) Write(message, name, time string) (int, error) {
	_, err := w.writerClient.Write(
		w.ctx,
		&grpcconnector.WriteRequest{Message: message, Name: name, Time: time},
	)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

func (w *mongoDBAdapter) Read() ([]*grpcconnector.MessageInfo, error) {
	toReturn, err := w.readerClient.Read(
		w.ctx,
		&grpcconnector.ReadRequest{Time: time.Now().Format("2006-01-02 15:04:05"), Number: 10},
	)
	if err != nil {
		return nil, err
	}
	return toReturn.Results, nil
}

// Initializes TLS, grpc mappings, context for logwriter
func (w *mongoDBAdapter) InitMongoAdapter() {
	creds, err := loadTLSCredentials()
	if err != nil {
		log.Panicln(err)
	}

	w.GrpcConn, err = grpc.Dial(
		"127.0.0.1:8082",
		grpc.WithPerRPCCredentials(&tokenAuth{"sometoken"}),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		log.Fatalf("cant connect to grpc")
	}

	w.writerClient = grpcconnector.NewWriterClient(w.GrpcConn)
	w.readerClient = grpcconnector.NewReaderClient(w.GrpcConn)

	w.ctx = context.Background()
	md := metadata.Pairs(
		"api-req-id", "123qwe",
		"dbname", w.DbParms.DbName,
		"collectionname", w.DbParms.CollectionName,
	)
	sHeader := metadata.Pairs("authorization", "val")
	grpc.SendHeader(w.ctx, sHeader)
	w.ctx = metadata.NewOutgoingContext(w.ctx, md)
}

type tokenAuth struct {
	Token string
}

func (t *tokenAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": t.Token,
	}, nil
}

func (c *tokenAuth) RequireTransportSecurity() bool {
	return false
}

// Enables TLS and adds certificates for the client
func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile("microservices/mongodb/certs/ca-cert.pem")
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	// Load client's certificate and private key
	clientCert, err := tls.LoadX509KeyPair("microservices/mongodb/certs/client-cert.pem", "microservices/clickhouse/certs/client-key.pem")
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		// Self signed certificate, TODO: Let`s Encrypt
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{clientCert},
		RootCAs:            certPool,
	}

	return credentials.NewTLS(config), nil
}

func signupHandle(w http.ResponseWriter, r *http.Request) {
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
		u = models.User{un, f, l, bs, role}
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
	dbMessages, err := wm.Read()
	fmt.Println(dbMessages)
	if err != nil {
		fmt.Println(err)
	}
	numOfAllMess := len(dbMessages)
	var lastMessages = make([]*grpcconnector.MessageInfo, 0, 5)
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
		err := bcrypt.CompareHashAndPassword(u.Pass, []byte(pswrd))
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
			m := models.ChatMessage{Time: time.Now().Format("2006-01-02 15:04:05"), Name: u.Login, Message: sMess}
			_, err := wm.Write(m.Message, m.Name, m.Time)
			if err != nil {
				fmt.Println(err)
			}
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

func getUser(c *http.Cookie) (models.User, bool) {
	// if the user exists already, get user
	var u models.User
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

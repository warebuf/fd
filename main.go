package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"

	"github.com/gorilla/websocket"
)

func indexHandler(res http.ResponseWriter, req *http.Request) {
	log.Println("/")
	t := template.Must(template.ParseFiles(filepath.Join("static", "index.html")))
	t.Execute(res, nil)
}

func loginHandler(res http.ResponseWriter, req *http.Request) {
	fmt.Println("provider")

	session, err := store.Get(req, "session-name")
	fmt.Println(session)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); ok && auth {
		res.Write([]byte("we aren't saving your data rn, sorry"))
	} else {
		gothic.BeginAuthHandler(res, req)
	}
}

func callbackHandler(res http.ResponseWriter, req *http.Request) {

	fmt.Printf("%+v\n", req)

	user, err := gothic.CompleteUserAuth(res, req)
	if err != nil {
		fmt.Fprintln(res, err)
		return
	}

	fmt.Printf("%+v\n", user)

	t := template.Must(template.ParseFiles(filepath.Join("static", "user.html")))

	session, err := store.Get(req, "session-name")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["Email"] = user.Email
	session.Values["authenticated"] = true

	// add the changes to the cookie
	err = session.Save(req, res)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// getting the cookie we are going to send
	// then creating an user_info struct for that cookie
	all_cookies := res.Header().Values("Set-Cookie")
	for i := 0; i < len(all_cookies); i++ { // potential vulnerability here b/c of semicolon
		stringSlice := strings.Split(all_cookies[i], ";")
		for j := 0; j < len(stringSlice); j++ {
			if len(stringSlice[j]) >= 13 { // potential vulnerability here b/c we are identifying cookie by first 13 characters (ie could make a fake cookie named session-name=2)
				if stringSlice[j][0:13] == "session-name=" {
					cookie_to_user_info[stringSlice[j][13:]] = &user_info{
						uid:           uid_count,
						email:         user.Email,
						nickname:      "bob",
						chat_history:  make(map[int]chan *message),
						current_rooms: make(map[int]*room),
					}
					uid_count++
				}
			}
		}
	}

	t.Execute(res, user)
}

func successHandler(res http.ResponseWriter, req *http.Request) {

	c, _ := req.Cookie("session-name")
	fmt.Printf("%+v", c)

	// Check if user is already authenticated
	session, err := store.Get(req, "session-name")
	fmt.Println(session)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); ok && auth {
		res.Write([]byte("hi " + session.Values["Email"].(string)))
	} else {
		//fmt.Println("User is not authenticated, redirecting to home page")
		http.Redirect(res, req, "/", http.StatusSeeOther)
	}
}

func chatHandler(res http.ResponseWriter, req *http.Request) {
	c, _ := req.Cookie("session-name")
	fmt.Printf("%+v\n", c)

	// Check if user is already authenticated
	session, err := store.Get(req, "session-name")
	fmt.Printf("%+v\n", session)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); ok && auth {
		t := template.Must(template.ParseFiles(filepath.Join("static", "chat.html")))
		fmt.Printf("%+v\n", req.Host)
		fmt.Printf("%+v\n", session.Values["Email"].(string))
		data := map[string]string{"email": session.Values["Email"].(string), "host": req.Host}
		t.Execute(res, data)
	} else {
		//fmt.Println("User is not authenticated, redirecting to home page")
		http.Redirect(res, req, "/", http.StatusSeeOther)
	}
}

func logoutHandler(res http.ResponseWriter, req *http.Request) {
	log.Println(("/logout/google"))
	gothic.Logout(res, req)
	// TODO: have to get rid of the session here, maybe even screw with the client's cookie, but that's  not as important as removing the session
	res.Header().Set("Location", "/")
	res.WriteHeader(http.StatusTemporaryRedirect)
}

func roomHandler(res http.ResponseWriter, req *http.Request) {
	log.Println("room")

	// implement

	active_rooms[0].ServeHTTP(res, req)
}

func gameHandler(res http.ResponseWriter, req *http.Request) {
	c, _ := req.Cookie("session-name")
	fmt.Printf("%+v", c)

	// Check if user is already authenticated
	session, err := store.Get(req, "session-name")
	fmt.Println(session)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); ok && auth {
		t := template.Must(template.ParseFiles(filepath.Join("static", "game.html")))
		t.Execute(res, req)
	} else {
		fmt.Println("User is not authenticated, redirecting to home page")
		http.Redirect(res, req, "/", http.StatusSeeOther)
	}
}

// global variables
var store *sessions.CookieStore // used to create and decrypt cookies
var active_rooms []*room
var port string

var uid_count = 0                             // used to assign a user id
var cookie_to_user_info map[string]*user_info // TODO: this might be vulnerable
var rid_count = 0                             // room id

type user_info struct {
	uid      int
	email    string
	nickname string

	// chat related stuff
	socket        *websocket.Conn       // socket is the web socket for this client.
	chat_history  map[int]chan *message // send is a channel on which messages are sent.
	current_rooms map[int]*room         // rid to room
}

func main() {

	// init
	cookie_to_user_info = make(map[string]*user_info)

	// init
	//goth.UseProviders(google.New("676118167957-4bpa000p9bfaf5vu6halmen6nfjnuo1r.apps.googleusercontent.com", "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", "https://testfdfdfdfd-f824fe0fe06f.herokuapp.com/callback"))
	goth.UseProviders(google.New("676118167957-4bpa000p9bfaf5vu6halmen6nfjnuo1r.apps.googleusercontent.com", "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", "http://localhost:3000/callback"))
	secret_key := []byte("secret-password") // used to encrypt cookies, have to use a stronger password, but used a simple one for illustration purposes
	store = sessions.NewCookieStore(secret_key)
	store.Options.HttpOnly = true // just to make sure javascript doesn't access cookies
	gothic.Store = store          // not sure why we do this, but if we do, gothic will handle all cookie authentication
	//port = os.Getenv("PORT")
	port = "3000"

	// create a room for users to chat in
	r := newRoom()
	active_rooms = append(active_rooms, r)

	mux := http.NewServeMux() // create the default multiplexer
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("/logout/google", logoutHandler)
	mux.HandleFunc("/chat", chatHandler)
	mux.HandleFunc("/success", successHandler)
	mux.HandleFunc("/callback", callbackHandler)
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/room", roomHandler)
	mux.HandleFunc("/game", gameHandler)
	mux.HandleFunc("/", indexHandler)

	go r.run()

	// run the server
	log.Println("listening on localhost:" + port)
	//log.Fatal(http.ListenAndServe(":"+port, mux))
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

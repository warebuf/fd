package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

func indexHandler(res http.ResponseWriter, req *http.Request) {
	log.Println("/")
	t := template.Must(template.ParseFiles(filepath.Join("static", "index.html")))
	t.Execute(res, req)
}

func authHandler(res http.ResponseWriter, req *http.Request) {
	fmt.Println("provider")

	session, err := store.Get(req, "session-name")
	fmt.Println(session)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); ok && auth {
		res.Write([]byte("we would've sent your user info here, but we didn't save it sorry"))
	} else {
		gothic.BeginAuthHandler(res, req)
	}
}

func googleHandler(res http.ResponseWriter, req *http.Request) {

	user, err := gothic.CompleteUserAuth(res, req)
	if err != nil {
		fmt.Fprintln(res, err)
		return
	}

	t := template.Must(template.ParseFiles(filepath.Join("static", "user.html")))

	session, err := store.Get(req, "session-name")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["Email"] = user.Email
	session.Values["authenticated"] = true

	err = session.Save(req, res)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
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
		res.Write([]byte("hi"))
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
		t.Execute(res, req)
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
	listOfRooms[0].ServeHTTP(res, req)
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
var store *sessions.CookieStore
var listOfRooms []*room
var port string

func main() {

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
	listOfRooms = append(listOfRooms, r)

	mux := http.NewServeMux() // create the default multiplexer
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("/logout/google", logoutHandler)
	mux.HandleFunc("/chat", chatHandler)
	mux.HandleFunc("/success", successHandler)
	mux.HandleFunc("/callback", googleHandler)
	mux.HandleFunc("/login", authHandler)
	mux.HandleFunc("/room", roomHandler)
	mux.HandleFunc("/game", gameHandler)
	mux.HandleFunc("/", indexHandler)

	go r.run()

	// run the server
	log.Println("listening on localhost:" + port)
	//log.Fatal(http.ListenAndServe(":"+port, mux))
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

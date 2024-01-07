package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

func indexHandler(res http.ResponseWriter, req *http.Request) {
	log.Println("/")
	t := template.Must(template.ParseFiles(filepath.Join("serving-files", "index.html")))
	t.Execute(res, req)
}

// this code is imperfect, do not put too much emphasis in understanding it rn
// all you need to understand is gothic.BeginAuthHandler() redirects the user to Google to get authenticated
func authHandler(res http.ResponseWriter, req *http.Request) {
	fmt.Println("provider")

	// Check if user already has a valid session going,
	// if the session is invalid => send them a page that tells them their session is invalid (could instead sent them to the homepage OR Google auth)
	// if the session is valid => refer them to the userpage (not working rn)
	// if they have no session => refer them to google to get authenicated
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

	t := template.Must(template.ParseFiles(filepath.Join("serving-files", "user.html")))

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
		t := template.Must(template.ParseFiles(filepath.Join("serving-files", "chat.html")))
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
	res.Write([]byte("gamer"))
}

// global variables
var store *sessions.CookieStore
var listOfRooms []*room
var port string

func main() {

	// init
	//goth.UseProviders(google.New("676118167957-4bpa000p9bfaf5vu6halmen6nfjnuo1r.apps.googleusercontent.com", "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", "https://testfdfdfdfd-f824fe0fe06f.herokuapp.com/auth/google/callback"))
	goth.UseProviders(google.New("676118167957-4bpa000p9bfaf5vu6halmen6nfjnuo1r.apps.googleusercontent.com", "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", "http://localhost:3000/auth/google/callback"))
	secret_key := []byte("secret-password") // used to encrypt cookies, have to use a stronger password, but used a simple one for illustration purposes
	store = sessions.NewCookieStore(secret_key)
	store.Options.HttpOnly = true // just to make sure javascript doesn't access cookies
	gothic.Store = store          // not sure why we do this, but if we do, gothic will handle all cookie authentication
	//port = os.Getenv("PORT")
	port = "3000"

	// create a room for users to chat in
	r := newRoom()
	listOfRooms = append(listOfRooms, r)

	// this library requires us to go from more specific to less specific
	p := pat.New()

	p.Get("/logout/google", logoutHandler)
	p.Get("/chat", chatHandler)
	p.Get("/success", successHandler)
	p.Get("/auth/google/callback", googleHandler)
	p.Get("/auth/{provider}", authHandler)
	p.Get("/room", roomHandler)
	p.Get("/game", gameHandler)
	p.Get("/", indexHandler)

	go r.run()

	// run the server
	log.Println("listening on localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, p))
}

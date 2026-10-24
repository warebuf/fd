package main

import (
	"fmt"
	"github.com/google/uuid" // https://github.com/google/uuid
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
)

func indexHandler(res http.ResponseWriter, req *http.Request) {
	log.Println("serving index")
	t := template.Must(template.ParseFiles(filepath.Join("static", "index.html")))
	t.Execute(res, nil)
}

func loginHandler(res http.ResponseWriter, req *http.Request) {
	log.Println("serving login")

	session, err := store.Get(req, "session-name")
	fmt.Println("session values", session.Values)
	fmt.Println("session id", session.ID)
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
	log.Println("serving callback")

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

	not_assigned := true
	var random_number uuid.UUIDd
	for not_assigned {
		random_number = uuid.New()
		uid_to_user.mutex.Lock()
		if _, found := uid_to_user.users[random_number]; !found {
			uid_to_user.mutex.Unlock()
			not_assigned = false
			fmt.Println("UID:", random_number)
		}
	}

	session.Values["Email"] = user.Email
	session.Values["authenticated"] = true
	session.Values["uid"] = random_number.String()
	fmt.Println("session val:", session.Values)
	fmt.Println("session ID:", session.ID)

	// add the changes to the cookie
	err = session.Save(req, res)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	uid_to_user.mutex.Lock()
	uid_to_user.users[random_number] = &user_info{
		uid:          random_number,
		email:        user.Email,
		nickname:     "bob",
		sockets:      make(map[*websocket.Conn]*room),
		join:         make(chan *websocket.Conn),
		leave:        make(chan *websocket.Conn),
		chat_history: make(map[*websocket.Conn]chan *message),
	}
	uid_to_user.mutex.Unlock()

	fmt.Printf("user: %+v\n", uid_to_user.users[random_number])

	t.Execute(res, user)
}

func successHandler(res http.ResponseWriter, req *http.Request) {
	log.Println("serving success")

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
	log.Println("serving chat")

	// Check if user is already authenticated
	session, err := store.Get(req, "session-name")
	fmt.Printf("%+v\n", session)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); ok && auth {

		// have the break down URL
		fmt.Println(req.URL.Path)
		parts := strings.Split(req.URL.Path, "/")

		if len(parts) >= 3 {
			fmt.Println("sss", parts)
			if _, err := uuid.Parse(parts[2]); err == nil {

				t := template.Must(template.ParseFiles(filepath.Join("static", "chat.html")))
				fmt.Printf("%+v\n", session.Values["Email"].(string))

				data := map[string]string{"email": session.Values["Email"].(string), "host": req.Host, "room_id": parts[2]}
				t.Execute(res, data)

			} else {
				http.Redirect(res, req, "/lobby", http.StatusSeeOther)
			}
		} else {
			http.Redirect(res, req, "/lobby", http.StatusSeeOther)
		}

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

	u := req.URL
	parameters, err := url.ParseQuery(u.RawQuery)
	fmt.Println(u, parameters, err)

	if err != nil {
		// vulnerability here
	} else {
		fmt.Println(parameters["rid"][0])
		fmt.Printf("%+T\n", parameters["rid"][0])
		i, err := uuid.Parse(parameters["rid"][0])
		if err != nil {
			// ... handle error
			panic(err)
		} else {
			fmt.Println("made it")
			rid_to_room.rooms[i].ServeHTTP(res, req)
		}
	}
}

func gameHandler(res http.ResponseWriter, req *http.Request) {
	//c, _ := req.Cookie("session-name")

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

func lobbyHandler(res http.ResponseWriter, req *http.Request) {
	fmt.Println("lobby")

	session, err := store.Get(req, "session-name")
	fmt.Println(session.Values)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); ok && auth {

		data := struct {
			Room_error string
			Table      map[string]uuid.UUID
		}{}

		if session.Values["Room_error"] != 0 {
			switch session.Values["Room_error"] {
			case 1:
				data.Room_error = "this room exists already"
			case 2:
				data.Room_error = "error retrieving this room name"
			case 3:
				data.Room_error = "cannot have no name"
			case 4:
				data.Room_error = "cannot delete this room"
			case 5:
				data.Room_error = "please do not delete global"
			}
			session.Values["Room_error"] = 0
			err = session.Save(req, res)
			if err != nil {
				http.Error(res, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		data.Table = roomname_to_rid.name_to_rid

		t := template.Must(template.ParseFiles(filepath.Join("static", "lobby.html")))
		t.Execute(res, data)
	} else {
		fmt.Println("User is not authenticated, redirecting to home page")
		http.Redirect(res, req, "/", http.StatusSeeOther)
	}
}

// TODO: note that allowing the user to create their own room name is a vulnerability
func createRoomHandler(res http.ResponseWriter, req *http.Request) {
	fmt.Println("wassup wassup")

	fmt.Printf("%+v\n", store)

	session, err := store.Get(req, "session-name")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	u := req.URL
	parameters, err := url.ParseQuery(u.RawQuery)

	if err != nil {

		session.Values["Room_error"] = 2
		err = session.Save(req, res)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(res, req, "/lobby", http.StatusFound)
	} else {
		fmt.Println(parameters["room_name"][0]) // vulnerability here, need to check if this exists

		roomname_to_rid.mutex.Lock()
		_, ok := roomname_to_rid.name_to_rid[parameters["room_name"][0]]
		roomname_to_rid.mutex.Unlock()

		if ok {
			session.Values["Room_error"] = 1
			err = session.Save(req, res)
			if err != nil {
				http.Error(res, err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(res, req, "/lobby", http.StatusFound)
		} else {
			// TODO: probably have to do more string cleanup here
			if len(strings.TrimSpace(parameters["room_name"][0])) == 0 {
				session.Values["Room_error"] = 3
				err = session.Save(req, res)
				if err != nil {
					http.Error(res, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {

				not_assigned := true
				var random_number uuid.UUID
				for not_assigned {
					random_number = uuid.New()
					rid_to_room.mutex.Lock()
					_, found := rid_to_room.rooms[random_number]
					rid_to_room.mutex.Unlock()
					if !found {
						not_assigned = false
						fmt.Println("UID:", random_number)
					}
				}

				rid_to_room.mutex.Lock()
				rid_to_room.rooms[random_number] = newRoom(random_number, parameters["room_name"][0])
				rid_to_room.mutex.Unlock()
				roomname_to_rid.mutex.Lock()
				roomname_to_rid.name_to_rid[parameters["room_name"][0]] = random_number
				roomname_to_rid.mutex.Unlock()
				go rid_to_room.rooms[random_number].run()
			}

		}
		http.Redirect(res, req, "/lobby", http.StatusFound)
	}
}

func deleteRoomHandler(res http.ResponseWriter, req *http.Request) {

	session, err := store.Get(req, "session-name")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	u := req.URL
	parameters, err := url.ParseQuery(u.RawQuery)
	fmt.Println(u)

	if err != nil {
		session.Values["Room_error"] = 2
		err = session.Save(req, res)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(res, req, "/lobby", http.StatusFound)
	} else {

		fmt.Println(parameters["room_name"][0]) // vulnerability here, need to check if this exists

		roomname_to_rid.mutex.Lock()
		rid, ok := roomname_to_rid.name_to_rid[parameters["room_name"][0]]
		roomname_to_rid.mutex.Unlock()
		if !ok {
			session.Values["Room_error"] = 4
			err = session.Save(req, res)
			if err != nil {
				http.Error(res, err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(res, req, "/lobby", http.StatusFound)
		} else {
			if parameters["room_name"][0] == "global" {
				session.Values["Room_error"] = 5
				err = session.Save(req, res)
				if err != nil {
					http.Error(res, err.Error(), http.StatusInternalServerError)
					return
				}
				http.Redirect(res, req, "/lobby", http.StatusFound)
			} else {
				rid_to_room.rooms[rid].open <- false

				http.Redirect(res, req, "/lobby", http.StatusFound)
			}

		}
	}
}

// Global Variables - Utility
var store *sessions.CookieStore // used to create and decrypt cookies
var port string

type user_map struct {
	users map[uuid.UUID]*user_info
	mutex sync.Mutex
}

type room_map struct {
	rooms map[uuid.UUID]*room
	mutex sync.Mutex
}

type room_organization struct {
	name_to_rid map[string]uuid.UUID
	mutex       sync.Mutex
}

var uid_to_user user_map
var rid_to_room room_map
var roomname_to_rid room_organization

type user_info struct {
	uid      uuid.UUID
	email    string
	nickname string

	sockets map[*websocket.Conn]*room // socket is the web socket for this client.
	join    chan *websocket.Conn
	leave   chan *websocket.Conn

	chat_history map[*websocket.Conn]chan *message // send is a channel on which messages are sent.
}

func main() {

	// making global variables
	uid_to_user.users = make(map[uuid.UUID]*user_info)
	uid_to_user.mutex = sync.Mutex{}
	rid_to_room.rooms = make(map[uuid.UUID]*room)
	rid_to_room.mutex = sync.Mutex{}

	roomname_to_rid.name_to_rid = make(map[string]uuid.UUID)
	roomname_to_rid.mutex = sync.Mutex{}

	// init
	// this key has been revoked
	//goth.UseProviders(google.New("676118167957-4bpa000p9bfaf5vu6halmen6nfjnuo1r.apps.googleusercontent.com", "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", "https://testfdfdfdfd-f824fe0fe06f.herokuapp.com/callback"))
	goth.UseProviders(google.New("676118167957-4bpa000p9bfaf5vu6halmen6nfjnuo1r.apps.googleusercontent.com", "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", "http://localhost:3000/callback"))
	secret_key := []byte("secret-password") // used to encrypt cookies, have to use a stronger password, but used a simple one for illustration purposes
	store = sessions.NewCookieStore(secret_key)
	store.Options.HttpOnly = true // just to make sure javascript doesn't access cookies
	gothic.Store = store          // not sure why we do this, but if we do, gothic will handle all cookie authentication
	//port = os.Getenv("PORT")
	port = "3000"

	// create a global room for users to chat in
	not_assigned := true
	for not_assigned {
		random_number := uuid.New()
		rid_to_room.mutex.Lock()
		_, found := rid_to_room.rooms[random_number]
		rid_to_room.mutex.Unlock()
		if !found {
			rid_to_room.mutex.Lock()
			rid_to_room.rooms[random_number] = newRoom(random_number, "global")
			rid_to_room.mutex.Unlock()
			roomname_to_rid.mutex.Lock()
			roomname_to_rid.name_to_rid["global"] = random_number
			roomname_to_rid.mutex.Unlock()
			not_assigned = false
			fmt.Println("RID:", random_number)
			go rid_to_room.rooms[random_number].run()
		}
	}

	mux := http.NewServeMux() // create the default multiplexer
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("/game", gameHandler)

	mux.HandleFunc("/chat/", chatHandler)
	mux.HandleFunc("/lobby", lobbyHandler)
	mux.HandleFunc("/room", roomHandler)
	mux.HandleFunc("/createRoom", createRoomHandler)
	mux.HandleFunc("/deleteRoom", deleteRoomHandler)

	mux.HandleFunc("/callback", callbackHandler)
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/logout/google", logoutHandler)

	mux.HandleFunc("/success", successHandler)
	mux.HandleFunc("/", indexHandler)

	// run the server
	log.Println("listening on localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

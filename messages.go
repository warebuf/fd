package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// a room is a map of clients, as well as a queue of people wanting to join/leave
type room struct {
	forward chan *message       // a channel is a thread-safe queue, incoming messages
	join    chan *user_info     // a channel for clients wishing to join
	leave   chan *user_info     // a channel for clients wishing to leave
	clients map[*user_info]bool // all current clients

	rid int
}

type message struct {
	Name    string
	Message string
	When    time.Time
}

func (c *user_info) read(rid int) {
	for {
		var msg *message
		if err := c.socket.ReadJSON(&msg); err == nil {
			msg.When = time.Now()
			msg.Name = c.email
			c.current_rooms[rid].forward <- msg
		} else {
			break
		}
	}
	c.socket.Close()
}

// will loop infinitely until channel is closed
func (c *user_info) write(rid int) {
	for msg := range c.chat_history[rid] {
		if err := c.socket.WriteJSON(msg); err != nil {
			break
		}
	}
	c.socket.Close()
}

// this function runs forever, but if we run as a Go routine, it will run in the background
// checks all 3 channels, runs the code if one hits
func (r *room) run() {
	for {
		select {
		case client := <-r.join: // joining
			r.clients[client] = true
		case client := <-r.leave: // leaving
			delete(r.clients, client)
			close(client.chat_history[r.rid])
		case msg := <-r.forward: // forward message to all clients
			for client := range r.clients {
				select {
				case client.chat_history[r.rid] <- msg: // send the message
				default: // failed to send
					delete(r.clients, client)
					close(client.chat_history[r.rid])
				}
			}
		}
	}
}

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

var upgrader = &websocket.Upgrader{ReadBufferSize: socketBufferSize, WriteBufferSize: socketBufferSize}

func (r *room) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	log.Println("room", w, r)

	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Fatal("ServeHTTP:", err)
		return
	}

	c, _ := req.Cookie("session-name")
	fmt.Printf("%+v\n", c.Value)

	temp := cookie_to_user_info[c.Value]

	temp.socket = socket
	temp.chat_history[r.rid] = make(chan *message)
	temp.current_rooms[r.rid] = r

	r.join <- cookie_to_user_info[c.Value]
	defer func() {
		r.leave <- cookie_to_user_info[c.Value]
		fmt.Println("client left the room")
	}()
	go cookie_to_user_info[c.Value].write(r.rid)
	cookie_to_user_info[c.Value].read(r.rid)
}

func newRoom() *room {
	return &room{
		forward: make(chan *message),
		join:    make(chan *user_info),
		leave:   make(chan *user_info),
		clients: make(map[*user_info]bool),
	}
}

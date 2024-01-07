package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// client represents a single chatting user.
type client struct {
	socket *websocket.Conn // socket is the web socket for this client.
	send   chan []byte     // send is a channel on which messages are sent.
	room   *room           // room is the room this client is chatting in.
}

// a room is a map of clients, as well as a queue of people wanting to join/leave
type room struct {
	forward chan []byte      // a channel is a thread-safe queue, incoming messages
	join    chan *client     // a channel for clients wishing to join
	leave   chan *client     // a channel for clients wishing to leave
	clients map[*client]bool // all current clients
}

func (c *client) read() {
	for {
		if _, msg, err := c.socket.ReadMessage(); err == nil {
			c.room.forward <- msg
		} else {
			break
		}
	}
	c.socket.Close()
}

func (c *client) write() {
	for msg := range c.send {
		if err := c.socket.WriteMessage(websocket.TextMessage, msg); err != nil {
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
			close(client.send)
		case msg := <-r.forward: // forward message to all clients
			for client := range r.clients {
				select {
				case client.send <- msg: // send the message
				default: // failed to send
					delete(r.clients, client)
					close(client.send)
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
	client := &client{
		socket: socket,
		send:   make(chan []byte, messageBufferSize),
		room:   r,
	}
	r.join <- client
	defer func() {
		r.leave <- client
		fmt.Println("client left the room")
	}()
	go client.write()
	client.read()
}

func newRoom() *room {
	return &room{
		forward: make(chan []byte),
		join:    make(chan *client),
		leave:   make(chan *client),
		clients: make(map[*client]bool),
	}
}

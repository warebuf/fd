package main

import (
	"fmt"
	"github.com/google/uuid"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// a room is a map of clients, as well as a queue of people wanting to join/leave
type room struct {
	forward chan *message // a channel is a thread-safe queue, incoming messages
	join    chan struct {
		*websocket.Conn
		uuid.UUID
	} // a channel for clients wishing to join
	leave   chan *websocket.Conn           // a channel for clients wishing to leave
	sockets map[*websocket.Conn]*user_info // cid

	open      chan bool
	rid       uuid.UUID
	room_name string
}

func newRoom(rid uuid.UUID, name string) *room {
	return &room{
		forward: make(chan *message),
		join: make(chan struct {
			*websocket.Conn
			uuid.UUID
		}),
		leave:     make(chan *websocket.Conn),
		sockets:   make(map[*websocket.Conn]*user_info),
		rid:       rid,
		room_name: name,
		open:      make(chan bool),
	}
}

type message struct {
	Name    string
	Message string
	When    time.Time
}

func (r *room) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	log.Println("room!!!")

	session, err := store.Get(req, "session-name")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Fatal("ServeHTTP:", err)
		return
	}

	fmt.Printf("+%T\n", session.Values["uid"])
	fmt.Println(session.Values["uid"])
	uid, _ := uuid.Parse(session.Values["uid"].(string))

	uid_to_user.mutex.Lock()
	uid_to_user.users[uid].sockets[socket] = r
	uid_to_user.users[uid].chat_history[socket] = make(chan *message)
	uid_to_user.mutex.Unlock()

	r.join <- struct {
		*websocket.Conn
		uuid.UUID
	}{socket, uid}

	go uid_to_user.users[uid].write(socket)
	go uid_to_user.users[uid].read(socket)
}

// this function runs forever, but if we run as a Go routine, it will run in the background
// checks all 3 channels, runs the code if one hits
func (r *room) run() {
	for {
		select {
		case cid := <-r.join: // joining
			r.sockets[cid.Conn] = uid_to_user.users[cid.UUID]
			//fmt.Printf("joining socket %+v\n", cid.Conn)
			fmt.Printf("joining socket %+v\n", cid.NetConn())
		case cid := <-r.leave: // leaving
			fmt.Printf("leaving socket %+v\n", cid.NetConn())

			uid_to_user.mutex.Lock()
			close(r.sockets[cid].chat_history[cid])
			delete(r.sockets[cid].chat_history, cid)
			delete(r.sockets[cid].sockets, cid)
			uid_to_user.mutex.Unlock()
			rid_to_room.mutex.Lock()
			delete(r.sockets, cid)
			rid_to_room.mutex.Unlock()

			fmt.Printf("this many sockets left %+v\n", len(r.sockets))
			for i, _ := range r.sockets {
				fmt.Printf("this socket still exists %+v \n", i.NetConn())
			}

			cid.Close()

		case msg := <-r.forward: // forward message to all clients
			for i, j := range r.sockets {
				select {
				case j.chat_history[i] <- msg: // send the message
				default: // failed to send
					fmt.Println("failed to send message, cid", i)
					fmt.Printf("+%v\n", i)
					fmt.Printf("+%v\n", j)

					uid_to_user.mutex.Lock()
					close(r.sockets[i].chat_history[i])
					delete(r.sockets[i].chat_history, i)
					delete(r.sockets[i].sockets, i)
					uid_to_user.mutex.Lock()
					delete(r.sockets, i)

					i.Close()
				}
			}
		case still_open := <-r.open:
			fmt.Printf("deleting this room\n")
			// maybe should go through each user and delete
			for i, j := range r.sockets {
				uid_to_user.mutex.Lock()
				close(r.sockets[cid].chat_history[cid])
				delete(r.sockets[cid].chat_history, cid)
				delete(r.sockets[cid].sockets, cid)
				uid_to_user.mutex.Unlock()
				rid_to_room.mutex.Lock()
				delete(r.sockets, cid)
				rid_to_room.mutex.Unlock()
			}
			fmt.Printf("deleting this room2\n")
			roomname_to_rid.mutex.Lock()
			delete(roomname_to_rid.name_to_rid, r.room_name)
			roomname_to_rid.mutex.Unlock()
			fmt.Printf("deleting this room3\n")
			rid_to_room.mutex.Lock()
			delete(rid_to_room.rooms, r.rid)
			rid_to_room.mutex.Unlock()
			fmt.Printf("deleting this room4\n")
			if still_open {
				return
			}
			return
		}
	}
}

func (c *user_info) read(socket *websocket.Conn) {
	defer func() {
		fmt.Printf("client left the room ", socket.NetConn())
		c.sockets[socket].leave <- socket
	}()

	for {
		var msg *message
		if err := socket.ReadJSON(&msg); err == nil {
			fmt.Println("reading msg")
			//fmt.Printf("socket: %+v\n", socket)
			fmt.Printf("reading from socket %+v\n", socket.NetConn())

			msg.When = time.Now()
			msg.Name = c.email
			fmt.Println(c.sockets[socket].room_name)
			c.sockets[socket].forward <- msg
		} else {
			break
		}
	}
}

func (c *user_info) write(socket *websocket.Conn) {
	for msg := range c.chat_history[socket] { // special trait of a channel, will loop infinitely until channel is closed
		if err := socket.WriteJSON(msg); err != nil {
			fmt.Println("error writing to socket")
			break
		} else {
			fmt.Printf("writing from socket %+v\n", socket.NetConn())
		}
	}
}

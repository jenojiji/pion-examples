package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type Server struct {
	room    *Room
	counter int32
}

func NewServer() *Server {
	return &Server{
		room: NewRoom(),
	}
}

func (s *Server) nextID() int {
	return int(atomic.AddInt32(&s.counter, 1))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		ID:   s.nextID(),
		Conn: conn,
	}

	client.readyChan = make(chan struct{})

	if err := s.room.Add(client); err != nil {
		conn.Close()
		http.Error(w, "room full", http.StatusForbidden)
		return
	}

	pc, err := NewPeer(client, s.room)
	if err != nil {
		conn.Close()
		return
	}
	client.PC = pc

	log.Printf("Client %d connected\n", client.ID)

	defer func() {
		log.Printf("Client %d disconnected\n", client.ID)
		pc.Close()
		s.room.Remove(client.ID)
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		fmt.Println("incoming message")
		if err != nil {
			return
		}
		s.handleSignal(client, msg)
	}
}

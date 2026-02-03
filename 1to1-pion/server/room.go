package main

import (
	"fmt"
	"sync"
)

type Room struct {
	mu      sync.Mutex
	clients map[int]*Client
}

func NewRoom() *Room {
	return &Room{
		clients: make(map[int]*Client),
	}
}

func (r *Room) Add(c *Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.clients) >= 2 {
		return fmt.Errorf("room full")
	}

	r.clients[c.ID] = c
	return nil
}

func (r *Room) Remove(id int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, id)
}

func (r *Room) Other(id int) *Client {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.clients {
		if c.ID != id {
			return c
		}
	}
	return nil
}

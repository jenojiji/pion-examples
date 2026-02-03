package main

import "encoding/json"

type Message struct {
	Type string `json:"type"`
	Data json.RawMessage `json:"data"`
}

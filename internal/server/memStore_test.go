package server

import (
	"log"
	"testing"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		err  error
		name string
		arg  []byte
	}{}

	for _, test := range tests {
		log.Println("\n\nTEST:", test.name)
	}
}

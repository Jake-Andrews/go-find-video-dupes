package main

import (
	"log"
)

type Sneed struct {
	Feed  string
	Chuck string
}

func sneed() {
	sn := Sneed{}
	log.Println(sn)

}

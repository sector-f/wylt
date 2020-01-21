package main

import "fmt"

type Player interface {
	Subscribe() <-chan
}

type Target interface {
	Publish() string
}

func main() {
	fmt.Println("main.go")
}

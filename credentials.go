package luno_go_examples

import (
	"io/ioutil"
	"log"
)

func ReadSecret(path string) string {
	secret, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("Error reading api secret: ", err)
	}
	return string(secret)
}

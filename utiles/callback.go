package utils

import (
	"log"

	"github.com/billikeu/Go-EdgeGPT/edgegpt"
)

var globalString string = "globalString"

func Callback(answer *edgegpt.Answer) {
	if answer.IsDone() {
		log.Println("Done", answer.Text())
		globalString = answer.Text()
		// log.Println(answer.NumUserMessages(), answer.MaxNumUserMessages(), answer.Text())
	}
}

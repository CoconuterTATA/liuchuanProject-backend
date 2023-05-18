package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/billikeu/Go-EdgeGPT/edgegpt"
	"github.com/sashabaranov/go-openai"
)

func processDSLNewBing(dsl string) string {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	bot := edgegpt.NewChatBot("cookie.json", []map[string]interface{}{}, "http://127.0.0.1:7890")
	err := bot.Init()

	if err != nil {
		panic(err)
	}
	err = bot.Ask("请你给我的回复中不要带有任何emoji表情", edgegpt.Creative, callback)
	if err != nil {
		panic(err)
	}

	err = bot.Ask(dsl, edgegpt.Creative, callback)
	if err != nil {
		panic(err)
	}
	return globalString

	// err = bot.Ask("It's not funny", edgegpt.Creative, callback)
	// if err != nil {
	// 	panic(err)
	// }

}

func processDSL(dsl string) string {
	config := openai.DefaultConfig("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")
	proxyUrl, err := url.Parse("http://localhost:7890")
	if err != nil {
		panic(err)
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
	}
	config.HTTPClient = &http.Client{
		Transport: transport,
	}

	client := openai.NewClientWithConfig(config)
	fmt.Println(dsl)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: dsl,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "error"
	}

	fmt.Println(resp.Choices[0].Message.Content)
	return resp.Choices[0].Message.Content
}

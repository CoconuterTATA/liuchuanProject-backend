package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/billikeu/Go-EdgeGPT/edgegpt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const apiKey = "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" // 替换为您的实际 OpenAI API 密钥

type ABIEvent struct {
	Anonymous bool              `json:"anonymous"`
	Inputs    []json.RawMessage `json:"inputs"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
}

type ABIInput struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type OpenAIRequest struct {
	Prompt string `json:"prompt"`
}

type EventOutput struct {
	Events []string `json:"events"`
}

type ABIElement struct {
	Type    string            `json:"type"`
	Name    string            `json:"name,omitempty"`
	Inputs  []json.RawMessage `json:"inputs,omitempty"`
	Outputs []json.RawMessage `json:"outputs,omitempty"`
}

type ABIOutput struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type OpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Text         string      `json:"text"`
		Index        int         `json:"index"`
		Logprobs     interface{} `json:"logprobs"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

var globalString string = "globalString"

func main() {
	// 创建路由
	router := gin.Default()

	// 跨域配置
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:8081"}
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	router.Use(cors.New(config))

	router.POST("/generateEvents", generateEventsHandler)

	router.POST("/newbing", func(c *gin.Context) {
		// 获取请求中的数据
		var inputData map[string]string
		if err := c.ShouldBindJSON(&inputData); err != nil {
			log.Println(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据不合法"})
			return
		}
		dsl := inputData["dsl"]
		// cookie := inputData["cookie"]
		// fmt.Println(dsl)
		// 处理 DSL 代码，生成新的代码
		// 调用 NewBing API 处理 DSL 代码，生成新的代码
		processedCodeFrombing := processDSLNewBing(dsl)
		c.JSON(http.StatusOK, gin.H{"code": processedCodeFrombing})

	})

	// 定义路由

	router.POST("/generate", func(c *gin.Context) {
		// 获取请求中的数据
		var inputData map[string]string
		if err := c.ShouldBindJSON(&inputData); err != nil {
			log.Println(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据不合法"})
			return
		}
		dsl := inputData["dsl"]
		fmt.Println(dsl)
		// 处理 DSL 代码，生成新的代码
		processedCode := processDSL(dsl)

		// 返回处理后的代码

		c.JSON(http.StatusOK, gin.H{"code": processedCode})
	})
	router.POST("/compile", func(c *gin.Context) {
		// 获取请求中的数据
		var inputData map[string]string
		if err := c.ShouldBindJSON(&inputData); err != nil {
			log.Println(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据不合法"})
			return
		}
		log.Printf("前端发送的数据：%v\n", inputData)

		// 编译 Solidity 代码
		compiledResult, err := compileSolidity(inputData["input"])
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "编译过程中发生错误"})
			return
		}
		// 返回编译结果
		c.JSON(http.StatusOK, gin.H{"abi": compiledResult["abi"], "bytecode": compiledResult["bytecode"]})
	})

	// 启动路由
	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

func callback(answer *edgegpt.Answer) {
	if answer.IsDone() {
		log.Println("Done", answer.Text())
		globalString = answer.Text()
		// log.Println(answer.NumUserMessages(), answer.MaxNumUserMessages(), answer.Text())
	}
}

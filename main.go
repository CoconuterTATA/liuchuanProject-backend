package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pavel-one/EdgeGPT-Go"
	"github.com/sashabaranov/go-openai"
)

const apiKey = "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" // 替换为您的实际 OpenAI API 密钥

type OpenAIRequest struct {
	Prompt string `json:"prompt"`
}

type EventOutput struct {
	Events []string `json:"events"`
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
		fmt.Println(dsl)
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

func generateEventsHandler(c *gin.Context) {
	// 获取请求中的数据
	var inputData map[string]string
	if err := c.ShouldBindJSON(&inputData); err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据不合法"})
		return
	}

	events := generateEvents(inputData["input"])
	eventOutput := EventOutput{Events: events}

	output, err := json.Marshal(eventOutput)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成事件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": output})
}

func generateEvents(contractCode string) []string {
	// 定义变量命名规则的正则表达式
	variableRegex := regexp.MustCompile(`(uint256|uint|address|bool)\s+_([a-zA-Z0-9]+);`)

	// 查找所有符合规则的变量
	matches := variableRegex.FindAllStringSubmatch(contractCode, -1)

	var events []string
	for _, match := range matches {
		varType := match[1]
		varName := match[2]
		eventName := fmt.Sprintf("%sChanged", varName)
		eventSignature := fmt.Sprintf("event %s(%s _newValue);", eventName, varType)
		events = append(events, eventSignature)
	}

	return events
}

func compileSolidity(code string) (map[string]interface{}, error) {
	cmd := exec.Command("solc", "--combined-json", "bin,abi", "--optimize", "-")
	cmd.Stdin = bytes.NewBufferString(code)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Println("编译错误：", err, stderr.String())
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		log.Println("解析编译结果失败：", err)
		return nil, err
	}

	// 提取编译结果中的 ABI 和字节码
	contracts := result["contracts"].(map[string]interface{})
	compiledResult := make(map[string]interface{})

	for _, v := range contracts {
		contractData := v.(map[string]interface{})
		compiledResult["abi"] = contractData["abi"]
		compiledResult["bytecode"] = contractData["bin"]
		break
	}

	return compiledResult, nil
}

func processDSLNewBing(dsl string) string {
	s := EdgeGPT.NewStorage()

	gpt, err := s.GetOrSet("any-key")
	if err != nil {
		log.Fatalln(err)
	}

	// send ask async
	mw, err := gpt.AskAsync("Hi, you're alive?")
	if err != nil {
		log.Fatalln(err)
	}

	go mw.Worker() // start worker

	for range mw.Chan {
		// update answer
		log.Println(mw.Answer.GetAnswer())
		log.Println(mw.Answer.GetType())
		log.Println(mw.Answer.GetSuggestions())
		log.Println(mw.Answer.GetMaxUnit())
		log.Println(mw.Answer.GetUserUnit())
	}

	// send sync ask
	as, err := gpt.AskSync(dsl)
	if err != nil {
		log.Fatalln(err)
	}
	return as.Answer.GetAnswer()

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

// func generateFromGPT(c *gin.Context) {
// 	// 获取请求中的数据
// 	var inputData map[string]string
// 	if err := c.ShouldBindJSON(&inputData); err != nil {
// 		log.Println(err)
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据不合法"})
// 		return
// 	}
// 	prompt := inputData["prompt"]

// 	client := &http.Client{}
// 	apiRequest, err := json.Marshal(map[string]interface{}{
// 		"prompt": prompt,
// 		// 添加其他 API 参数，如 max_tokens、temperature 等
// 	})
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode API request"})
// 		return
// 	}
// 	log.Println("Sending API request:", string(apiRequest))
// 	request, err := http.NewRequest("POST", "https://api.openai.com/v1/engines/davinci-codex/completions", strings.NewReader(string(apiRequest)))
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API request"})
// 		return
// 	}
// 	request.Header.Set("Authorization", "Bearer "+apiKey)
// 	request.Header.Set("Content-Type", "application/json")

// 	response, err := client.Do(request)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call OpenAI API"})
// 		return
// 	}
// 	defer response.Body.Close()

// 	body, err := ioutil.ReadAll(response.Body)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read API response"})
// 		return
// 	}

// 	var openAIResponse OpenAIResponse
// 	err = json.Unmarshal(body, &openAIResponse)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode API response"})
// 		return
// 	}

// 	if len(openAIResponse.Choices) > 0 {
// 		c.JSON(http.StatusOK, gin.H{"result": openAIResponse.Choices[0].Text})
// 	} else {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "No results from API"})
// 	}

// }

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
	"strings"

	"github.com/billikeu/Go-EdgeGPT/edgegpt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
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

func generateEventsHandler(c *gin.Context) {
	// 获取请求中的数据
	var inputData map[string]string
	if err := c.ShouldBindJSON(&inputData); err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据不合法"})
		return
	}

	compiledResult, err := compileSolidity(inputData["input"])
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "编译过程中发生错误"})
		return
	}

	abiBytes, err := json.Marshal(compiledResult["abi"])
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化 ABI 失败"})
		return
	}
	abi := string(abiBytes)

	events, err := generateEvents(abi)

	// 将事件列表直接序列化为 JSON 数组，而不是序列化整个结构体
	// output, err := json.Marshal(events)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成事件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events})

}

func generateEvents(abi string) ([]string, error) {
	var abiEvents []ABIEvent
	err := json.Unmarshal([]byte(abi), &abiEvents)
	if err != nil {
		return nil, fmt.Errorf("解析 ABI 失败: %v", err)
	}

	eventRegex := regexp.MustCompile(`^(Investment|Withdrawal|Earnings|Distribution)[a-zA-Z0-9]+`)

	var events []string
	for _, event := range abiEvents {
		if event.Type == "event" && eventRegex.MatchString(event.Name) {
			var inputParams []string
			for _, input := range event.Inputs {
				var inputParam ABIInput
				err := json.Unmarshal(input, &inputParam)
				if err != nil {
					return nil, fmt.Errorf("解析 ABI 事件输入参数失败: %v", err)
				}
				inputParams = append(inputParams, fmt.Sprintf("%v %v", inputParam.Type, inputParam.Name))
			}
			eventSignature := fmt.Sprintf("event %s(%s);", event.Name, strings.Join(inputParams, ", "))
			events = append(events, eventSignature)
		}
	}

	return events, nil
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

func callback(answer *edgegpt.Answer) {
	if answer.IsDone() {
		log.Println("Done", answer.Text())
		globalString = answer.Text()
		// log.Println(answer.NumUserMessages(), answer.MaxNumUserMessages(), answer.Text())
	}
}

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

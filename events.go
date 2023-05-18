package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

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

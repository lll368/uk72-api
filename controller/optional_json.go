package controller

import (
	"bytes"
	"io"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// decodeOptionalJsonRequest 允许空请求体；非空请求体必须是合法 JSON，避免管理端坏请求继续执行状态变更。
func decodeOptionalJsonRequest(c *gin.Context, req any) error {
	if c.Request.Body == nil {
		return nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	return common.Unmarshal(body, req)
}

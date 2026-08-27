package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// GenID 生成一个带前缀的随机 ID（16 字节随机 + 时间戳后缀），用于实体主键。
func GenID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	randHex := hex.EncodeToString(buf)
	return prefix + "_" + time.Now().Format("20060102150405") + "_" + randHex
}

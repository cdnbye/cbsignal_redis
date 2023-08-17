package util

import (
	"cbsignal/client"
	"github.com/bytedance/sonic"
	"github.com/gobwas/ws"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// 获取域名（不包含端口）
func GetDomain(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	a := strings.Split(parsed.Host, ":")
	return a[0]
}

func GetVersionNum(ver string) int {
	digs := strings.Split(ver, ".")
	a, _ := strconv.Atoi(digs[0])
	b, _ := strconv.Atoi(digs[1])
	return a*10 + b
}

func WriteCustomStatusCode(conn net.Conn, code ws.StatusCode, reason string) {
	var body = ws.NewCloseFrameBody(code, reason)
	var frame = ws.NewCloseFrame(body)
	ws.WriteHeader(conn, frame.Header)
	conn.Write(body)
}

func WriteVersion(w http.ResponseWriter, version int) {
	resp := client.SignalVerResp{
		Ver: version,
	}
	b, err := sonic.Marshal(resp)
	if err == nil {
		w.Write(b)
	}
}

func SetHeaderJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

func SetOriginAllowAll(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

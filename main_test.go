package main

import (
	"bytes"
	"encoding/json"
	"github.com/mitchellh/mapstructure"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUtilizationRate(t *testing.T) {

	var a int64 = 1
	var b int64 = 3000
	var c float32 = float32(a) / float32(b)
	t.Logf("%f", c)
}

func TestVersion(t *testing.T) {
	ver := "0.1.1"
	digs := strings.Split(ver, ".")
	a, _ := strconv.Atoi(digs[0])
	b, _ := strconv.Atoi(digs[1])
	t.Logf("%d", a*10+b)
}

func TestTime(t *testing.T) {
	s := 2 * time.Second.Nanoseconds()
	start := time.Now()
	time.Sleep(300 * time.Microsecond)
	t.Logf("%d %d", time.Since(start).Nanoseconds(), s)
}

func TestFnv32(t *testing.T) {
	key := "ddffvfgfgf"
	hash := uint32(2166136261)
	const prime32 = uint32(16777619)
	for i := 0; i < len(key); i++ {
		hash *= prime32
		hash ^= uint32(key[i])
	}
	t.Log(hash)
}

type a struct {
	A string `json:"a"`
}

func TestMap(t *testing.T) {
	var result a
	m := map[string]string{
		"a": "12",
	}
	mapstructure.Decode(m, &result)
	t.Logf("%+v", result)
}

func TestJson(t *testing.T) {
	type obj struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	json1 := obj{
		Name: "x",
		Age:  20,
	}
	json2 := obj{
		Name: "t",
		Age:  33,
	}
	b1, _ := json.Marshal(json1)
	b2, _ := json.Marshal(json2)
	var buf bytes.Buffer
	//enc := json.NewEncoder(&buf)

	buf.WriteByte('[')
	buf.Write(b1)
	buf.WriteByte(',')
	buf.Write(b2)
	//enc.Encode(b1)
	//buf.WriteByte(',')
	//buf.UnreadByte()
	//enc.Encode(b2)
	buf.WriteByte(']')

	result := buf.Bytes()
	t.Log(string(result))

	type objs []obj
	var ret objs
	err := json.Unmarshal(result, &ret)
	if err != nil {
		t.Error(err)
	}
	t.Log(ret)
	t.Log(ret[0].Name)
}

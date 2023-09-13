package main

import (
	"bytes"
	"cbsignal/client"
	"cbsignal/handler"
	"cbsignal/hub"
	"cbsignal/nodes"
	"cbsignal/redis"
	"cbsignal/util"
	"cbsignal/util/fastmap/cmap"
	"cbsignal/util/log"
	"cbsignal/util/ratelimit"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	VERSION                   = "4.6.3"
	CHECK_CLIENT_INTERVAL     = 6 * 60
	KEEP_LIVE_INTERVAL        = 7
	REJECT_JOIN_CPU_Threshold = 720
	REJECT_MSG_CPU_Threshold  = 770
)

var (
	cfg     = pflag.StringP("config", "c", "", "Config file path.")
	newline = []byte{'\n'}
	space   = []byte{' '}

	selfIp   string
	selfAddr string

	signalPorts    []string
	signalPortsTLS []portTLS

	versionNum   int
	buffers      sync.Pool
	limitEnabled bool
	limitRate    int64
	limiter      *ratelimit.Bucket

	securityEnabled bool
	statsEnabled    bool
	maxTimeStampAge int64
	securityToken   string

	redisCli redis.RedisClient
)

type portTLS struct {
	Port string `json:"port"`
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

func init() {
	pflag.Parse()

	versionNum = util.GetVersionNum(VERSION)

	// Initialize viper
	if *cfg != "" {
		viper.SetConfigFile(*cfg) // 如果指定了配置文件，则解析指定的配置文件
	} else {
		viper.AddConfigPath("./") // 如果没有指定配置文件，则解析默认的配置文件
		viper.SetConfigName("config")
	}
	viper.SetConfigType("yaml") // 设置配置文件格式为YAML
	viper.AutomaticEnv()        // 读取匹配的环境变量
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	if err := viper.ReadInConfig(); err != nil { // viper解析配置文件
		panic(err)
	}

	// Initialize logger
	log.InitLogger(viper.GetString("log.writers"),
		viper.GetString("log.logger_level"),
		viper.GetBool("log.log_format_text"),
		viper.GetString("log.logger_dir")+"/signalhub.log",
		viper.GetInt("log.log_rotate_size"),
		viper.GetInt("log.log_backup_count"),
		viper.GetInt("log.log_max_age"))

	signalPorts = viper.GetStringSlice("port")
	if err := viper.UnmarshalKey("tls", &signalPortsTLS); err != nil {
		log.Fatal(err)
	}

	selfIp = util.GetInternalIP()
	if len(signalPorts) > 0 {
		selfAddr = fmt.Sprintf("%s:%s", selfIp, signalPorts[0])
	} else if len(signalPortsTLS) > 0 {
		selfAddr = fmt.Sprintf("%s:%s", selfIp, signalPortsTLS[0])
	} else {
		selfAddr = fmt.Sprintf("%s", selfIp)
	}

	// init redis client
	isRedisCluster := viper.GetBool("redis.is_cluster")
	redisPsw := viper.GetString("redis.password")
	if isRedisCluster {
		var redisAddrs []*redis.Addr
		err := viper.UnmarshalKey("redis.cluster", &redisAddrs)
		if err != nil {
			panic(err)
		}
		if len(redisAddrs) == 0 {
			redisAddrs = append(redisAddrs, &redis.Addr{
				Host: viper.GetString("redis.host"),
				Port: viper.GetString("redis.port"),
			})
		}
		redisCli = redis.InitRedisCluster(selfAddr, redisAddrs, redisPsw)
	} else {
		redisAddr := &redis.Addr{
			Host: viper.GetString("redis.host"),
			Port: viper.GetString("redis.port"),
		}
		redisDB := viper.GetInt("redis.dbname")
		redisCli = redis.InitRedisClient(selfAddr, redisAddr.String(), redisPsw, redisDB)
	}
	_, err := redisCli.Ping(context.Background()).Result()
	if err != nil {
		panic(err)
	}

	buffers = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	setupConfigFromViper()

	//开始监听
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		//log.Warnf("config is changed :%s \n", e.Name)
		setupConfigFromViper()
	})

	hub.Init(selfAddr)

	go checkConns()

	go keepAlive()
}

func main() {

	// Catch SIGINT signals
	intrChan := make(chan os.Signal)
	signal.Notify(intrChan, os.Interrupt)

	// Increase resources limitations
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	log.Warnf("signal version %s", VERSION)
	// rate limit
	if limitEnabled {
		log.Warnf("Init ratelimit with rate %d", limitRate)
		limiter = ratelimit.NewBucketWithQuantum(time.Second, limitRate, limitRate)
	}

	if securityEnabled {
		if maxTimeStampAge == 0 || securityToken == "" {
			panic("maxTimeStampAge or token is empty when security on")
		}
		if len(securityToken) > 8 {
			panic("security token is larger than 8")
		}
		log.Warnf("security on\nmaxTimeStampAge %d\ntoken %s", maxTimeStampAge, securityToken)
	}

	if len(signalPorts) > 0 {
		for _, port := range signalPorts {
			go func(port string) {
				log.Warnf("Start to listening the incoming requests on http address: %s\n", port)
				err := http.ListenAndServe(":"+port, nil)
				if err != nil {
					log.Fatal("ListenAndServe: ", err)
				}
			}(port)
		}
	}

	if len(signalPortsTLS) > 0 {
		for _, portTls := range signalPortsTLS {
			if portTls.Port != "" && util.Exists(portTls.Cert) && util.Exists(portTls.Key) {
				go func(portTls portTLS) {
					log.Warnf("Start to listening the incoming requests on https address: %s\n", portTls.Port)
					err := http.ListenAndServeTLS(":"+portTls.Port, portTls.Cert, portTls.Key, nil)
					if err != nil {
						log.Fatal("ListenAndServe: ", err)
					}
				}(portTls)

				certFile, err := ioutil.ReadFile(portTls.Cert)
				if err != nil {
					log.Error(err)
					continue
				}
				pemBlock, _ := pem.Decode(certFile)
				if pemBlock == nil {
					log.Error("decode error")
					continue
				}
				cert, err := x509.ParseCertificate(pemBlock.Bytes)
				if err != nil {
					log.Error(err)
					continue
				}
				handler.Certs = append(handler.Certs, handler.CertInfo{
					Name:     cert.Subject.CommonName,
					ExpireAt: cert.NotAfter,
				})
			}
		}
	}

	nodes.NewNodeHub(selfAddr)

	http.HandleFunc("/ws", commonHandler)
	http.HandleFunc("/wss", commonHandler)
	http.HandleFunc("/", commonHandler)

	if statsEnabled {
		http.HandleFunc("/count", handler.CountHandler())
		http.HandleFunc("/total_count", handler.TotalCountHandler())
		http.HandleFunc("/version", handler.VersionHandler(VERSION))

		info := handler.SignalInfo{
			Version:         VERSION,
			SecurityEnabled: securityEnabled,
		}
		if limitEnabled {
			info.RateLimit = limitRate
		}
		http.HandleFunc("/info", handler.StatsHandler(info))
	}
	// health check
	http.HandleFunc("/health_check", handler.HealthCheck())

	<-intrChan

	log.Warn("Shutting down server...")

	// do cleanup
	redisCli.Close()
	log.Sync()
}

func commonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		wsHandler(w, r)
	} else {
		util.SetOriginAllowAll(w)
		if r.Method == "GET" {
			pollingHandler(w, r)
		} else if r.Method == "POST" {
			postHandler(w, r)
		}
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	if !commonCheck() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	query := r.URL.Query()
	id := query.Get("id")
	isHello := query.Has("hello")
	//log.Printf("id %s", id)
	if id == "" || len(id) < 6 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	// 校验
	if securityEnabled && !checkToken(id, query.Get("token"), r.Header.Get("Origin")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	c, ok := hub.GetClient(id)
	if ok {
		if !c.IsPolling {
			// websocket
			w.WriteHeader(http.StatusConflict)
			return
		}
	} else {
		c = client.NewPollingPeerClient(id)
		hub.DoRegister(c)
		if isHello {
			util.WriteVersion(w, versionNum)
			return
		}
	}
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	hdrs, err := handler.NewPostHandler(b, c)
	if err != nil {
		//log.Error("NewPostHandler", err)
		//log.Warn(string(bytes))
	} else {
		for _, hdr := range hdrs {
			hdr.Handle()
		}
	}
}

func pollingHandler(w http.ResponseWriter, r *http.Request) {
	if !commonCheck() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	query := r.URL.Query()
	id := query.Get("id")
	//log.Printf("id %s", id)
	if id == "" || len(id) < 6 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	// 校验
	if securityEnabled && !checkToken(id, query.Get("token"), r.Header.Get("Origin")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	c, ok := hub.GetClient(id)
	if ok {
		if !c.IsPolling {
			// websocket or polling on
			w.WriteHeader(http.StatusConflict)
			return
		}
	} else {
		log.Infof("pollingHandler NewPollingPeerClient %s", id)
		c = client.NewPollingPeerClient(id)
		hub.DoRegister(c)
	}

	msgLen := len(c.MsgQueue)
	if msgLen > 0 {
		buf := buffers.Get().(*bytes.Buffer)
		buf.Reset()
		defer buffers.Put(buf)
		count := 0
		buf.WriteByte('[')
		for i := 0; i < msgLen; i++ {
			msg, ok := <-c.MsgQueue
			if !ok {
				break
			}
			buf.Write(msg)
			buf.WriteByte(',')
			count++
		}
		buf.Truncate(buf.Len() - 1) // 去掉最后一个逗号
		buf.WriteByte(']')
		util.SetHeaderJson(w)
		w.Write(buf.Bytes())
	} else {
		c.UpdateTs()
		select {
		case <-time.After(time.Duration(120) * time.Second):
			log.Infof("polling reach timeout")
		case msg, ok := <-c.MsgQueue:
			if ok {
				// Consume event.
				var buf bytes.Buffer
				buf.WriteByte('[')
				buf.Write(msg)
				buf.WriteByte(']')
				util.SetHeaderJson(w)
				w.Write(buf.Bytes())
			}
		case <-r.Context().Done():
			// Client connection closed before any events occurred and before the timeout was exceeded.
			// 节点离开
			log.Infof("pollingHandler peer leave %s", id)
			if c.IsPolling {
				if ok := hub.DoUnregister(id); ok {
					log.Infof("close polling peer %s", id)
					c.Close()
				}
			}
		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	id := query.Get("id")
	//platform := query.Get("p")
	//ver := query.Get("v")
	//log.Printf("id %s", id)

	// Upgrade connection
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.Error("UpgradeHTTP", err)
		return
	}

	if id == "" || len(id) < 6 {
		util.WriteCustomStatusCode(conn, 4000, "id is not valid")
		return
	}
	// 校验
	if securityEnabled && !checkToken(id, query.Get("token"), r.Header.Get("Origin")) {
		util.WriteCustomStatusCode(conn, 4000, "token is not valid")
		return
	}
	c, ok := hub.GetClient(id)
	if ok {
		if !c.IsPolling {
			util.WriteCustomStatusCode(conn, 4000, "ws already exist")
			return
		}
		log.Infof("%s switch from polling to ws", id)
		c.IsPolling = false
		c.Conn = conn
		msgLen := len(c.MsgQueue)
		for i := 0; i < msgLen; i++ {
			msg, ok := <-c.MsgQueue
			if !ok {
				break
			}
			c.SendDataWs(msg)
		}
		close(c.MsgQueue)
		c.UpdateTs()
	} else {
		c = client.NewWebsocketPeerClient(id, conn)
		hub.DoRegister(c)
	}

	if !commonCheck() {
		util.WriteCustomStatusCode(conn, 5000, "check failed")
		return
	}

	// 发送版本号
	c.SendMsgVersion(versionNum)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Error(err)
			}
			// 节点离开
			log.Infof("peer leave")
			if ok := hub.DoUnregister(id); ok {
				//log.Infof("close peer %s", id)
				c.Close()
			}
		}()
		//err := conn.SetReadDeadline(time.Now().Add(h.heartbeat * 3))
		msg := make([]wsutil.Message, 0, 4)
		for {
			msg, err = wsutil.ReadClientMessage(conn, msg[:0])
			if err != nil {
				log.Infof("read message error: %v", err)
				break
			}
			cpuUsage := atomic.LoadInt64(&handler.G_CPU)
			for _, m := range msg {
				// ping
				if m.OpCode.IsControl() {
					if m.OpCode == ws.OpClose {
						conn.Close()
						return
					}
					c.UpdateTs()
					if err := redis.UpdateLocalPeerExpiration(c.PeerId); err != nil {
						log.Error(err)
					}
					//log.Warnf("receive ping from %s platform %s", id, platform)
					err := wsutil.HandleClientControlMessage(conn, m)
					if err != nil {
						log.Infof("handle control error: %v", err)
					}
					continue
				}

				// 限流
				if cpuUsage > REJECT_MSG_CPU_Threshold {
					log.Warnf("handle msg reach cpu %d", cpuUsage)
					break
				}
				if !redis.IsAlive {
					log.Warnf("handle msg redis not alive")
					break
				}

				data := bytes.TrimSpace(bytes.Replace(m.Payload, newline, space, -1))
				hdr, err := handler.NewHandler(data, c)
				if err != nil {
					// 心跳包
					//c.UpdateTs()   // TODO 验证
					log.Error("NewHandler", err)
				} else {
					hdr.Handle()
				}
			}
		}
	}()
}

func setupConfigFromViper() {
	limitEnabled = viper.GetBool("ratelimit.enable")
	limitRate = viper.GetInt64("ratelimit.max_rate")
	securityEnabled = viper.GetBool("security.enable")
	maxTimeStampAge = viper.GetInt64("security.maxTimeStampAge")
	securityToken = viper.GetString("security.token")
	statsEnabled = viper.GetBool("stats.enable")
	handler.StatsToken = viper.GetString("stats.token")
}

func checkConns() {
	ticker := time.NewTicker(CHECK_CLIENT_INTERVAL * time.Second) // same to cpu sample rate
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			go checkConns()
		}
	}()

	for range ticker.C {
		log.Infof("start check client alive...")
		go func() {
			wsCountRemoved := 0
			httpCountRemoved := 0
			wsCount := 0
			httpCount := 0
			for i := 0; i < cmap.SHARD_COUNT; i++ {
				now := time.Now().Unix()
				for val := range hub.GetInstance().Clients.GetChanByShard(i) {
					cli := val.Val
					if cli.IsExpired(now) {
						// 节点过期
						//log.Warnf("client %s is expired for %d, close it", cli.PeerId, now-cli.Timestamp)
						if ok := hub.DoUnregister(cli.PeerId); ok {
							cli.Close()
							if cli.IsPolling {
								httpCountRemoved++
							} else {
								wsCountRemoved++
							}
						}
					} else {
						if cli.IsPolling {
							httpCount++
						} else {
							wsCount++
						}
					}
				}
				time.Sleep(2 * time.Second)
			}
			if wsCountRemoved > 0 || httpCountRemoved > 0 {
				log.Warnf("check cmap finished, closed clients: ws %d polling %d", wsCountRemoved, httpCountRemoved)
			}
			log.Warnf("current clients ws %d, polling %d", wsCount, httpCount)
		}()
	}
}

func keepAlive() {
	ticker := time.NewTicker(KEEP_LIVE_INTERVAL * time.Second) // same to cpu sample rate
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			go keepAlive()
		}
	}()

	for range ticker.C {
		if redis.IsAlive {
			//log.Infof("start update...")
			if err := redis.UpdateClientCount(hub.GetClientCount()); err != nil {
				log.Error("UpdateClientCount", err)
			}
		}
	}
}

func checkToken(id, queryToken, origin string) bool {
	token := strings.Split(queryToken, "-")
	if len(token) < 2 {
		log.Warnf("token not valid %s origin %s", queryToken, origin)
		return false
	}
	sign := token[0]
	tsStr := token[1]
	if ts, err := strconv.ParseInt(tsStr, 10, 64); err != nil {
		//log.Warnf("ts ParseInt", err)
		return false
	} else {
		now := time.Now().Unix()
		if ts < now-maxTimeStampAge || ts > now+maxTimeStampAge {
			log.Warnf("ts expired for %d origin %s", now-ts, origin)
			return false
		}
		//ip := strings.Split(r.RemoteAddr, ":")[0]
		//log.Infof("client ip %s", ip)

		hm := hmac.New(md5.New, []byte(securityToken))
		hm.Write([]byte(tsStr + id))
		realSign := hex.EncodeToString(hm.Sum(nil))[:8]
		if sign != realSign {
			log.Warnf("client token %s not match %s origin %s", sign, realSign, origin)
			return false
		}
	}
	return true
}

func commonCheck() bool {
	// cpu usage
	cpuUsage := atomic.LoadInt64(&handler.G_CPU)
	if cpuUsage > REJECT_JOIN_CPU_Threshold {
		log.Warnf("peer join reach cpu %d", cpuUsage)
		return false
	}
	if !redis.IsAlive {
		log.Warnf("peer join redis not alive")
		return false
	}

	// rate limit
	if limitEnabled {
		if limiter.TakeAvailable(1) == 0 {
			log.Warnf("reach rate limit %d", limiter.Capacity())
			return false
		}
		//else {
		//	log.Infof("rate limit remaining %d capacity %d", limiter.Available(), limiter.Capacity())
		//}
	}
	return true
}

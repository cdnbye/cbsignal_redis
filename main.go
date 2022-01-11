package main

import (
	"bytes"
	"cbsignal/client"
	"cbsignal/handler"
	"cbsignal/hub"
	"cbsignal/redis"
	"cbsignal/rpcservice"
	"cbsignal/rpcservice/broadcast"
	"cbsignal/rpcservice/signaling"
	"cbsignal/util"
	"cbsignal/util/ratelimit"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/lexkong/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"net"
	"net/http"
	"sync/atomic"

	//_ "net/http/pprof"
	"net/rpc"

	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	VERSION                   = "2.9.0"
	CHECK_CLIENT_INTERVAL     = 15 * 60
	EXPIRE_LIMIT              = 12 * 60
	REJECT_JOIN_CPU_Threshold = 800
	REJECT_MSG_CPU_Threshold  = 900
)

var (
	cfg = pflag.StringP("config", "c", "", "Config file path.")
	newline = []byte{'\n'}
	space   = []byte{' '}

	selfIp string
	selfPort string
	selfAddr string

	signalPort     string
	signalPortTLS  string
	signalCertPath string
	signalKeyPath  string

	versionNum int

	limitEnabled bool
	limitRate    int64
	limiter      *ratelimit.Bucket

	securityEnabled bool
	statsEnabled bool
	maxTimeStampAge int64
	securityToken string

	redisCli redis.RedisClient
)

func init()  {
	pflag.Parse()

	versionNum = util.GetVersionNum(VERSION)

	// Initialize viper
	if *cfg != "" {
		viper.SetConfigFile(*cfg) // 如果指定了配置文件，则解析指定的配置文件
	} else {
		viper.AddConfigPath("./") // 如果没有指定配置文件，则解析默认的配置文件
		viper.SetConfigName("config")
	}
	viper.SetConfigType("yaml")     // 设置配置文件格式为YAML
	viper.AutomaticEnv()            // 读取匹配的环境变量
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	if err := viper.ReadInConfig(); err != nil { // viper解析配置文件
		panic(err)
	}

	// Initialize logger
	passLagerCfg := log.PassLagerCfg{
		Writers:        viper.GetString("log.writers"),
		LoggerLevel:    viper.GetString("log.logger_level"),
		LoggerFile:     viper.GetString("log.logger_dir"),
		LogFormatText:  viper.GetBool("log.log_format_text"),
		RollingPolicy:  viper.GetString("log.rollingPolicy"),
		LogRotateDate:  viper.GetInt("log.log_rotate_date"),
		LogRotateSize:  viper.GetInt("log.log_rotate_size"),
		LogBackupCount: viper.GetInt("log.log_backup_count"),
	}
	if err := log.InitWithConfig(&passLagerCfg); err != nil {
		fmt.Errorf("Initialize logger %s", err)
	}

	selfIp = util.GetInternalIP()
	selfPort = viper.GetString("rpc_port")
	if selfPort == "" {
		panic("port for rpc is required")
	}
	selfAddr = fmt.Sprintf("%s:%s", selfIp, selfPort)

	// init redis client
	isRedisCluster := viper.GetBool("redis.is_cluster")
	redisAddr := fmt.Sprintf("%s:%s", viper.GetString("redis.host"), viper.GetString("redis.port"))
	redisPsw := viper.GetString("redis.password")
	redisDB := viper.GetInt("redis.dbname")
	redisCli = redis.InitRedisClient(isRedisCluster, selfAddr, redisAddr, redisPsw, redisDB)
	_, err := redisCli.Ping().Result()
	if err != nil {
		panic(err)
	}

	signalPort = viper.GetString("port")
	signalPortTLS = viper.GetString("tls.port")
	signalCertPath = viper.GetString("tls.cert")
	signalKeyPath = viper.GetString("tls.key")

	setupConfigFromViper()

	//开始监听
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Warnf("config is changed :%s \n", e.Name)
		setupConfigFromViper()
	})

	hub.Init()
	go func() {
		for {
			time.Sleep(CHECK_CLIENT_INTERVAL*time.Second)
			now := time.Now().Unix()
			log.Warnf("start check client alive...")
			count := 0
			for item := range hub.GetInstance().Clients.IterBuffered() {
				cli := item.Val
				if cli.IsExpired(now, EXPIRE_LIMIT) {
					// 节点过期
					//log.Warnf("client %s is expired for %d, close it", cli.PeerId, now-cli.Timestamp)
					if ok := hub.DoUnregister(cli.PeerId); ok {
						cli.Close()
						count ++
					}
				}
			}
			log.Warnf("check client finished, closed %d clients", count)
		}
	}()

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

	if signalPort != "" {
		go func() {
			log.Warnf("Start to listening the incoming requests on http address: %s\n", signalPort)
			err := http.ListenAndServe(":"+signalPort, nil)
			if err != nil {
				log.Fatal("ListenAndServe: ", err)
			}
		}()
	}

	if  signalPortTLS != "" && util.Exists(signalCertPath) && util.Exists(signalKeyPath) {
		go func() {
			log.Warnf("Start to listening the incoming requests on https address: %s\n", signalPortTLS)
			err := http.ListenAndServeTLS(":"+signalPortTLS, signalCertPath, signalKeyPath, nil)
			if err != nil {
				log.Fatal("ListenAndServe: ", err)
			}
		}()
	}

	rpcservice.NewNodeHub(selfAddr)
	// rpcservice
	go func() {

		log.Warnf("register rpcservice service on tcp address: %s\n", selfPort)
		listener, err := net.Listen("tcp", ":"+selfPort)
		if err != nil {
			log.Fatal("ListenTCP error:", err)
		}
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatal("Accept error:", err)
			}
			go rpc.ServeConn(conn)
		}
	}()
	time.Sleep(6*time.Second)

	// 注册rpc广播服务
	if err := broadcast.RegisterBroadcastService();err != nil {
		panic(err)
	}

	// 注册rpc信令服务
	if err := signaling.RegisterSignalService();err != nil {
		panic(err)
	}

	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/wss", wsHandler)
	http.HandleFunc("/", wsHandler)

	if statsEnabled {
		http.HandleFunc("/count", handler.CountHandler())
		http.HandleFunc("/total_count", handler.TotalCountHandler())
		http.HandleFunc("/version", handler.VersionHandler(VERSION))

		info := handler.SignalInfo{
			Version:            VERSION,
			SecurityEnabled: securityEnabled,
		}
		if limitEnabled {
			info.RateLimit = limitRate
		}
		http.HandleFunc("/info", handler.StatsHandler(info))
	}

	<-intrChan

	log.Info("Shutting down server...")

	// do cleanup
	redisCli.Close()
	rpcservice.ClearNodeHub()
}

func wsHandler(w http.ResponseWriter, r *http.Request) {

	// cpu usage
	cpuUsage := atomic.LoadInt64(&handler.G_CPU)
	if cpuUsage > REJECT_JOIN_CPU_Threshold {
		log.Warnf("peer join reach cpu %d", cpuUsage)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// rate limit
	if limitEnabled {
		if limiter.TakeAvailable(1) == 0 {
			log.Warnf("reach rate limit %d", limiter.Capacity())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		//else {
		//	log.Infof("rate limit remaining %d capacity %d", limiter.Available(), limiter.Capacity())
		//}
	}

	// Upgrade connection
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	r.ParseForm()
	id := r.Form.Get("id")
	//platform := r.Form.Get("p")
	//log.Printf("id %s", id)
	if id == "" {
		conn.Close()
		return
	}

	c := client.NewPeerClient(id, conn)

	// test
	//closeInvalidConn(c)
	//return

	// 校验
	if securityEnabled {
		token := strings.Split(r.Form.Get("token"), "-")
		if len(token) < 2 {
			log.Warnf("token not valid %s origin %s", r.Form.Get("token"), r.Header.Get("Origin"))
			closeInvalidConn(c)
			return
		}
		sign := token[0]
		tsStr := token[1]
		if ts, err := strconv.ParseInt(tsStr, 10, 64); err != nil {
			//log.Warnf("ts ParseInt", err)
			closeInvalidConn(c)
			return
		} else {
			now := time.Now().Unix()
			if ts < now - maxTimeStampAge || ts > now + maxTimeStampAge  {
				log.Warnf("ts expired for %d origin %s", now - ts, r.Header.Get("Origin"))
				closeInvalidConn(c)
				return
			}
			//ip := strings.Split(r.RemoteAddr, ":")[0]
			//log.Infof("client ip %s", ip)

			hm := hmac.New(md5.New, []byte(securityToken))
			hm.Write([]byte(tsStr + id))
			realSign := hex.EncodeToString(hm.Sum(nil))[:8]
			if sign != realSign {
				log.Warnf("client token %s not match %s origin %s", sign, realSign, r.Header.Get("Origin"))
				closeInvalidConn(c)
				return
			}
		}
	}

	hub.DoRegister(c)

	// 发送版本号
	c.SendMsgVersion(versionNum)

	go func() {
		defer func() {
			// 节点离开
			log.Infof("peer leave")
			if ok := hub.DoUnregister(id); ok {
				//log.Infof("close peer %s", id)
				closeInvalidConn(c)
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
						conn.Close()    // TODO 验证
						return
					}
					c.UpdateTs()
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

				data := bytes.TrimSpace(bytes.Replace(m.Payload, newline, space, -1))
				hdr, err := handler.NewHandler(data, c)
				if err != nil {
					// 心跳包
					//c.UpdateTs()
					log.Error("NewHandler", err)
				} else {
					hdr.Handle()
				}
			}
		}
	}()
}

func closeInvalidConn(cli *client.Client)  {
	cli.SendMsgClose("invalid client")
	cli.Close()
}

func setupConfigFromViper()  {
	limitEnabled = viper.GetBool("ratelimit.enable")
	limitRate = viper.GetInt64("ratelimit.max_rate")
	securityEnabled = viper.GetBool("security.enable")
	maxTimeStampAge = viper.GetInt64("security.maxTimeStampAge")
	securityToken = viper.GetString("security.token")
	statsEnabled = viper.GetBool("stats.enable")
	handler.StatsToken = viper.GetString("stats.token")
}



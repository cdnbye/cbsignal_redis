
### build
Make sure that the golang development environment is installed
```bash
git clone https://github.com/cdnbye/cbsignal_redis.git
cd cbsignal
make
```
or directly use compiled linux file [cbsignal](https://github.com/cdnbye/cbsignal_redis/releases) .

### deploy
Make sure you have setup redis server, then edit `config.yaml`:
```yaml
redis:
  host: REDIS_IP
  port: 6379
  dbname: 0
  is_cluster: false
```
Upload binary file, admin.sh and config.yaml to server, create `cert` directory with `signaler.pem` and `signaler.key`, then start service:
```bash
chmod +x admin.sh
sudo ./admin.sh start
```

### test
```
import Hls from 'cdnbye';
var hlsjsConfig = {
    p2pConfig: {
        wsSignalerAddr: 'ws://YOUR_SIGNAL',
        // Other p2pConfig options provided by hlsjs-p2p-engine
    }
};
// Hls constructor is overriden by included bundle
var hls = new Hls(hlsjsConfig);
// Use `hls` just like the usual hls.js ...
```

### Get real-time information of signal service
```
GET /info
```
Response:
```
Status: 200

{
  "ret": 0,
  "data": {
      "version"
      "current_connections"
      "cluster_mode"
      "num_goroutine"
      "num_per_map"
  }
}
```

### Cluster Mode
RPC is used to communicate between all nodes. Specify RPC port in `config.yaml`, then start service:
```bash
sudo ./admin.sh start cluster config.yaml
``` 

## Run by Docker
The default redis address in default config is 127.0.0.1:6379
```sh
sudo docker run --net host --restart=unless-stopped -d cdnbye/cbsignal_redis:latest
```

## Run by Docker with your own Config
```sh
mkdir config && cd config
touch config.yaml
```
Then cony your config to config.yaml, you can copy your SSL cert to this directory, then:
```sh
sudo docker run --net host --restart=unless-stopped -d -v "$(pwd)"/config:/cbsignal_redis/config  cdnbye/cbsignal_redis:latest
```

## Related projects
* [cbsignal_node](https://github.com/cdnbye/cbsignal_node) - High performance CDNBye signaling service written in node.js

### go语言版的 CDNBye 信令服务器，可用于Web、安卓、iOS SDK等所有CDNBye产品
#### 编译二进制文件
请先确保已安装golang开发环境
```bash
git clone https://github.com/cdnbye/cbsignal_redis.git
cd cbsignal
make
```
或者直接使用已经编译好的linux可执行文件 [cbsignal](https://github.com/cdnbye/cbsignal_redis/releases)

#### 部署
首先确保内网中已经有redis服务, 编辑 `config.yaml`:
```yaml
redis:
  host: REDIS_IP
  port: 6379
  dbname: 0
  is_cluster: false
```
将编译生成的二进制文件、admin.sh和config.yaml上传至服务器，并在同级目录创建`cert`文件夹，将证书和秘钥文件分别改名为`signaler.pem`和`signaler.key`放入cert，之后启动服务：
```bash
chmod +x admin.sh
echo -17 > /proc/$(pidof cbsignal)/oom_adj     # 防止进程被OOM killer杀死
sudo ./admin.sh start
```

### 测试
```
import Hls from 'cdnbye';
var hlsjsConfig = {
    p2pConfig: {
        wsSignalerAddr: 'ws://YOUR_SIGNAL',
        // Other p2pConfig options provided by hlsjs-p2p-engine
    }
};
// Hls constructor is overriden by included bundle
var hls = new Hls(hlsjsConfig);
// Use `hls` just like the usual hls.js ...
```

### 通过API获取信令服务的实时信息
```
GET /info
```
响应:
```
Status: 200

{
  "ret": 0,
  "data": {
      "version"
      "current_connections"
      "cluster_mode"
      "num_goroutine"
      "num_per_map"
  }
}
```

### 集群模式
节点之间采用RPC进行通信，首先在 `config_cluster.yaml` 中指定 RPC 端口, 然后启动服务：
```bash
sudo ./admin.sh start cluster config_cluster.yaml
``` 

## 通过Docker部署
默认配置连接redis地址是127.0.0.1:6379
```sh
sudo docker run --net host --restart=unless-stopped -d cdnbye/cbsignal_redis:latest
```

## 通过Docker部署并自定义配置
```sh
mkdir config && cd config
touch config.yaml
```
config.yaml是你的自定义配置，可以在此配置证书文件等，然后运行：
```sh
sudo docker run --net host --restart=unless-stopped -d -v "$(pwd)"/config:/cbsignal_redis/config  cdnbye/cbsignal_redis:latest
```

## 相关项目
* [cbsignal_node](https://github.com/cdnbye/cbsignal_node) - 基于node.js开发的高性能CDNBye信令服务





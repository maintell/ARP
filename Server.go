package main

import (
	"bytes"
	"context"
	"fmt"
	redigo "github.com/gomodule/redigo/redis"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
	"time"
)

var transport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second, //连接超时
		KeepAlive: 30 * time.Second, //长连接超时时间
	}).DialContext,
	MaxIdleConns:          100,              //最大空闲连接
	IdleConnTimeout:       90 * time.Second, //空闲超时时间
	TLSHandshakeTimeout:   10 * time.Second, //tls握手超时时间
	ExpectContinueTimeout: 1 * time.Second,  //100-continue状态码超时时间
}

type Server interface {
	Runnable
}

type Instance struct {
	access        sync.Mutex
	running       bool
	poolList      []*redigo.Pool
	OriginalRedis *redigo.Pool
	OriginMap     map[string]Originservers
	ctx           context.Context
}

func New(config *Config) (*Instance, error) {
	var server = &Instance{ctx: context.Background()}
	err, done := initInstanceWithConfig(config, server)
	if done {
		return nil, err
	}

	return server, nil
}

func NewMultipleHostsReverseProxy(matchOrigin *map[string]Originservers) *httputil.ReverseProxy {
	//请求协调者
	director := func(req *http.Request) {
		tenantid := req.Header.Get("tenantid")
		if len(tenantid) == 0 {
			return
		}
		matches := *matchOrigin
		matchOriginServer := matches[tenantid]
		req.URL.Scheme = matchOriginServer.Schme
		req.URL.Host = matchOriginServer.Host + ":" + strconv.Itoa(matchOriginServer.Port)
		//只在第一代理中设置此header头
		req.Header.Set("X-Real-Ip", req.RemoteAddr)
		log.Println(tenantid, "|", req.URL.Path, "->", matchOriginServer.Host+":"+strconv.Itoa(matchOriginServer.Port))
	}
	//更改内容
	modifyFunc := func(resp *http.Response) error {
		if resp.StatusCode != 200 {
			//获取内容
			oldPayload, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			//追加内容
			newPayload := []byte("StatusCode error:" + string(oldPayload))
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(newPayload))
			resp.ContentLength = int64(len(newPayload))
			resp.Header.Set("Content-Length", strconv.FormatInt(int64(len(newPayload)), 10))
		}
		return nil
	}
	errFunc := func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "{\"code\":\"500\",\"reason\":\""+err.Error()+"\",\"message\":\"upstream Error\"\\\"\"}", 500)
	}
	return &httputil.ReverseProxy{Director: director, Transport: transport, ModifyResponse: modifyFunc, ErrorHandler: errFunc}
}

func initInstanceWithConfig(config *Config, server *Instance) (error, bool) {
	// register patter for redis tunnel
	for _, redisServer := range config.Redisservers {
		//server.poolList
		redisPool := RedisPoolInitialization(
			redisServer.IP+":"+strconv.Itoa(redisServer.Port),
			redisServer.Auth,
			redisServer.Db)
		// store the list in instance for close
		server.poolList = append(server.poolList, redisPool)
		for _, pattern := range redisServer.Pattern {
			log.Println("Intercepted", pattern.Path)
			http.HandleFunc(pattern.Path, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("access-control-allow-origin", "*")
				w.Header().Set("access-control-allow-methods", "POST, GET, OPTIONS, PUT, DELETE")
				w.Header().Set("access-control-allow-headers", "*")
				w.Header().Set("Access-Control-Expose-Headers", "*")
				w.Header().Set("access-control-max-age", "7200")
				if r.Method == "OPTIONS" {
					//w.Header().Set("access-control-allow-origin", "*")
					//w.Header().Set("access-control-allow-methods", "POST, GET, OPTIONS, PUT, DELETE")
					//w.Header().Set("access-control-allow-headers", "*")
					//w.Header().Set("Access-Control-Expose-Headers", "*")
					return
				}
				//handler configured api request
				connections := redisPool.Get()
				defer connections.Close()
				headerContent := r.Header.Get(pattern.Headermatch)
				getContent, err := redigo.String(connections.Do("GET", pattern.Keypattern+headerContent))
				if err != nil {
					//没找到数据,这个时候需要向真正的源请求
					tenantid := r.Header.Get("tenantid")
					if len(tenantid) == 0 {
						return
					}
					matchOriginServer := server.OriginMap[tenantid]
					r.URL.Scheme = matchOriginServer.Schme
					r.URL.Host = matchOriginServer.Host + ":" + strconv.Itoa(matchOriginServer.Port)
					// 浅拷贝一个request 对象，避免后续修影响了源对象
					transport := http.DefaultTransport
					backEndRequest := new(http.Request)
					*backEndRequest = *r
					// 构造新请求
					response, err := transport.RoundTrip(backEndRequest)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					// 获取响应数据并返回
					for k, v := range response.Header {
						for _, v1 := range v {
							w.Header().Add(k, v1)
						}
					}
					w.WriteHeader(response.StatusCode)
					io.Copy(w, response.Body)
					response.Body.Close()
				} else {
					w.Header().Set("Content-Type", ": application/json;charset=UTF-8")
					var sb strings.Builder
					sb.WriteString(pattern.Returnprefix)
					sb.WriteString(getContent)
					sb.WriteString(pattern.Returnsuffix)
					fmt.Fprintf(w, sb.String())
					log.Println("Redis ", r.URL.Path, "->", len(sb.String()))
				}
			})
		}
	}
	//开始预处理回源服务器地址
	server.OriginMap = make(map[string]Originservers)
	go func() {
		server.OriginalRedis = RedisPoolInitialization(
			config.Originredis.IP+":"+strconv.Itoa(config.Originredis.Port),
			config.Originredis.Auth,
			config.Originredis.Db)
		for {
			conn := server.OriginalRedis.Get()
			KeyArray, valueErr := redigo.Strings(conn.Do("KEYS", config.Originredis.Keypattern+"*"))
			if valueErr != nil {
				fmt.Println(valueErr.Error())
			}
			for _, key := range KeyArray {
				getContent, err := redigo.String(conn.Do("GET", key))
				if err != nil {
					fmt.Println(err.Error())
				} else {
					MatchKey := strings.Replace(key, config.Originredis.Keypattern, "", -1)
					serverSplited := strings.Split(getContent, ":")
					port, _ := strconv.Atoi(serverSplited[1])
					serverx := Originservers{
						Schme:       "http",
						Host:        serverSplited[0],
						Port:        port,
						Headermatch: "tenantid",
					}
					server.OriginMap[MatchKey] = serverx
				}
			}
			time.Sleep(30 * time.Second)
		}
	}()

	for _, originserver := range config.Originservers {
		for _, matchString := range originserver.Matchlist {
			server.OriginMap[matchString] = originserver
		}
	}
	proxy := NewMultipleHostsReverseProxy(&server.OriginMap)

	var ListenUrl = config.Serverconfig.Listen + ":" + strconv.Itoa(config.Serverconfig.Port)

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "OPTIONS" {
			writer.Header().Set("access-control-allow-origin", "*")
			writer.Header().Set("access-control-allow-methods", "POST, GET, OPTIONS, PUT, DELETE")
			writer.Header().Set("access-control-allow-headers", "*")
			return
		}
		proxy.ServeHTTP(writer, request)
	})
	fmt.Println("Server Start :", ListenUrl)
	err := http.ListenAndServe(ListenUrl, nil)
	if err != nil {
		newError("ARP ", Version(), " start failed", err).AtWarning().WriteToLog()
		return err, false
	} else {
		newError("ARP ", Version(), " started at", ListenUrl).AtWarning().WriteToLog()
	}
	return nil, false
}

// Close shutdown the ARP instance.
func (s *Instance) Close() error {
	s.access.Lock()
	defer s.access.Unlock()
	s.running = false
	for _, pool := range s.poolList {
		pool.Close()
	}
	s.OriginalRedis.Close()
	return nil
}

func (s *Instance) Start() error {
	s.access.Lock()
	defer s.access.Unlock()
	s.running = true
	newError("ARP ", Version(), " started").AtWarning().WriteToLog()

	return nil
}

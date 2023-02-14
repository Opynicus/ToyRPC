/*
 * @Author: Opynicus
 * @Date: 2023-02-14 13:36:08
 * @LastEditTime: 2023-02-14 14:36:43
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\registry\registry.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultPath    = "/_toyrpc_/registry"
	defaultTimeout = time.Minute * 5
)

type ServerItem struct {
	addr  string
	start time.Time
}

type Registry struct {
	timeout time.Duration
	mtx     sync.Mutex
	servers map[string]*ServerItem
}

func NewRegistry(timeout time.Duration) *Registry {
	return &Registry{
		servers: make(map[string]*ServerItem),
		timeout: timeout,
	}
}

var DefaultRPCRegister = NewRegistry(defaultTimeout)

// putServer puts a new server or updates an existing server's start time.
func (r *Registry) putServer(addr string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{addr: addr, start: time.Now()}
	} else {
		s.start = time.Now() // if exists, update start time to keep alive
	}
}

func (r *Registry) aliveServers() []string {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.servers, addr) // delete expired servers
		}
	}
	sort.Strings(alive)
	return alive
}

func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		// keep it simple, server is in req.Header
		w.Header().Set("X-ToyRPC-Servers", strings.Join(r.aliveServers(), ","))
	case "POST":
		// keep it simple, server is in req.Header
		addr := req.Header.Get("X-ToyRPC-Server")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Registry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r)
	log.Println("rpc registry path:", registryPath)
}

func HandleHTTP() {
	DefaultRPCRegister.HandleHTTP(defaultPath)
}

func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heart beat to registry", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-ToyRPC-Server", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heart beat err:", err)
		return err
	}
	return nil
}

func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 { // if duration is 0, use default duration
		duration = defaultTimeout - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr)
	go func() { // send heartbeat periodically
		t := time.NewTicker(duration) // send heartbeat every duration
		for err == nil {              // if err != nil, stop sending heartbeat
			<-t.C // wait for next tick
			err = sendHeartbeat(registry, addr)
		}
	}()
}

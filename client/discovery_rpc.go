/*
 * @Author: Opynicus
 * @Date: 2023-02-14 14:39:40
 * @LastEditTime: 2023-02-14 15:59:22
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\client\discovery_rpc.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package client

import (
	"log"
	"net/http"
	"strings"
	"time"
)

const defaultUpdateTimeout = time.Second * 10

type RegistryDiscovery struct {
	*MultiServersDiscovery
	registry   string
	timeout    time.Duration
	lastUpdate time.Time
}

func NewRegistryDiscovery(registryAddr string, timeout time.Duration) *RegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	return &RegistryDiscovery{
		MultiServersDiscovery: NewMultiServerDiscovery(make([]string, 0)),
		registry:              registryAddr,
		timeout:               timeout,
	}
}

func (rd *RegistryDiscovery) Update(servers []string) error {
	rd.mtx.Lock()
	defer rd.mtx.Unlock()
	rd.servers = servers
	rd.lastUpdate = time.Now()
	return nil
}

func (rd *RegistryDiscovery) Refresh() error {
	rd.mtx.Lock()
	defer rd.mtx.Unlock()
	if rd.lastUpdate.Add(rd.timeout).After(time.Now()) { // if last update time is not timeout, return nil
		return nil
	}
	log.Println("rpc registry: refresh servers from registry", rd.registry)
	resp, err := http.Get(rd.registry)
	if err != nil {
		log.Println("rpc registry refresh err:", err)
		return err
	}
	servers := strings.Split(resp.Header.Get("X-ToyRPC-Servers"), ",")
	rd.servers = make([]string, 0, len(servers))
	for _, server := range servers {
		if strings.TrimSpace(server) != "" { // remove empty string
			rd.servers = append(rd.servers, strings.TrimSpace(server))
		}
	}
	rd.lastUpdate = time.Now()
	return nil
}

func (rd *RegistryDiscovery) Get(mode int) (string, error) {
	if err := rd.Refresh(); err != nil {
		return "", err
	}
	return rd.MultiServersDiscovery.Get(mode)
}

func (rd *RegistryDiscovery) GetAll() ([]string, error) {
	if err := rd.Refresh(); err != nil {
		return nil, err
	}
	return rd.MultiServersDiscovery.GetAll()
}

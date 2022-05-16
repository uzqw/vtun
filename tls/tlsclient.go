package tls

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"time"

	"github.com/net-byte/vtun/common/cache"
	"github.com/net-byte/vtun/common/cipher"
	"github.com/net-byte/vtun/common/config"
	"github.com/net-byte/vtun/tun"
	"github.com/songgao/water"
)

// Start tls client
func StartClient(config config.Config) {
	log.Printf("vtun tls client started on %v", config.LocalAddr)
	iface := tun.CreateTun(config)
	go tunToTLS(config, iface)
	for {
		tlsconfig := &tls.Config{
			//InsecureSkipVerify: true,
		}
		if config.TLSSni != "" {
			tlsconfig.ServerName = config.TLSSni
		}
		conn, err := tls.Dial("tcp", config.ServerAddr, tlsconfig)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		cache.GetCache().Set("tlsconn", conn, 24*time.Hour)
		tlsToTun(config, conn, iface)
		cache.GetCache().Delete("tlsconn")
	}
}

func tunToTLS(config config.Config, iface *water.Interface) {
	packet := make([]byte, config.MTU)
	for {
		n, err := iface.Read(packet)
		if err != nil || n == 0 {
			continue
		}
		if v, ok := cache.GetCache().Get("tlsconn"); ok {
			b := packet[:n]
			if config.Obfs {
				packet = cipher.XOR(packet)
			}
			tlsconn := v.(net.Conn)
			tlsconn.SetWriteDeadline(time.Now().Add(time.Duration(config.Timeout) * time.Second))
			_, err = tlsconn.Write(b)
			if err != nil {
				continue
			}
		}
	}
}

func tlsToTun(config config.Config, tlsconn net.Conn, iface *water.Interface) {
	defer tlsconn.Close()
	packet := make([]byte, config.MTU)
	for {
		tlsconn.SetReadDeadline(time.Now().Add(time.Duration(config.Timeout) * time.Second))
		n, err := tlsconn.Read(packet)
		if err != nil || err == io.EOF {
			break
		}
		b := packet[:n]
		if config.Obfs {
			b = cipher.XOR(b)
		}
		_, err = iface.Write(b)
		if err != nil {
			break
		}
	}
}
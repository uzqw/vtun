package tun

import (
	"log"
	"net"
	"runtime"
	"strconv"

	"github.com/net-byte/vtun/common/config"
	"github.com/net-byte/vtun/common/netutil"
	"github.com/net-byte/water"
)

// CreateTun creates a tun interface
func CreateTun(config config.Config) (iFace *water.Interface) {
	c := water.Config{DeviceType: water.TUN}
	c.PlatformSpecificParams = water.PlatformSpecificParams{}
	os := runtime.GOOS
	if os == "windows" {
		c.PlatformSpecificParams.Name = "vtun"
		c.PlatformSpecificParams.Network = []string{config.CIDR, config.CIDRv6}
	}
	if config.DeviceName != "" {
		c.PlatformSpecificParams.Name = config.DeviceName
	}
	iFace, err := water.New(c)
	if err != nil {
		log.Fatalln("failed to create tun interface:", err)
	}
	log.Printf("interface created %v", iFace.Name())
	setRoute(config, iFace)
	return iFace
}

// setRoute sets the system routes
func setRoute(config config.Config, iFace *water.Interface) {
	ip, _, err := net.ParseCIDR(config.CIDR)
	if err != nil {
		log.Panicf("error cidr %v", config.CIDR)
	}
	ipv6, _, err := net.ParseCIDR(config.CIDRv6)
	if err != nil {
		log.Panicf("error ipv6 cidr %v", config.CIDRv6)
	}
	os := runtime.GOOS
	if os == "linux" {
		netutil.ExecCmd("/sbin/ip", "link", "set", "dev", iFace.Name(), "mtu", strconv.Itoa(config.MTU))
		netutil.ExecCmd("/sbin/ip", "addr", "add", config.CIDR, "dev", iFace.Name())
		netutil.ExecCmd("/sbin/ip", "-6", "addr", "add", config.CIDRv6, "dev", iFace.Name())
		netutil.ExecCmd("/sbin/ip", "link", "set", "dev", iFace.Name(), "up")
		if !config.ServerMode && config.GlobalMode {
			physicaliFace := netutil.GetInterface()
			serverAddrIP := netutil.LookupServerAddrIP(config.ServerAddr)
			if physicaliFace != "" && serverAddrIP != nil {
				if config.LocalGateway != "" {
					netutil.ExecCmd("/sbin/ip", "route", "add", "0.0.0.0/1", "dev", iFace.Name())
					netutil.ExecCmd("/sbin/ip", "route", "add", "128.0.0.0/1", "dev", iFace.Name())
					if serverAddrIP.To4() != nil {
						netutil.ExecCmd("/sbin/ip", "route", "add", serverAddrIP.To4().String()+"/32", "via", config.LocalGateway, "dev", physicaliFace)
					}
				}
				if config.LocalGatewayv6 != "" {
					netutil.ExecCmd("/sbin/ip", "-6", "route", "add", "::/1", "dev", iFace.Name())
					if serverAddrIP.To16() != nil {
						netutil.ExecCmd("/sbin/ip", "-6", "route", "add", serverAddrIP.To16().String()+"/128", "via", config.LocalGatewayv6, "dev", physicaliFace)
					}
				}
			}
		}
	} else if os == "darwin" {
		netutil.ExecCmd("ifconfig", iFace.Name(), "inet", ip.String(), config.ServerIP, "up")
		netutil.ExecCmd("ifconfig", iFace.Name(), "inet6", ipv6.String(), config.ServerIPv6, "up")
		if !config.ServerMode && config.GlobalMode {
			physicaliFace := netutil.GetInterface()
			serverAddrIP := netutil.LookupServerAddrIP(config.ServerAddr)
			if physicaliFace != "" && serverAddrIP != nil {
				if config.LocalGateway != "" {
					netutil.ExecCmd("route", "add", "default", config.ServerIP)
					netutil.ExecCmd("route", "change", "default", config.ServerIP)
					netutil.ExecCmd("route", "add", "0.0.0.0/1", "-interface", iFace.Name())
					netutil.ExecCmd("route", "add", "128.0.0.0/1", "-interface", iFace.Name())
					if serverAddrIP.To4() != nil {
						netutil.ExecCmd("route", "add", serverAddrIP.To4().String(), config.LocalGateway)
					}
				}
				if config.LocalGatewayv6 != "" {
					netutil.ExecCmd("route", "add", "-inet6", "default", config.ServerIPv6)
					netutil.ExecCmd("route", "change", "-inet6", "default", config.ServerIPv6)
					netutil.ExecCmd("route", "add", "-inet6", "::/1", "-interface", iFace.Name())
					if serverAddrIP.To16() != nil {
						netutil.ExecCmd("route", "add", "-inet6", serverAddrIP.To16().String(), config.LocalGatewayv6)
					}
				}
			}
		}
	} else if os == "windows" {
		if !config.ServerMode && config.GlobalMode {
			serverAddrIP := netutil.LookupServerAddrIP(config.ServerAddr)
			if serverAddrIP != nil {
				if config.LocalGateway != "" {
					netutil.ExecCmd("cmd", "/C", "route", "delete", "0.0.0.0", "mask", "0.0.0.0")
					netutil.ExecCmd("cmd", "/C", "route", "add", "0.0.0.0", "mask", "0.0.0.0", config.ServerIP, "metric", "6")
					if serverAddrIP.To4() != nil {
						netutil.ExecCmd("cmd", "/C", "route", "add", serverAddrIP.To4().String()+"/32", config.LocalGateway, "metric", "5")
					}
				}
				if config.LocalGatewayv6 != "" {
					netutil.ExecCmd("cmd", "/C", "route", "-6", "delete", "::/0", "mask", "::/0")
					netutil.ExecCmd("cmd", "/C", "route", "-6", "add", "::/0", "mask", "::/0", config.ServerIPv6, "metric", "6")
					if serverAddrIP.To16() != nil {
						netutil.ExecCmd("cmd", "/C", "route", "-6", "add", serverAddrIP.To16().String()+"/128", config.LocalGatewayv6, "metric", "5")
					}
				}
			}
		}
	} else {
		log.Printf("not support os %v", os)
	}
	log.Printf("interface configured %v", iFace.Name())
}

// ResetRoute resets the system routes
func ResetRoute(config config.Config) {
	if config.ServerMode || !config.GlobalMode {
		return
	}
	os := runtime.GOOS
	if os == "darwin" {
		if config.LocalGateway != "" {
			netutil.ExecCmd("route", "add", "default", config.LocalGateway)
			netutil.ExecCmd("route", "change", "default", config.LocalGateway)
		}
		if config.LocalGatewayv6 != "" {
			netutil.ExecCmd("route", "add", "-inet6", "default", config.LocalGatewayv6)
			netutil.ExecCmd("route", "change", "-inet6", "default", config.LocalGatewayv6)
		}
	} else if os == "windows" {
		serverAddrIP := netutil.LookupServerAddrIP(config.ServerAddr)
		if serverAddrIP != nil {
			if config.LocalGateway != "" {
				netutil.ExecCmd("cmd", "/C", "route", "delete", "0.0.0.0", "mask", "0.0.0.0")
				netutil.ExecCmd("cmd", "/C", "route", "add", "0.0.0.0", "mask", "0.0.0.0", config.LocalGateway, "metric", "6")
			}
			if config.LocalGatewayv6 != "" {
				netutil.ExecCmd("cmd", "/C", "route", "-6", "delete", "::/0", "mask", "::/0")
				netutil.ExecCmd("cmd", "/C", "route", "-6", "add", "::/0", "mask", "::/0", config.LocalGatewayv6, "metric", "6")
			}
		}
	}
}

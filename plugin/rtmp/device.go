package plugin_rtmp

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"m7s.live/v5"
)

type RTMPDevice struct {
	m7s.DeviceTask
	tcpAddr *net.TCPAddr
	url     *url.URL
}

func (d *RTMPDevice) Start() (err error) {
	d.url, err = url.Parse(d.Device.URL)
	if err != nil {
		return
	}
	if ips, err := net.LookupIP(d.url.Hostname()); err != nil {
		return err
	} else if len(ips) == 0 {
		return fmt.Errorf("no IP found for host: %s", d.url.Hostname())
	} else {
		d.tcpAddr, err = net.ResolveTCPAddr("tcp", net.JoinHostPort(ips[0].String(), d.url.Port()))
		if err != nil {
			return err
		}
		if d.tcpAddr.Port == 0 {
			if d.url.Scheme == "rtmps" {
				d.tcpAddr.Port = 443
			} else {
				d.tcpAddr.Port = 1935
			}
		}
	}
	return d.DeviceTask.Start()
}

func (d *RTMPDevice) GetTickInterval() time.Duration {
	return time.Second * 5
}

func (d *RTMPDevice) Tick(any) {
	startTime := time.Now()
	conn, err := net.DialTCP("tcp", nil, d.tcpAddr)
	if err != nil {
		d.Device.ChangeStatus(m7s.DeviceStatusOffline)
		return
	}
	conn.Close()
	d.Device.RTT = time.Since(startTime)
	d.Device.ChangeStatus(m7s.DeviceStatusOnline)
}

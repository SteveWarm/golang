package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/SteveWarm/golang/logger"
	"github.com/rakyll/globalconf"
)

var (
	g_listen       *string = flag.String("listen", "", "")
	g_http         *string = flag.String("http", "", "")
	g_https        *string = flag.String("https", "", "")
	g_buff_size    *int    = flag.Int("buff", 1000, "")
	g_idle_timeout *int    = flag.Int("idle_timeout", 80, "")
	g_dial_timeout *int    = flag.Int("dial_timeout", 3, "")
	g_keep_alive   *int    = flag.Int("keep_alive", 280, "tcp keep alive second")

	g_log_path    = flag.String("log_path", "/tmp/", "")
	g_log_name    = flag.String("log_name", "httptunnel.log", "")
	g_log_level   = flag.Int("log_level", 2, "")
	g_log_console = flag.Bool("log_console", false, "")
)

var (
	POST   = []byte("post")
	GET    = []byte("get ")
	PUT    = []byte("put ")
	DELETE = []byte("dele")
)

func main() {
	confpath := "tunnel.conf"
	if len(os.Args) == 2 {
		confpath = os.Args[1]
	}

	conf, err := globalconf.NewWithOptions(&globalconf.Options{
		Filename: confpath,
	})

	if err != nil {
		fmt.Fprintln(os.Stderr, "load config faild! path:", confpath, "err:", err)
		os.Exit(1)
	}

	conf.Parse()

	if g_listen == nil || g_http == nil || g_https == nil {
		os.Exit(1)
		return
	}

	logger.SetConsole(*g_log_console)
	logger.SetRollingDaily(*g_log_path, *g_log_name)
	logger.SetLevel(logger.LEVEL(*g_log_level))

	addr := *g_listen
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen at", addr, "faild!", err)
		os.Exit(1)
		return
	}

	var g_conn_id = uint64(1)

	for {
		conn, err := l.Accept()
		if err != nil {
			logger.Error("accept err:", err)
			if conn != nil {
				conn.Close()
			}
			continue
		}

		go handleConn(atomic.AddUint64(&g_conn_id, 1), conn)
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	logger.Info("server stop signal:", <-ch)
}

var g_conn_id = uint64(1)

func handleConn(connid uint64, aConn net.Conn) {
	defer aConn.Close()
	aStr := fmt.Sprintf(
		"a %s - %s",
		aConn.RemoteAddr().String(),
		aConn.LocalAddr().String())
	logger.Info(connid, aStr, "come")

	setTcpProperties(aConn)

	buf := make([]byte, 4)
	aConn.SetReadDeadline(time.Now().Add(4 * time.Second))
	if n, err := io.ReadFull(aConn, buf); err != nil || n != 4 {
		logger.Warn(connid, "n:", n, "err:", err)
		return
	}

	var addr string
	var method []byte = bytes.ToLower(buf)
	if bytes.Compare(method, POST) == 0 || bytes.Compare(method, GET) == 0 || bytes.Compare(method, DELETE) == 0 || bytes.Compare(method, PUT) == 0 {
		// http
		addr = *g_http
	} else {
		// https
		addr = *g_https
	}

	bConn, err := net.DialTimeout("tcp", addr, time.Duration(*g_dial_timeout)*time.Second)
	if err != nil {
		logger.Error(connid, "dial ", addr, "err:", err)
		return
	}
	defer bConn.Close()

	bStr := fmt.Sprintf(
		"b %s - %s",
		bConn.LocalAddr().String(),
		bConn.RemoteAddr().String())

	logger.Info(connid, aStr, "<->", bStr)
	setTcpProperties(bConn)

	n, err := bConn.Write(buf)
	if err != nil {
		logger.Error("write to b err:", err, "n:", n)
		return
	}

	aConn.SetDeadline(time.Now().Add(999 * time.Hour))
	bConn.SetDeadline(time.Now().Add(999 * time.Hour))

	stopCh := make(chan uint64, 2)
	reportCh := make(chan int, 100)
	go pipe(connid, stopCh, reportCh, aConn, bConn)
	go pipe(connid, stopCh, reportCh, bConn, aConn)
	idelTime := time.Duration(*g_idle_timeout) * time.Second
	timer := time.NewTimer(idelTime)

	pkgcount := uint64(0)
	pkgsize := uint64(0)
	defer func() {
		logger.Info(connid, "addr:", addr, "pkgcount:", pkgcount, "pkgsize:", pkgsize)
	}()

	for {
		select {
		case <-stopCh:
			logger.Info(connid, "broken release begin")
			//关闭远程连接和本地连接
			bConn.Close()
			aConn.Close()
			<-stopCh
			close(stopCh)
			logger.Info(connid, "broken release done")
			return
		case n := <-reportCh:
			timer.Reset(idelTime)
			pkgsize += uint64(n)
			pkgcount += 1
			if pkgcount%10 == 0 {
				logger.Info(connid, "addr:", addr, "pkgcount:", pkgcount, "pkgsize:", pkgsize)
			}
		case <-timer.C:
			logger.Info(connid, "idel timeout release begin")
			bConn.Close()
			aConn.Close()
			<-stopCh
			<-stopCh
			logger.Info(connid, "idel timeout release done")
			return
		}
	}
}

func pipe(id uint64, stopCh chan uint64, reportCh chan int, reader net.Conn, writer net.Conn) {
	defer func() {
		stopCh <- id
	}()

	var buff []byte = make([]byte, *g_buff_size)
	var n1 int
	var n2 int
	var err1 error
	var err2 error

	for {
		n1, err1 = reader.Read(buff)
		if err1 == nil {
			n2, err2 = writer.Write(buff[:n1])
			if err2 != nil {
				if err2 == io.EOF {
					logger.Info(id, "b closed")
				} else {
					logger.Info(id, "write faild!", err2.Error())
				}
				return
			}

			if n2 != n1 {
				// 小概率吧
				logger.Error(id, "[BUG] n1", n1, "!= n2", n2)
				return
			}

			if reportCh != nil {
				reportCh <- n1
			}
		} else {
			if err1 == io.EOF {
				logger.Info(id, "a closed")
			} else {
				logger.Info(id, "read faild!", err1.Error())
			}
			return
		}
	}
}

func setTcpProperties(conn net.Conn) {
	c, ok := conn.(*net.TCPConn)
	if ok {
		c.SetKeepAlive(true)
		c.SetKeepAlivePeriod(time.Duration(*g_keep_alive) * time.Second)
	}
}

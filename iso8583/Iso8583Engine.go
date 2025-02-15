/*
 * Copyright (c) 2019. Author Randy Ardiansyah <Detwentyfive@gmail.com>
 */

package iso8583

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/randyardiansyah25/go-iso8583/logger"
	"github.com/randyardiansyah25/libpkg/net/tcp"
	strutils "github.com/randyardiansyah25/libpkg/util/str"
)

type TcpHandler func(iso ISO8583Object)

var defaultHandler TcpHandler

func GetEngine(readerTimeout int, fieldNumberKey ...int) *TCPIso8583Engine {
	return &TCPIso8583Engine{
		Timeout:         readerTimeout,
		FieldNumber:     fieldNumberKey,
		tcpHandlerGroup: make(map[string]TcpHandler),
	}
}

type TCPIso8583Engine struct {
	FieldNumber     []int
	Timeout         int
	tcpHandlerGroup map[string]TcpHandler
}

func (t *TCPIso8583Engine) RunInBackground(port string) error {
	return t.listen(port, true)
}

func (t *TCPIso8583Engine) Run(port string) error {
	return t.listen(port, false)
}

func (t *TCPIso8583Engine) listen(port string, doInBackground bool) (err error) {
	listener, err := net.Listen("tcp", fmt.Sprint(":", port))
	if err != nil {
		return err
	}

	go logger.Watcher()

	if doInBackground {
		go acceptConnection(listener, t.tcpHandlerGroup, t.Timeout, t.FieldNumber)
	} else {
		acceptConnection(listener, t.tcpHandlerGroup, t.Timeout, t.FieldNumber)
	}
	return
}

func (t *TCPIso8583Engine) AddHandler(handler TcpHandler, key ...string) {
	t.tcpHandlerGroup[strings.Join(key, "")] = handler
}

func (t *TCPIso8583Engine) AddDefaultHandler(handler TcpHandler) {
	defaultHandler = handler
}

func acceptConnection(listener net.Listener, handlerChain map[string]TcpHandler, timeout int, fieldNumber []int) {
	for {
		c, err := listener.Accept()
		if err != nil {
			//_ = glg.Error("New client rejected by : ", err.Error())
			logger.Error("New client rejected by : ", err.Error())
			continue
		}
		to := time.Duration(time.Duration(timeout) * time.Second)
		_ = c.SetReadDeadline(time.Now().Add(to))
		go handler(c, handlerChain, fieldNumber)
	}
}

func handler(c net.Conn, handlerChain map[string]TcpHandler, fieldNumber []int) {
	defer func() {
		_ = c.Close()
	}()
	message, err := tcp.BasicIOHandlerReader(c)
	if err != nil {
		//_ = glg.Error("read error : ", err.Error())
		logger.Error("read error : ", err.Error())
		return
	}

	iso, err := NewISO8583()
	if err != nil {
		//_ = glg.Error("ISO 8583 parser error : ", err.Error())
		logger.Error("ISO 8583 parser error : ", err.Error())
		return
	}
	err = iso.Parse(message)
	if err != nil {
		//_ = glg.Error("ISO 8583 parser error : ", err.Error())
		logger.Error("ISO 8583 parser error : ", err.Error())
		return
	}

	var fieldValues []string
	for _, field := range fieldNumber {
		fieldVal := iso.GetField(field)
		fieldValues = append(fieldValues, fieldVal)
	}

	funct := handlerChain[strings.Join(fieldValues, "")]

	if funct != nil {
		funct(iso)
	} else {
		//iso.SetField(39, rc.ISOFailed)
		//iso.SetField(48, "Not found")
		//_ = glg.Error("Handle not found..")
		if defaultHandler != nil {
			defaultHandler(iso)
		} else {
			logger.Error("Handle not found..")
			return
		}

	}
	resp, err := iso.ComposeMessage()
	if err != nil {
		//_ = glg.Error("ISO 8583 compose error : ", err.Error())
		logger.Error("ISO 8583 compose error : ", err.Error())
		return
	}

	ln := len(resp)
	h := strconv.Itoa(ln)
	resp = fmt.Sprint(strutils.LeftPad(h, 4, "0"), resp)
	_, _ = c.Write([]byte(resp))

}

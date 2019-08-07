package main

import (
	"io"
	"log"
	"net"
	"runtime/debug"
	"strconv"
)

func main() {
	listener, err := net.Listen("tcp", ":9999")
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		connRW := NewReadWriter(conn)
		if err != nil {
			log.Println(err)
			continue
		}

		go func() {
			defer func() {
				if info := recover(); info != nil {
					_ = conn.Close()
					log.Println(string(debug.Stack()))
				}
			}()

			ver := connRW.MustReadByte()
			nMethods := connRW.MustReadByte()
			methods := connRW.MustReadByteSize(int(nMethods))
			log.Printf("ver: %d, nMethods: %d, methods: %d\n", ver, nMethods, methods)

			connRW.MustWrite([]byte{5, 0})

			ver2 := connRW.MustReadByte()
			cmd := connRW.MustReadByte()
			// ignore rsv, always 0
			_ = connRW.MustReadByte()

			atyp := connRW.MustReadByte()

			dstAddrLen := -1
			if atyp == 1 {
				dstAddrLen = 4
			} else if atyp == 4 {
				dstAddrLen = 16
			} else if atyp == 3 {
				dstAddrLen = int(connRW.MustReadByte())
			}

			dstAddrArr := connRW.MustReadByteSize(dstAddrLen)
			dstPortArr := connRW.MustReadByteSize(2)

			log.Printf("ver: %d, cmd: %d, atyp: %d, rsv: 0, dst.addr: %d, dst.port: %d", ver2, cmd, atyp, dstAddrArr, dstPortArr)
			connRW.MustWrite([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})

			dstAddr := getAddress(atyp, dstAddrArr)
			port := getPort(dstPortArr)

			c, err := net.Dial("tcp", dstAddr+":"+strconv.Itoa(port))
			if err != nil {
				panic(err)
			}
			// transfer from client to remote
			go func() {
				_, _ = io.Copy(c, conn)
				_ = c.Close()
				connRW.Close()
			}()
			// transfer from remote to client
			go func() {
				_, _ = io.Copy(conn, c)
				_ = c.Close()
				connRW.Close()
			}()
		}()
	}
}

func getAddress(atyp byte, addrArr []byte) string {
	// Domain
	if atyp == 3 {
		return string(addrArr)
	}

	// IPv4 or IPv6
	if len(addrArr) == 4 || len(addrArr) == 16 {
		return net.IP(addrArr).String()
	}

	panic("not support protocol")
}

func getPort(portAddr []byte) int {
	if portAddr[1] > 0 {
		return int(portAddr[0])*256 + int(portAddr[1])
	}
	return int(portAddr[0])*256 + 256 + int(portAddr[1])
}

// simplify IO operations
type ReadWriter struct {
	rw io.ReadWriteCloser
}

func (r *ReadWriter) MustReadByte() byte {
	buf := make([]byte, 1)
	_, err := r.rw.Read(buf)
	if err != nil {
		panic(err)
	}
	return buf[0]
}

func (r *ReadWriter) MustReadByteSize(size int) []byte {
	buf := make([]byte, size)
	_, err := r.rw.Read(buf)
	if err != nil {
		panic(err)
	}
	return buf
}

func (r *ReadWriter) MustWrite(bytes []byte) {
	_, err := r.rw.Write(bytes)
	if err != nil {
		panic(err)
	}
}

func (r *ReadWriter) Close() {
	_ = r.rw.Close()
}

func NewReadWriter(r io.ReadWriteCloser) *ReadWriter {
	return &ReadWriter{rw: r}
}

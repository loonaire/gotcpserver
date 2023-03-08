package main

import "servertcp/tcpserver"

func main() {

	server := tcpserver.NewTcpServer("127.0.0.1", "5555")
	server.ListenAndServe()
}

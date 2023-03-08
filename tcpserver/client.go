package tcpserver

import (
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

const (
	MAX_PACKET_LENGTH = 201
	SOCKET_TIMEOUT    = 60
)

var (
	errClientDisconnected     = errors.New("Client déconnecté")
	errClientConnexionTimeout = errors.New("Client déconnecté pour inactivité")
)

type client struct {
	srv  *TcpServer
	conn *net.TCPConn
	name string
}

func (c *client) send(packetData string) {
	packet := []byte(packetData + "\n")
	log.Printf("%s - >> %q\n", c.conn.RemoteAddr(), packet)
	c.conn.Write(packet)
}

func (c *client) recv() (string, error) {
	// crée un buffer pour le paquet
	packet := make([]byte, MAX_PACKET_LENGTH)
	packetlen, err := (c.conn).Read(packet)
	// si le client n'envoie plus de paquet depuis un certain temps, on le deconnecte
	// note: SetReadDeadline peu être remplacé par SetDeadline
	(c.conn).SetReadDeadline(time.Now().Add(SOCKET_TIMEOUT * time.Second))
	if err != nil {
		switch err {
		case io.EOF:
			return "", errClientDisconnected
		default:
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// client inactif, voir pour passer ca en case
				return "", errClientConnexionTimeout
			}
			return "", err
		}
	}
	if packetlen == 0 {
		log.Println("reception d'un paquet vide")
		return "", nil
	}
	log.Printf("%s - << %q\n", c.conn.RemoteAddr(), string(packet[:packetlen]))
	return strings.Trim(string(packet[:packetlen]), "\r\n"), nil
}

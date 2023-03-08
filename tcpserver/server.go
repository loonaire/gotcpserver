package tcpserver

import (
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TcpServer struct {
	ip                     string
	port                   string
	clients                map[*client]struct{}
	connectedClientsByName map[string]*client
	mu                     sync.Mutex
	shutdownServer         chan int // le type int permet de donner une durée avant l'extinction du serveur, sinon le type bool est préférable
	wg                     sync.WaitGroup
}

func NewTcpServer(ip string, port string) *TcpServer {
	return &TcpServer{ip: ip, port: port, clients: make(map[*client]struct{}), mu: sync.Mutex{}, connectedClientsByName: make(map[string]*client), shutdownServer: make(chan int, 1) /*, gameServers: make(map[int]*GameServer)*/}
}

func (srv *TcpServer) ListenAndServe() {
	addr, errAddr := net.ResolveTCPAddr("tcp", srv.ip+":"+srv.port)
	if errAddr != nil {
		log.Fatalln("Impossible de résoudre l'adresse ip")
	}
	listener, errListen := net.ListenTCP("tcp", addr)
	if errListen != nil {
		log.Fatalln("Impossible d'écouter sur l'adresse ", srv.ip, " et sur le port ", srv.port)
	}
	defer listener.Close()
	log.Println("Serveur démarré sur l'ip:", srv.ip, " et sur le port:", srv.port)

	defer srv.wg.Wait()

	// goroutine qui attend la demande d'extinction du serveur
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
	shutdownSignal:
		for {
			select {
			case timeWait := <-srv.shutdownServer:
				/*
					attend la durée souhaité puis démarre l'extinction du serveur
					cette partie est surement à retravailler pour permettre par exemple d'arrêter ce chrono
				*/
				log.Println(timeWait)
				time.Sleep(time.Duration(timeWait) * time.Second)
				break shutdownSignal
			default:
			}
		}
		listener.Close()
	}()

	// boucle qui accepte les connexions
	for {
		conn, errConn := listener.AcceptTCP()
		if errConn != nil {
			// on gère l'erreur de la fermeture du listener
			log.Println("Arret de l'acceptation des connexions entrantes")
			break
		}
		srv.wg.Add(1)
		go srv.handleConn(conn)
	}

	// à partir d'ici, le serveur s'arrete
	srv.closeAllConnection()
	log.Println("hey c'est fini")
}

func (srv *TcpServer) handleConn(conn *net.TCPConn) {
	defer srv.wg.Done()
	defer conn.Close()

	client := &client{srv: srv, conn: conn}

	srv.addClient(client)
	defer srv.deleteClient(client)

	client.send("bonjour")
	client.send("Sasir votre nom:")
	name, errName := client.recv()
	client.name = name
	if errName != nil {
		log.Println(errName)
		return
	}

	if srv.checkClientAlreadyConnected(client.name) {
		// vérifie si le client est déja connecté
		client.send("Nom déja utilisé")
		return
	}

	srv.addConnectedClient(client)
	defer srv.disconnectConnectedClient(client)

	for {
		packet, recvError := client.recv()
		if recvError != nil {
			log.Println(recvError)
			break
		}

		if packet == "" {
			// gère le cas ou un paquet vide est reçu
			continue
		}

		switch string(packet[0]) {
		case "/":
			// si / est le premier caractère, il s'agit d'une commande
			// on split la chaine pour récupérer les arguments:
			packetSplit := strings.Split(packet[1:], " ")
			log.Println(packetSplit)
			switch packetSplit[0] {
			case "exit":
				// deconnecte le client
				log.Println("bye")
				// ici on return la valeur pour éviter le double switch
				return
			case "shutdown":
				// donne l'ordre de l'arret du serveur
				// si shutdown seulement, on coupe direct
				// si shutdown + nombre, on coupe à la fin du temps
				switch len(packetSplit) {
				case 1:
					// seulement shutdown, on coupe directement
					srv.shutdownServer <- 0
				case 2:
					// shutdown + arg, si arg est un entier, on coupe le serveur dans arg minutes
					timeBeforeShutdown, errConv := strconv.Atoi(packetSplit[1])
					if errConv == nil {
						// si la conversion réussie, on éteint le serveur
						srv.shutdownServer <- timeBeforeShutdown
					}
				default:
					// si il y a + de 2 arguments, on fait rien
					client.send("Trop d'arguments")
				}
			case "w":
				// envoi un message privé, packet de la forme: w nomClient message ...
				if srv.checkClientAlreadyConnected(packetSplit[1]) {
					srv.sendToConnectedClient(packetSplit[1], packet)
				} else {
					client.send("Erreur, utilisateur déconnecté")
				}
			default:
				client.send("erreur, commande invalide")
			}
		default:
			srv.sendToAllConnectedClients(packet)
		}
	}
}

func (srv *TcpServer) closeAllConnection() {
	/* TODO
	Cette fonction est personnalisable, ici je ferme les connexions avec close(),
	une manière plus propre est d'envoyé au client un paquet qui provoque la deconnexion coté client, de cette manière
	le serveur reçoit io.EOF comme erreur et il se déconnecte sans erreur.
	*/
	for cli := range srv.clients {
		cli.conn.Close()
	}
}

func (srv *TcpServer) sendToAllClients(message string) {
	//envoi un message à tous les clients
	for client := range srv.clients {
		client.send(message)
	}
}

func (srv *TcpServer) sendToAllConnectedClients(message string) {
	// envoi le message a tous les clients qui sont connecté (qui ont un nom)
	for name := range srv.connectedClientsByName {
		srv.connectedClientsByName[name].send(message)
	}
}

func (srv *TcpServer) sendToConnectedClient(clientName string, message string) {
	// envoi un message a un seul client (message privé)
	srv.connectedClientsByName[clientName].send(message)
}

func (srv *TcpServer) addClient(client *client) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.clients[client] = struct{}{}
}

func (srv *TcpServer) deleteClient(client *client) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	delete(srv.clients, client)

}

func (srv *TcpServer) addConnectedClient(client *client) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.connectedClientsByName[client.name] = client
}

func (srv *TcpServer) disconnectConnectedClient(client *client) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	delete(srv.connectedClientsByName, client.name)
}

func (srv *TcpServer) checkClientAlreadyConnected(name string) bool {
	// fonction qui vérifie si un nom est déja utilisé
	// TODO cette fonction peux être renommée pour mieux correspondre à d'autres programmes
	if _, isConnected := srv.connectedClientsByName[name]; isConnected {
		// vérifie si le client veux utiliser un nom déja utlisé
		return true
	}
	return false
}
